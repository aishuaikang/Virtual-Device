package modules

import (
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
		deviceId:  2345,
		title:     "test jamming",
		config:    &config.Config{},
		generator: mocks.NewJammingDataGenerator(2345, 10),
		ctx:       ctx,
		cancel:    cancel,
		clients:   make(map[string]*jammingClientSession),
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
