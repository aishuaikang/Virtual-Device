package modules

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/mocks"
	"virtual-device-ui/internal/protocol"
)

func newJammingPipeTest(t *testing.T) (*jamming, *jammingClientSession, net.Conn, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	module := &jamming{
		deviceId:   2345,
		title:      "test jamming",
		config:     &config.Config{},
		generator:  mocks.NewJammingDataGenerator(2345, 10),
		ctx:        ctx,
		cancel:     cancel,
		clients:    make(map[string]*jammingClientSession),
		simulation: config.JammingConfig{FaultMode: config.JammingFaultNormal, FaultChannel: 1},
	}
	session, created := module.getOrCreateSession("test-server")
	if !created {
		t.Fatal("session was not created")
	}
	client, server := net.Pipe()
	if !module.attachSessionConn("test-server", session, client) {
		t.Fatal("connection was not attached")
	}
	moduleDone := make(chan struct{})
	go func() {
		module.Start()
		close(moduleDone)
	}()
	return module, session, server, func() {
		module.Stop()
		server.Close()
		<-moduleDone
	}
}

func startJammingPipeReceiver(module *jamming, session *jammingClientSession) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		module.handleDataReception("test-server", session)
		close(done)
	}()
	return done
}

func sendJammingPowerCommand(t *testing.T, server net.Conn, enabled byte) {
	t.Helper()
	if err := server.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	request := (&protocol.JammingPacket{
		Header: protocol.ProtocolHeader,
		ID:     2345,
		Length: 8,
		Cmd:    protocol.CmdSetPower,
		Data:   []byte{enabled, 0x21},
		Tail:   protocol.ProtocolTail,
	}).ToBytes()
	if _, err := server.Write(request); err != nil {
		t.Fatalf("write power command after idle: %v", err)
	}
	response := make([]byte, 7)
	if _, err := io.ReadFull(server, response); err != nil {
		t.Fatalf("read power response: %v", err)
	}
	packet, err := protocol.ParseJammingPacket(response)
	if err != nil || packet.Cmd != protocol.CmdSetPower || packet.Data[0] != 0 {
		t.Fatalf("unexpected power response: %x, error=%v", response, err)
	}
}

func TestJammingIdleConnectionSurvivesUnattendedCycles(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		module, session, server, stop := newJammingPipeTest(t)
		defer stop()
		synctest.Wait()

		// A connected device can wait for targets without receiving any commands.
		time.Sleep(2 * time.Minute)
		if got := module.ConnectedCount(); got != 1 {
			t.Fatalf("idle connection was removed: connected=%d", got)
		}

		done := startJammingPipeReceiver(module, session)
		for range 2 {
			sendJammingPowerCommand(t, server, 1)
			time.Sleep(time.Minute)
			sendJammingPowerCommand(t, server, 0)
			time.Sleep(time.Minute)
		}
		// Read deadlines must also allow long standby on the same connection.
		time.Sleep(10 * time.Minute)
		sendJammingPowerCommand(t, server, 1)
		module.Stop()
		<-done
	})
}

func TestJammingStopInterruptsBlockedIO(t *testing.T) {
	for _, blockedWrite := range []bool{false, true} {
		name := "read"
		if blockedWrite {
			name = "write"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				module, session, server, stop := newJammingPipeTest(t)
				defer stop()
				done := startJammingPipeReceiver(module, session)
				if blockedWrite {
					request := (&protocol.JammingPacket{
						Header: protocol.ProtocolHeader, ID: 2345, Length: 8,
						Cmd: protocol.CmdSetPower, Data: []byte{1, 0x21}, Tail: protocol.ProtocolTail,
					}).ToBytes()
					if _, err := server.Write(request); err != nil {
						t.Fatal(err)
					}
				}
				synctest.Wait()
				stopped := make(chan struct{})
				go func() {
					module.Stop()
					close(stopped)
				}()
				select {
				case <-stopped:
				case <-time.After(time.Second):
					t.Fatal("Stop was blocked by network I/O")
				}
				<-done
				if got := module.ConnectedCount(); got != 0 {
					t.Fatalf("connections remain after Stop: %d", got)
				}
			})
		})
	}
}

func TestJammingDisconnectAllowsNewSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		module, session, server, stop := newJammingPipeTest(t)
		defer stop()
		done := startJammingPipeReceiver(module, session)
		synctest.Wait()
		server.Close()
		<-done
		if got := module.ConnectedCount(); got != 0 {
			t.Fatalf("disconnected session remains active: %d", got)
		}
		replacement, created := module.getOrCreateSession("test-server")
		if !created || replacement == session || replacement.ctx.Err() != nil {
			t.Fatal("could not create a fresh session after disconnect")
		}
		client, nextServer := net.Pipe()
		defer nextServer.Close()
		if !module.attachSessionConn("test-server", replacement, client) {
			t.Fatal("could not attach replacement connection")
		}
		nextDone := startJammingPipeReceiver(module, replacement)
		// Late cleanup from the old receiver must not remove its replacement.
		module.markSessionDisconnected("test-server", session)
		sendJammingPowerCommand(t, nextServer, 1)
		module.Stop()
		<-nextDone
	})
}

func TestJammingStoppedModuleRejectsLateConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		module, session, _, stop := newJammingPipeTest(t)
		defer stop()
		module.Stop()
		client, server := net.Pipe()
		defer client.Close()
		defer server.Close()
		if module.attachSessionConn("test-server", session, client) {
			t.Fatal("stopped module accepted an old session connection")
		}
		if _, created := module.getOrCreateSession("test-server"); created {
			t.Fatal("stopped module created a new session")
		}
	})
}

func jammingTestWire(cmd byte, data []byte) []byte {
	return (&protocol.JammingPacket{
		Header: protocol.ProtocolHeader, ID: 2345, Length: uint8(6 + len(data)),
		Cmd: cmd, Data: data, Tail: protocol.ProtocolTail,
	}).ToBytes()
}

func readJammingResponse(t *testing.T, server net.Conn) *protocol.JammingPacket {
	t.Helper()
	server.SetReadDeadline(time.Now().Add(time.Second))
	header := make([]byte, 4)
	if _, err := io.ReadFull(server, header); err != nil {
		t.Fatal(err)
	}
	if header[3] < 6 {
		t.Fatalf("invalid response header: %x", header)
	}
	wire := make([]byte, int(header[3]))
	copy(wire, header)
	if _, err := io.ReadFull(server, wire[4:]); err != nil {
		t.Fatal(err)
	}
	packet, err := protocol.ParseJammingPacket(wire)
	if err != nil {
		t.Fatal(err)
	}
	return packet
}

func assertJammingChannels(t *testing.T, packet *protocol.JammingPacket, stuckChannel int, enabled byte) {
	t.Helper()
	if packet.Cmd != protocol.CmdQueryModules || len(packet.Data) != 110 {
		t.Fatalf("expected full ten-channel status: %+v", packet)
	}
	for i := range 10 {
		want := enabled
		if i+1 == stuckChannel {
			want = 1
		}
		data := packet.Data[i*11 : (i+1)*11]
		if data[6] != want || data[8] != 8 || (want == 0 && data[4] != 0) {
			t.Fatalf("channel %d = %x, expected switch %d and normal alarm", i+1, data, want)
		}
	}
}

func TestJammingFragmentedCommand(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		module, session, server, stop := newJammingPipeTest(t)
		defer stop()
		done := startJammingPipeReceiver(module, session)
		wire := jammingTestWire(protocol.CmdSetPower, bytes.Repeat([]byte{1, 0x30}, 10))
		for _, value := range wire {
			if _, err := server.Write([]byte{value}); err != nil {
				t.Fatal(err)
			}
		}
		if ack := readJammingResponse(t, server); ack.Cmd != 4 || ack.Data[0] != 0 {
			t.Fatalf("unexpected ACK: %+v", ack)
		}
		server.Write(jammingTestWire(protocol.CmdQueryModules, nil))
		assertJammingChannels(t, readJammingResponse(t, server), 0, 1)
		module.Stop()
		<-done
	})
}

