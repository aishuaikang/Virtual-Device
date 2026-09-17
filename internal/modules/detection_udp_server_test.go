package modules

import (
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/mocks"
	"virtual-device-ui/internal/utils"
)

func reserveDetectionUDPPort(t *testing.T) int {
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

func TestDetectionUDPServerSendsSpectrumFrameForFFTCommand(t *testing.T) {
	originalCfg := config.GetConfig()
	cfg := config.DefaultConfig()
	cfg.MinPushSpeed = 20
	cfg.MaxPushSpeed = 20
	config.SetGlobalConfig(cfg)
	defer config.SetGlobalConfig(originalCfg)

	detectionCfg := config.DetectionConfig{
		Enabled:           true,
		DeviceID:          2000,
		DroneCount:        0,
		Host:              "127.0.0.1",
		Port:              reserveDetectionUDPPort(t),
		HeartbeatInterval: 1,
	}
	module := NewDetectionUDPModule(detectionCfg, mocks.NewMockDataGenerator(0))

	done := make(chan struct{})
	go func() {
		_ = module.Start()
		close(done)
	}()
	defer func() {
		module.Stop()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("detection module did not stop in time")
		}
	}()

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: detectionCfg.Port})
	if err != nil {
		t.Fatalf("dial udp server: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 512)
	command := []byte("start -fft 64 -band 3600,5800, \n")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := conn.Write(command); err != nil {
			if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
				continue
			}
		}
		if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			continue
		}
		if n != 4+64*2 {
			continue
		}
		if got := binary.BigEndian.Uint32(buf[:4]); got != 3600*1000+2320 {
			t.Fatalf("unexpected spectrum frequency header: %d", got)
		}
		return
	}

	t.Fatal("timed out waiting for spectrum frame")
}

func TestDetectionUDPServerReturnsAckForCommands(t *testing.T) {
	originalCfg := config.GetConfig()
	cfg := config.DefaultConfig()
	cfg.MinPushSpeed = 500
	cfg.MaxPushSpeed = 500
	config.SetGlobalConfig(cfg)
	defer config.SetGlobalConfig(originalCfg)

	detectionCfg := config.DetectionConfig{
		Enabled:           true,
		DeviceID:          2000,
		DroneCount:        0,
		Host:              "127.0.0.1",
		Port:              reserveDetectionUDPPort(t),
		HeartbeatInterval: 1,
	}
	module := NewDetectionUDPModule(detectionCfg, mocks.NewMockDataGenerator(0))

	done := make(chan struct{})
	go func() {
		_ = module.Start()
		close(done)
	}()
	defer func() {
		module.Stop()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("detection module did not stop in time")
		}
	}()

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: detectionCfg.Port})
	if err != nil {
		t.Fatalf("dial udp server: %v", err)
	}
	defer conn.Close()

	cases := []string{
		"hello\n",
		"start -fft 64 -band 70,6000, -gain 40\n",
	}

	for _, command := range cases {
		if _, err := conn.Write([]byte(command)); err != nil {
			t.Fatalf("write command %q: %v", command, err)
		}

		if err := conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}

		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("read ack for %q: %v", command, err)
		}

		got := string(buf[:n])
		want := buildDetectorCommandAck(detectionCfg.DeviceID, strings.TrimSpace(command))
		if got != want {
			t.Fatalf("unexpected ack for %q: got %q want %q", command, got, want)
		}
	}
}

func TestDetectionUDPServerRestartsSpectrumAfterStop(t *testing.T) {
	originalCfg := config.GetConfig()
	cfg := config.DefaultConfig()
	cfg.MinPushSpeed = 20
	cfg.MaxPushSpeed = 20
	config.SetGlobalConfig(cfg)
	defer config.SetGlobalConfig(originalCfg)

	detectionCfg := config.DetectionConfig{
		Enabled:           true,
		DeviceID:          2000,
		DroneCount:        0,
		Host:              "127.0.0.1",
		Port:              reserveDetectionUDPPort(t),
		HeartbeatInterval: 1,
	}
	module := NewDetectionUDPModule(detectionCfg, mocks.NewMockDataGenerator(0))

	done := make(chan struct{})
	go func() {
		_ = module.Start()
		close(done)
	}()
	defer func() {
		module.Stop()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("detection module did not stop in time")
		}
	}()

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: detectionCfg.Port})
	if err != nil {
		t.Fatalf("dial udp server: %v", err)
	}
	defer conn.Close()

	startCommand := []byte("start -fft 64 -band 3600,5800, \n")
	stopCommand := []byte("start -fft 0,\n")

	firstFreq := waitForSpectrumFrame(t, conn, startCommand, 2*time.Second)
	if firstFreq == 0 {
		t.Fatal("expected first spectrum frame")
	}

	if _, err := conn.Write(stopCommand); err != nil {
		t.Fatalf("write stop command: %v", err)
	}
	drainUDP(t, conn, 80*time.Millisecond)

	secondFreq := waitForSpectrumFrame(t, conn, startCommand, 2*time.Second)
	if secondFreq == 0 {
		t.Fatal("expected spectrum frame after restart")
	}
}

func waitForSpectrumFrame(t *testing.T, conn *net.UDPConn, command []byte, timeout time.Duration) uint32 {
	t.Helper()

	buf := make([]byte, 512)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := conn.Write(command); err != nil {
			continue
		}
		if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			continue
		}
		if n == 4+64*2 {
			return binary.BigEndian.Uint32(buf[:4])
		}
	}

	return 0
}

func TestSpectrumAnalysisPushIntervalMatchesRealDetector(t *testing.T) {
	command := mustParseDetectionCommand(t, "start -fft 64 -band 70,6000, -gain 40\n")

	for range 50 {
		interval := nextDetectionPushInterval(command, 300, 1000)
		if interval < 38 || interval > 90 {
			t.Fatalf("expected spectrum interval to be within real detector range, got %d", interval)
		}
	}

	normalCommand := mustParseDetectionCommand(t, "start -freq 2450 -set_ant 255,-turn_on_gpio 3\n")
	for range 10 {
		interval := nextDetectionPushInterval(normalCommand, 300, 1000)
		if interval < 300 || interval > 1000 {
			t.Fatalf("expected normal detection interval to use configured range, got %d", interval)
		}
	}
}

func mustParseDetectionCommand(t *testing.T, raw string) *utils.DetectionCommand {
	t.Helper()

	commandString := utils.DetectionCommandString(raw)
	command, err := commandString.ParseCommand()
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	return command
}

func drainUDP(t *testing.T, conn *net.UDPConn, duration time.Duration) {
	t.Helper()

	buf := make([]byte, 512)
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, err := conn.Read(buf)
		if err == nil {
			continue
		}
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			continue
		}
		t.Fatalf("drain udp: %v", err)
	}
}
