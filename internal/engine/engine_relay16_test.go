//go:build integration

package engine

import (
	"io"
	"net"
	"testing"
	"time"

	"virtual-device-ui/internal/config"
)

func relay16EngineConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Analysis.Enabled, cfg.FPV.Enabled, cfg.Jamming.Enabled = false, false, false
	cfg.DirectedStrike.Enabled, cfg.PrecisionDF.Enabled = false, false
	for i := range cfg.Detections {
		cfg.Detections[i].Enabled = false
	}
	cfg.RandomDroneRefreshInterval = 0
	cfg.Relay16 = config.Relay16Config{Enabled: true, Host: "127.0.0.1"}
	return cfg
}

func TestEngineRelay16StatusStopAndRestart(t *testing.T) {
	e := New()
	defer e.Stop()
	cfg := relay16EngineConfig()
	if err := e.Start(cfg); err != nil {
		t.Fatal(err)
	}
	status := e.GetStatus()
	if !status.Running || status.Relay16 == nil || !status.Relay16.Listening {
		t.Fatalf("relay simulator missing: %+v", status)
	}
	address := status.Relay16.ListenAddress
	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := connection.Write([]byte{0, 1, 0, 0, 0, 9, 1, 15, 0, 0, 0, 15, 2, 0x49, 0x12}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(connection, make([]byte, 12)); err != nil {
		t.Fatal(err)
	}
	status = e.GetStatus()
	if status.Relay16.OutputMask != 0x1249 {
		t.Fatalf("output missing from engine snapshot: %+v", status.Relay16)
	}
	found := false
	for _, module := range status.Modules {
		if module.Name == "relay16" {
			found = module.Connected && module.ConnectionCount == 1 && module.LastActivityAt != ""
		}
	}
	if !found {
		t.Fatalf("relay connection missing from module status: %+v", status.Modules)
	}
	e.Stop()
	if e.GetStatus().Running {
		t.Fatal("engine remained running after stop")
	}
	if _, err := connection.Read(make([]byte, 1)); err == nil {
		t.Fatal("stop left relay client connected")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("stop leaked relay listener: %v", err)
	}
	_ = listener.Close()
	if err := e.Start(cfg); err != nil {
		t.Fatal(err)
	}
	if snapshot := e.GetStatus().Relay16; snapshot.OutputMask != 0 || !snapshot.Listening {
		t.Fatalf("restart did not reset the board: %+v", snapshot)
	}
}

func TestEngineRelay16StartupFailureReleasesOtherListeners(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	var ports []int
	var addresses []string
	for range 3 {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		ports = append(ports, listener.Addr().(*net.TCPAddr).Port)
		addresses = append(addresses, listener.Addr().String())
		_ = listener.Close()
	}
	cfg := relay16EngineConfig()
	cfg.DirectedStrike = config.DirectedStrikeConfig{Enabled: true, Host: "127.0.0.1", Port: ports[0]}
	cfg.PrecisionDF.Enabled, cfg.PrecisionDF.Host = true, "127.0.0.1"
	cfg.PrecisionDF.CommandPort, cfg.PrecisionDF.DataPort = ports[1], ports[2]
	cfg.Relay16.Port = busy.Addr().(*net.TCPAddr).Port
	e := New()
	defer e.Stop()
	if err := e.Start(cfg); err == nil || e.IsRunning() {
		t.Fatal("engine accepted an occupied relay port")
	}
	for _, address := range addresses {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatalf("failed startup leaked %s: %v", address, err)
		}
		_ = listener.Close()
	}
}