func TestJammingInstantSwitchBurst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		module, session, server, stop := newJammingPipeTest(t)
		defer stop()
		done := startJammingPipeReceiver(module, session)
		var burst []byte
		for range 100 {
			// No wait, ACK or query between ON and OFF: send them in the same burst.
			burst = append(burst, jammingTestWire(4, bytes.Repeat([]byte{1, 0x30}, 10))...)
			burst = append(burst, jammingTestWire(4, bytes.Repeat([]byte{0, 0x30}, 10))...)
			burst = append(burst, jammingTestWire(5, nil)...)
		}
		written := make(chan error, 1)
		go func() {
			_, err := server.Write(burst)
			written <- err
		}()
		for range 100 {
			for range 2 {
				if ack := readJammingResponse(t, server); ack.Cmd != 4 || len(ack.Data) != 1 || ack.Data[0] != 0 {
					t.Fatalf("unexpected ACK: %+v", ack)
				}
			}
			assertJammingChannels(t, readJammingResponse(t, server), 0, 0)
		}
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		module.Stop()
		<-done
	})
}

func TestJammingDroppedResponsesPreserveExecution(t *testing.T) {
	for _, mode := range []string{config.JammingFaultDropACK, config.JammingFaultDropStatus, config.JammingFaultSilent} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				module, session, server, stop := newJammingPipeTest(t)
				defer stop()
				done := startJammingPipeReceiver(module, session)
				settings := module.simulation
				settings.FaultMode = mode
				if err := module.UpdateSimulation(settings); err != nil {
					t.Fatal(err)
				}
				server.Write(jammingTestWire(4, bytes.Repeat([]byte{1, 0x30}, 10)))
				if mode == config.JammingFaultDropStatus {
					readJammingResponse(t, server)
					server.Write(jammingTestWire(5, nil))
				}
				server.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
				var data [1]byte
				if _, err := server.Read(data[:]); err == nil {
					t.Fatal("fault mode returned a suppressed response")
				} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
					t.Fatalf("expected silence on a live connection: %v", err)
				}
				settings.FaultMode = config.JammingFaultNormal
				if err := module.UpdateSimulation(settings); err != nil {
					t.Fatal(err)
				}
				server.Write(jammingTestWire(5, nil))
				assertJammingChannels(t, readJammingResponse(t, server), 0, 1)
				module.Stop()
				<-done
			})
		})
	}
}

func TestJammingReadbackFaults(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		module, session, server, stop := newJammingPipeTest(t)
		defer stop()
		done := startJammingPipeReceiver(module, session)
		settings := module.simulation
		settings.FaultMode, settings.FaultChannel = config.JammingFaultStuckOn, 3
		if err := module.UpdateSimulation(settings); err != nil {
			t.Fatal(err)
		}
		server.Write(jammingTestWire(4, bytes.Repeat([]byte{0, 0x30}, 10)))
		if ack := readJammingResponse(t, server); ack.Data[0] != 0 {
			t.Fatal("stuck-on fault should ACK the command before readback exposes it")
		}
		server.Write(jammingTestWire(5, nil))
		assertJammingChannels(t, readJammingResponse(t, server), 3, 0)

		settings.FaultMode = config.JammingFaultPartialStatus
		if err := module.UpdateSimulation(settings); err != nil {
			t.Fatal(err)
		}
		server.Write(jammingTestWire(5, nil))
		if packet := readJammingResponse(t, server); packet.Length != 105 || len(packet.Data) != 99 {
			t.Fatalf("partial status must contain nine complete channels: %+v", packet)
		}
		settings.FaultMode = config.JammingFaultNormal
		if err := module.UpdateSimulation(settings); err != nil {
			t.Fatal(err)
		}
		server.Write(jammingTestWire(5, nil))
		assertJammingChannels(t, readJammingResponse(t, server), 3, 0)
		server.Write(jammingTestWire(4, bytes.Repeat([]byte{0, 0x30}, 10)))
		readJammingResponse(t, server)
		server.Write(jammingTestWire(5, nil))
		assertJammingChannels(t, readJammingResponse(t, server), 0, 0)
		module.Stop()
		<-done
	})
}

