package modules

import (
	"net"
	"strings"
	"testing"
	"time"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/mocks"
)

func TestParseFingerprintCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOK    bool
		wantKind  fingerprintCommandKind
		frequency int
		rawName   string
		index     int
	}{
		{name: "capability", input: "-train\n", wantOK: true, wantKind: fingerprintCommandCapability},
		{name: "train", input: "-train 2455\n", wantOK: true, wantKind: fingerprintCommandTrain, frequency: 2455},
		{name: "save", input: "-save type_xiaomi8\n", wantOK: true, wantKind: fingerprintCommandSave, rawName: "type_xiaomi8"},
		{name: "list", input: "-list_type\n", wantOK: true, wantKind: fingerprintCommandList},
		{name: "delete", input: "-del_type 2\n", wantOK: true, wantKind: fingerprintCommandDelete, index: 2},
		{name: "frequency too low", input: "-train 69", wantOK: false},
		{name: "unsafe save name", input: "-save type_a;reboot", wantOK: false},
		{name: "delete index zero", input: "-del_type 0", wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command, ok := parseFingerprintCommand(test.input)
			if ok != test.wantOK {
				t.Fatalf("parse ok = %v, want %v", ok, test.wantOK)
			}
			if !ok {
				return
			}
			if command.kind != test.wantKind || command.frequency != test.frequency || command.name != test.rawName || command.index != test.index {
				t.Fatalf("unexpected command: %+v", command)
			}
		})
	}
}

func TestDetectionUDPServerSimulatesFingerprintTrainingLifecycle(t *testing.T) {
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
	time.Sleep(20 * time.Millisecond)

	writeFingerprintCommand(t, conn, "-train\n")
	if got := waitForUDPText(t, conn, 2*time.Second, func(message string) bool {
		return strings.Contains(message, "support AI")
	}); got == "" {
		t.Fatal("timed out waiting for capability response")
	}

	writeFingerprintCommand(t, conn, "-train 2455\n")
	sample := waitForUDPText(t, conn, 2*time.Second, func(message string) bool {
		return strings.Contains(message, "freq=") && strings.Contains(message, "Confidence=")
	})
	if sample == "" {
		t.Fatal("timed out waiting for fingerprint training sample")
	}

	writeFingerprintCommand(t, conn, "-save type_xiaomi8\n")
	if got := waitForUDPText(t, conn, time.Second, func(message string) bool {
		return strings.Contains(message, "type_xiaomi8 is saved")
	}); got == "" {
		t.Fatal("timed out waiting for explicit saved response")
	}

	writeFingerprintCommand(t, conn, "-list_type\n")
	list := waitForUDPText(t, conn, time.Second, func(message string) bool {
		return strings.Contains(message, "the pattern is list below:")
	})
	if !strings.Contains(list, "1 type_xiaomi8") {
		t.Fatalf("unexpected fingerprint list: %q", list)
	}

	writeFingerprintCommand(t, conn, "start -freq 2455 -set_ant 255,-turn_on_gpio 3\n")
	detection := waitForUDPText(t, conn, 2*time.Second, func(message string) bool {
		return strings.Contains(message, "model=type_xiaomi8 (user defined)")
	})
	if detection == "" {
		t.Fatal("timed out waiting for user-defined ordinary detection result")
	}

	writeFingerprintCommand(t, conn, "-del_type 1\n")
	list = waitForUDPText(t, conn, time.Second, func(message string) bool {
		return strings.Contains(message, "the pattern is list below:")
	})
	if list == "" || strings.Contains(list, "type_xiaomi8") {
		t.Fatalf("fingerprint was not deleted: %q", list)
	}
}

func writeFingerprintCommand(t *testing.T, conn *net.UDPConn, command string) {
	t.Helper()
	if _, err := conn.Write([]byte(command)); err != nil {
		t.Fatalf("write command %q: %v", command, err)
	}
}

func waitForUDPText(t *testing.T, conn *net.UDPConn, timeout time.Duration, accept func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 2048)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			t.Fatalf("read udp response: %v", err)
		}
		message := string(buf[:n])
		if accept(message) {
			return message
		}
	}
	return ""
}
