//go:build integration

package modules

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"virtual-device-ui/internal/config"
)

func startRelay16(t *testing.T, delay int) (*Relay16Module, <-chan struct{}) {
	t.Helper()
	module, err := NewRelay16Module(config.Relay16Config{Host: "127.0.0.1", ResponseDelayMS: delay})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { module.Start(); close(done) }()
	t.Cleanup(func() {
		module.Stop()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("relay simulator did not stop")
		}
	})
	return module, done
}

func relay16Request(transaction uint16, pdu []byte) []byte {
	request := make([]byte, 7+len(pdu))
	binary.BigEndian.PutUint16(request, transaction)
	binary.BigEndian.PutUint16(request[4:], uint16(1+len(pdu)))
	request[6] = 1
	copy(request[7:], pdu)
	return request
}

func readRelay16Reply(t *testing.T, connection net.Conn, transaction uint16, pdu []byte) {
	t.Helper()
	want := relay16Request(transaction, pdu)
	got := make([]byte, len(want))
	if _, err := io.ReadFull(connection, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = % X, want % X", got, want)
	}
}

func dialRelay16(t *testing.T, module *Relay16Module) net.Conn {
	t.Helper()
	connection, err := net.DialTimeout("tcp", module.Snapshot().ListenAddress, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := connection.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return connection
}

func TestRelay16TCPFragmentationPipelineAndReconnect(t *testing.T) {
	t.Parallel()
	module, _ := startRelay16(t, 0)
	connection := dialRelay16(t, module)
	request := relay16Request(0x1234, []byte{15, 0, 0, 0, 16, 2, 0x49, 0x92})
	for _, part := range [][]byte{request[:3], request[3:8], request[8:]} {
		if _, err := connection.Write(part); err != nil {
			t.Fatal(err)
		}
	}
	readRelay16Reply(t, connection, 0x1234, []byte{15, 0, 0, 0, 16})
	batch := append(
		relay16Request(2, []byte{1, 0, 0, 0, 16}),
		relay16Request(3, []byte{15, 0, 0, 0, 15, 2, 0, 0})...,
	)
	batch = append(batch, relay16Request(4, []byte{1, 0, 0, 0, 16})...)
	if _, err := connection.Write(batch); err != nil {
		t.Fatal(err)
	}
	readRelay16Reply(t, connection, 2, []byte{1, 2, 0x49, 0x92})
	readRelay16Reply(t, connection, 3, []byte{15, 0, 0, 0, 15})
	readRelay16Reply(t, connection, 4, []byte{1, 2, 0, 0x80})
	_ = connection.Close()

	reconnected := dialRelay16(t, module)
	if _, err := reconnected.Write(relay16Request(5, []byte{1, 0, 0, 0, 16})); err != nil {
		t.Fatal(err)
	}
	readRelay16Reply(t, reconnected, 5, []byte{1, 2, 0, 0x80})
	if snapshot := module.Snapshot(); snapshot.OutputMask != 0x8000 || snapshot.ReceivedFrames != 5 {
		t.Fatalf("unexpected board snapshot: %+v", snapshot)
	}
}

func TestRelay16TCPRejectsInvalidHeaders(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		header []byte
	}{
		{"protocol", []byte{0, 1, 0, 1, 0, 6, 1}},
		{"unit", []byte{0, 1, 0, 0, 0, 6, 2}},
		{"empty PDU", []byte{0, 1, 0, 0, 0, 1, 1}},
		{"oversize", []byte{0, 1, 0, 0, 0, 255, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, _ := startRelay16(t, 0)
			connection := dialRelay16(t, module)
			if _, err := connection.Write(tc.header); err != nil {
				t.Fatal(err)
			}
			if _, err := connection.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
				t.Fatalf("invalid header did not close connection: %v", err)
			}
			if module.Snapshot().OutputMask != 0 {
				t.Fatal("invalid header changed outputs")
			}
		})
	}
}

func TestRelay16ConcurrentClientsPreserveIndependentCoils(t *testing.T) {
	t.Parallel()
	module, _ := startRelay16(t, 0)
	var clients sync.WaitGroup
	for index := range 8 {
		connection := dialRelay16(t, module)
		clients.Add(1)
		go func() {
			defer clients.Done()
			defer connection.Close()
			pdu := []byte{15, 0, byte(index), 0, 1, 1, 1}
			if _, err := connection.Write(relay16Request(uint16(index), pdu)); err != nil {
				t.Error(err)
				return
			}
			want := relay16Request(uint16(index), pdu[:5])
			got := make([]byte, len(want))
			if _, err := io.ReadFull(connection, got); err != nil {
				t.Error(err)
				return
			}
			if !bytes.Equal(got, want) {
				t.Errorf("response = % X, want % X", got, want)
			}
		}()
	}
	clients.Wait()
	if mask := module.Snapshot().OutputMask; mask != 0xff {
		t.Fatalf("concurrent writes lost outputs: %04X", mask)
	}
}

func TestRelay16StopInterruptsResponseDelay(t *testing.T) {
	t.Parallel()
	module, done := startRelay16(t, 10000)
	connection := dialRelay16(t, module)
	if _, err := connection.Write(relay16Request(1, []byte{15, 0, 0, 0, 15, 2, 0xff, 0x7f})); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for module.Snapshot().ReceivedFrames == 0 {
		if time.Now().After(deadline) {
			t.Fatal("request did not reach delayed response handler")
		}
		time.Sleep(time.Millisecond)
	}
	module.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stop waited for response delay")
	}
	if _, err := connection.Read(make([]byte, 1)); err == nil {
		t.Fatal("client remained open after stop")
	}
	if snapshot := module.Snapshot(); snapshot.Listening || snapshot.Connected || snapshot.OutputMask != 0 {
		t.Fatalf("unexpected stopped state: %+v", snapshot)
	}
}