func TestJammingNetworkFaultPreservesStateUntilRecoveryOFF(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		module, session, server, stop := newJammingPipeTest(t)
		defer stop()
		done := startJammingPipeReceiver(module, session)
		server.Write(jammingTestWire(4, bytes.Repeat([]byte{1, 0x30}, 10)))
		readJammingResponse(t, server)
		settings := module.simulation
		settings.FaultMode = config.JammingFaultDisconnected
		if err := module.UpdateSimulation(settings); err != nil {
			t.Fatal(err)
		}
		<-done
		if module.ConnectedCount() != 0 {
			t.Fatal("disconnected mode kept the old socket")
		}
		if _, created := module.getOrCreateSession("test-server"); created {
			t.Fatal("network fault allowed a reconnect")
		}
		assertJammingChannels(t, module.generator.GenerateModuleInfoPacket(), 0, 1)
		settings.FaultMode = config.JammingFaultNormal
		if err := module.UpdateSimulation(settings); err != nil {
			t.Fatal(err)
		}
		replacement, created := module.getOrCreateSession("test-server")
		if !created {
			t.Fatal("reconnect was rejected after network recovery")
		}
		client, nextServer := net.Pipe()
		defer nextServer.Close()
		if !module.attachSessionConn("test-server", replacement, client) {
			t.Fatal("could not attach the new socket")
		}
		nextDone := startJammingPipeReceiver(module, replacement)
		module.markSessionDisconnected("test-server", session)
		nextServer.Write(jammingTestWire(5, nil))
		assertJammingChannels(t, readJammingResponse(t, nextServer), 0, 1)
		nextServer.Write(jammingTestWire(4, bytes.Repeat([]byte{0, 0x30}, 10)))
		readJammingResponse(t, nextServer)
		nextServer.Write(jammingTestWire(5, nil))
		assertJammingChannels(t, readJammingResponse(t, nextServer), 0, 0)
		module.Stop()
		<-nextDone
	})
}

func TestJammingStopCancelsResponseDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		module, session, server, stop := newJammingPipeTest(t)
		defer stop()
		settings := module.simulation
		settings.ResponseDelayMS = 10000
		if err := module.UpdateSimulation(settings); err != nil {
			t.Fatal(err)
		}
		done := startJammingPipeReceiver(module, session)
		server.Write(jammingTestWire(5, nil))
		synctest.Wait()
		before := time.Now()
		module.Stop()
		<-done
		if time.Since(before) >= time.Second {
			t.Fatal("Stop waited for the artificial response delay")
		}
	})
}

func TestJammingResponseDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		module, session, server, stop := newJammingPipeTest(t)
		defer stop()
		settings := module.simulation
		settings.ResponseDelayMS = 500
		if err := module.UpdateSimulation(settings); err != nil {
			t.Fatal(err)
		}
		done := startJammingPipeReceiver(module, session)
		before := time.Now()
		server.Write(jammingTestWire(5, nil))
		assertJammingChannels(t, readJammingResponse(t, server), 0, 0)
		if elapsed := time.Since(before); elapsed != 500*time.Millisecond {
			t.Fatalf("response delay = %v, want 500ms", elapsed)
		}
		module.Stop()
		<-done
	})
}

func TestJammingRejectsRuntimeConnectionChanges(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*config.JammingConfig)
	}{
		{"device ID", func(c *config.JammingConfig) { c.DeviceID++ }},
		{"hosts", func(c *config.JammingConfig) { c.Hosts = []string{"other-server"} }},
		{"port", func(c *config.JammingConfig) { c.Port++ }},
		{"enabled", func(c *config.JammingConfig) { c.Enabled = !c.Enabled }},
		{"invalid delay", func(c *config.JammingConfig) { c.ResponseDelayMS = -1 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			module, _, _, stop := newJammingPipeTest(t)
			defer stop()
			settings := module.simulation
			settings.FaultMode = config.JammingFaultDisconnected
			tt.change(&settings)
			if err := module.UpdateSimulation(settings); err == nil {
				t.Fatal("invalid runtime edit was accepted")
			}
			if module.ConnectedCount() != 1 {
				t.Fatal("rejected edit disconnected the module")
			}
		})
	}
}

func TestJammingRejectsUnknownCommands(t *testing.T) {
	module, _, _, stop := newJammingPipeTest(t)
	defer stop()
	for _, cmd := range []byte{0, 0xff} {
		if response := module.handleCommand(&protocol.JammingPacket{Cmd: cmd}); response != nil {
			t.Fatalf("unknown command 0x%02x returned a fake ACK: %+v", cmd, response)
		}
	}
}
