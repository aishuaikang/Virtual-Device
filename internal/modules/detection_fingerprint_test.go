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
		name              string
		input             string
		wantOK            bool
		wantKind          fingerprintCommandKind
		authorizationCode string
		frequency         int
		rawName           string
		index             int
	}{
		{name: "capability", input: "-train\n", wantOK: true, wantKind: fingerprintCommandCapability},
		{name: "authorization", input: "-setfun f381d12cca7987fe\n", wantOK: true, wantKind: fingerprintCommandAuthorize, authorizationCode: "f381d12cca7987fe"},
		{name: "authorization uppercase", input: "-setfun F381D12CCA7987FE\n", wantOK: true, wantKind: fingerprintCommandAuthorize, authorizationCode: "F381D12CCA7987FE"},
		{name: "train", input: "-train 2455\n", wantOK: true, wantKind: fingerprintCommandTrain, frequency: 2455},
		{name: "save", input: "-save xiaomi8\n", wantOK: true, wantKind: fingerprintCommandSave, rawName: "xiaomi8"},
		{name: "list", input: "-list_type\n", wantOK: true, wantKind: fingerprintCommandList},
		{name: "delete", input: "-del_type 2\n", wantOK: true, wantKind: fingerprintCommandDelete, index: 2},
		{name: "authorization too short", input: "-setfun f381d12cca7987f", wantOK: false},
		{name: "authorization contains whitespace", input: "-setfun f381d12cca7987f e", wantOK: false},
		{name: "frequency too low", input: "-train 69", wantOK: false},
		{name: "unsafe save name", input: "-save type_a;reboot", wantOK: false},
		{name: "delete index zero", input: "-del_type 0", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, ok := parseFingerprintCommand(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parse ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if command.kind != tt.wantKind ||
				command.authorizationCode != tt.authorizationCode ||
				command.frequency != tt.frequency ||
				command.name != tt.rawName ||
				command.index != tt.index {
				t.Fatalf("unexpected command: %+v", command)
			}
		})
	}
}

func TestRedactDetectionCommandForLog(t *testing.T) {
	const code = "f381d12cca7987fe"
	if got := redactDetectionCommandForLog([]byte("-setfun " + code + "\n")); got != "-setfun [REDACTED]" {
		t.Fatalf("redacted command = %q", got)
	}
	if got := redactDetectionCommandForLog([]byte("-train\n")); got != "-train" {
		t.Fatalf("ordinary command = %q", got)
	}
}

func TestDetectionUDPServerSimulatesFingerprintAuthorizationAndTraining(t *testing.T) {
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
	waitForUDPText(t, conn, time.Second, func(message string) bool {
		return message == "The device does not support AI"
	})

	const wrongCode = "0000000000000000"
	writeFingerprintCommand(t, conn, "-setfun "+wrongCode+"\n")
	wrongAck := waitForUDPText(t, conn, time.Second, func(message string) bool {
		return strings.HasSuffix(message, "-setfun "+wrongCode)
	})
	if !strings.Contains(wrongAck, "dna(0)") || !strings.Contains(wrongAck, "b4") {
		t.Fatalf("unexpected authorization acknowledgment: %q", wrongAck)
	}
	writeFingerprintCommand(t, conn, "-train\n")
	waitForUDPText(t, conn, time.Second, func(message string) bool {
		return message == "The device does not support AI"
	})

	writeFingerprintCommand(t, conn, "-setfun "+simulatedFingerprintAuthorizationCode+"\n")
	ack := waitForUDPText(t, conn, time.Second, func(message string) bool {
		return strings.HasSuffix(message, "-setfun "+simulatedFingerprintAuthorizationCode)
	})
	if ack == "" {
		t.Fatal("timed out waiting for authorization acknowledgment")
	}
	for probe := 0; probe < 3; probe++ {
		writeFingerprintCommand(t, conn, "-train\n")
		if got := waitForUDPText(t, conn, time.Second, func(message string) bool {
			return message == "The device support AI"
		}); got == "" {
			t.Fatalf("probe %d did not report AI support", probe+1)
		}
	}

	writeFingerprintCommand(t, conn, "-train 2455\n")
	if sample := waitForUDPText(t, conn, 2*time.Second, func(message string) bool {
		return strings.Contains(message, "freq=2455.") && strings.Contains(message, "Confidence=")
	}); sample == "" {
		t.Fatal("timed out waiting for fingerprint training sample")
	}

	writeFingerprintCommand(t, conn, "-save xiaomi8\n")
	if saved := waitForUDPText(t, conn, time.Second, func(message string) bool {
		return strings.Contains(message, "xiaomi8 is saved")
	}); saved == "" {
		t.Fatal("timed out waiting for saved response")
	}
	if ack := waitForUDPText(t, conn, time.Second, func(message string) bool {
		return strings.HasSuffix(message, "-save xiaomi8")
	}); ack == "" {
		t.Fatal("timed out waiting for save acknowledgment")
	}
	writeFingerprintCommand(t, conn, "-list_type\n")
	list := waitForUDPText(t, conn, time.Second, func(message string) bool {
		return strings.Contains(message, "the pattern is list below:")
	})
	if !strings.Contains(list, "1 xiaomi8") {
		t.Fatalf("unexpected fingerprint list: %q", list)
	}
}

func TestFingerprintListResponseIncludesLegacyRawNames(t *testing.T) {
	server := &detectionUDPServer{fingerprints: []string{"test123", "type_530"}}
	response := server.fingerprintListResponse()
	if response != "the pattern is list below:\n1 test123\n2 type_530" {
		t.Fatalf("response=%q", response)
	}
}

func writeFingerprintCommand(t *testing.T, conn *net.UDPConn, command string) {
	t.Helper()
	if _, err := conn.Write([]byte(command)); err != nil {
		t.Fatalf("write fingerprint command: %v", err)
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
