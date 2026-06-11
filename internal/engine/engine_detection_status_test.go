package engine

import (
	"net"
	"strings"
	"testing"
	"time"

	"virtual-device-ui/internal/config"
)

func reserveUDPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("reserve udp port: %v", err)
	}
	port := listener.LocalAddr().(*net.UDPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("close reserved udp port: %v", err)
	}
	return port
}

func writeUDPCommand(t *testing.T, conn *net.UDPConn, command []byte, label string) {
	t.Helper()

	if _, err := conn.Write(command); err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return
		}
		t.Fatalf("send detection command %s: %v", label, err)
	}
}

func TestGetStatusMarksDetectionConnectedAfterUDPCommand(t *testing.T) {
	portA := reserveUDPPort(t)
	portB := reserveUDPPort(t)
	for portB == portA {
		portB = reserveUDPPort(t)
	}

	cfg := config.DefaultConfig()
	cfg.Analysis.Enabled = false
	cfg.FPV.Enabled = false
	cfg.Jamming.Enabled = false
	cfg.Detections = []config.DetectionConfig{
		{
			Enabled:           true,
			DeviceID:          2000,
			DroneCount:        1,
			Host:              "127.0.0.1",
			Port:              portA,
			HeartbeatInterval: 1,
		},
		{
			Enabled:           true,
			DeviceID:          2001,
			DroneCount:        1,
			Host:              "127.0.0.1",
			Port:              portB,
			HeartbeatInterval: 1,
		},
	}

	e := New()
	if err := e.Start(cfg); err != nil {
		t.Fatalf("start engine: %v", err)
	}
	defer e.Stop()

	connA, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: portA})
	if err != nil {
		t.Fatalf("dial udp server a: %v", err)
	}
	defer connA.Close()

	connB, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: portB})
	if err != nil {
		t.Fatalf("dial udp server b: %v", err)
	}
	defer connB.Close()

	command := []byte("start -freq 5840 -set_ant 255 -turn_on_gpio 3")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		writeUDPCommand(t, connA, command, "a")
		writeUDPCommand(t, connB, []byte("st -freq 3,-fpv_only 1,-rx2 2 -gain 70,-switch 1"), "b")

		status := e.GetStatus()
		for _, mod := range status.Modules {
			if mod.Name != "detection" {
				continue
			}
			if mod.ConnectionCount >= 2 && mod.Connected && len(mod.ClientAddresses) >= 2 && mod.LastActivityAt != "" {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	status := e.GetStatus()
	t.Fatalf("expected detection module to become connected after udp command, got %#v", status)
}
