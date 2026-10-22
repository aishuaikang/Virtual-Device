//go:build integration

package engine

import (
	"net"
	"testing"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/protocol"
)

func precisionDFEngineConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Analysis.Enabled, cfg.FPV.Enabled, cfg.Jamming.Enabled, cfg.DirectedStrike.Enabled = false, false, false, false
	for i := range cfg.Detections {
		cfg.Detections[i].Enabled = false
	}
	cfg.RandomDroneRefreshInterval = 0
	cfg.PrecisionDF.Enabled, cfg.PrecisionDF.Host = true, "127.0.0.1"
	cfg.PrecisionDF.CommandPort, cfg.PrecisionDF.DataPort = 0, 0
	return cfg
}

func TestEnginePrecisionDFStatusAndStop(t *testing.T) {
	e := New()
	if err := e.Start(precisionDFEngineConfig()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	status := e.GetStatus()
	if !status.Running || status.PrecisionDF == nil || !status.PrecisionDF.Listening {
		t.Fatalf("receiver missing from engine status: %+v", status)
	}
	command, err := net.Dial("tcp", status.PrecisionDF.CommandAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer command.Close()
	if _, err := protocol.ReadPrecisionDFCommand(command); err != nil {
		t.Fatal(err)
	}
	e.Stop()
	if e.GetStatus().Running {
		t.Fatal("engine still running after stop")
	}
	for _, address := range []string{status.PrecisionDF.CommandAddress, status.PrecisionDF.DataAddress} {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatalf("engine stop left receiver listener open: %v", err)
		}
		_ = listener.Close()
	}
}

func TestEnginePrecisionDFFailureReleasesDirectedStrike(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	directedPort, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := directedPort.Addr().String()
	port := directedPort.Addr().(*net.TCPAddr).Port
	_ = directedPort.Close()
	cfg := precisionDFEngineConfig()
	cfg.DirectedStrike.Enabled, cfg.DirectedStrike.Host, cfg.DirectedStrike.Port = true, "127.0.0.1", port
	cfg.PrecisionDF.DataPort = busy.Addr().(*net.TCPAddr).Port
	e := New()
	if err := e.Start(cfg); err == nil {
		e.Stop()
		t.Fatal("engine accepted an occupied receiver data port")
	}
	if e.IsRunning() {
		t.Fatal("engine marked running after startup failure")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("startup failure leaked directed-strike listener: %v", err)
	}
	_ = listener.Close()
}
