//go:build integration

package modules

import (
	"bytes"
	"net"
	"testing"
	"time"

	"virtual-device-ui/internal/config"
)

func TestJammingTCPReconnectRecovery(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	previous := config.GetConfig()
	defer config.SetGlobalConfig(previous)
	cfg := config.DefaultConfig()
	cfg.Jamming.Port = listener.Addr().(*net.TCPAddr).Port
	config.SetGlobalConfig(cfg)
	module := NewJammingModule(cfg.Jamming.DeviceID)
	done := make(chan struct{})
	go func() {
		defer close(done)
		module.Start()
	}()
	defer func() {
		module.Stop()
		<-done
	}()
	accept := func() net.Conn {
		t.Helper()
		listener.SetDeadline(time.Now().Add(7 * time.Second))
		connection, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		connection.SetDeadline(time.Now().Add(3 * time.Second))
		t.Cleanup(func() { connection.Close() })
		return connection
	}
	server := accept()
	server.Write(jammingTestWire(5, nil))
	assertJammingChannels(t, readJammingResponse(t, server), 0, 0)

	// A single TCP write contains an immediate ON/OFF pair and its status query.
	burst := append(jammingTestWire(4, bytes.Repeat([]byte{1, 0x30}, 10)), jammingTestWire(4, bytes.Repeat([]byte{0, 0x30}, 10))...)
	burst = append(burst, jammingTestWire(5, nil)...)
	for range 100 {
		if _, err := server.Write(burst); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if ack := readJammingResponse(t, server); ack.Cmd != 4 || ack.Data[0] != 0 {
				t.Fatalf("unexpected switch ACK: %+v", ack)
			}
		}
		assertJammingChannels(t, readJammingResponse(t, server), 0, 0)
	}
	server.Write(jammingTestWire(4, bytes.Repeat([]byte{1, 0x30}, 10)))
	readJammingResponse(t, server)
	cfg.Jamming.FaultMode = config.JammingFaultDisconnected
	if err := module.UpdateSimulation(cfg.Jamming); err != nil {
		t.Fatal(err)
	}
	if module.ConnectedCount() != 0 {
		t.Fatal("network fault left a connected session")
	}
	var data [1]byte
	if _, err := server.Read(data[:]); err == nil {
		t.Fatal("old TCP connection was not closed")
	}
	cfg.Jamming.FaultMode = config.JammingFaultNormal
	if err := module.UpdateSimulation(cfg.Jamming); err != nil {
		t.Fatal(err)
	}
	reconnected := accept()
	reconnected.Write(jammingTestWire(5, nil))
	assertJammingChannels(t, readJammingResponse(t, reconnected), 0, 1)
	// The platform owns recovery: OFF ACK followed by a complete all-off readback.
	reconnected.Write(jammingTestWire(4, bytes.Repeat([]byte{0, 0x30}, 10)))
	if ack := readJammingResponse(t, reconnected); ack.Data[0] != 0 {
		t.Fatal("recovery OFF was rejected")
	}
	reconnected.Write(jammingTestWire(5, nil))
	assertJammingChannels(t, readJammingResponse(t, reconnected), 0, 0)
}
