package modules

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/mocks"
	"virtual-device-ui/internal/utils"
)

type stubStringer string

func (s stubStringer) String() string {
	return string(s)
}

type stubMockGenerator struct{}

func (stubMockGenerator) GenerateAnalysisData(int) fmt.Stringer {
	return stubStringer("")
}

func (stubMockGenerator) GenerateDetectionData(int, *utils.DetectionCommand) fmt.Stringer {
	return stubStringer("")
}

func (stubMockGenerator) GenerateSpectrumFrame(*utils.DetectionCommand) []byte {
	return nil
}

func (stubMockGenerator) GenerateDirectionHeartbeatData(int) fmt.Stringer {
	return stubStringer("")
}

func (stubMockGenerator) GenerateFPVData() mocks.FPVData {
	return mocks.FPVData("SET+OK\n")
}

func (stubMockGenerator) GenerateFPVWarningData() fmt.Stringer {
	return mocks.FPVWarningData("Waring,Freq 5800,RSSI 0.55\r\n")
}

func (stubMockGenerator) GenerateDeviceID() int {
	return 2000
}

func (stubMockGenerator) RefreshRandomDrones() {}

func (stubMockGenerator) TotalDroneCount() int {
	return 0
}

func TestFPVSupportsPlatformCommandsAndWarningPush(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	originalCfg := config.GetConfig()
	cfg := config.DefaultConfig()
	cfg.MinPushSpeed = 20
	cfg.MaxPushSpeed = 20
	cfg.FPV.Hosts = []string{"127.0.0.1"}
	cfg.FPV.Port = listener.Addr().(*net.TCPAddr).Port
	config.SetGlobalConfig(cfg)
	defer config.SetGlobalConfig(originalCfg)

	module := NewFPVModule(cfg.FPV.DeviceID, stubMockGenerator{})
	moduleDone := make(chan struct{})
	go func() {
		module.Start()
		close(moduleDone)
	}()
	defer func() {
		module.Stop()
		select {
		case <-moduleDone:
		case <-time.After(2 * time.Second):
			t.Fatal("fpv module did not stop in time")
		}
	}()

	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		t.Fatalf("unexpected listener type %T", listener)
	}
	if err := tcpListener.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set accept deadline failed: %v", err)
	}

	conn, err := listener.Accept()
	if err != nil {
		t.Fatalf("accept failed: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	line := readLineContaining(t, conn, reader, 2*time.Second, "AT+OK")
	if strings.TrimSpace(line) != "AT+OK" {
		t.Fatalf("unexpected initial response: %q", line)
	}

	writeCommand(t, conn, "AT\r\n")
	line = readLineContaining(t, conn, reader, 2*time.Second, "AT+OK")
	if strings.TrimSpace(line) != "AT+OK" {
		t.Fatalf("unexpected AT response: %q", line)
	}

	writeCommand(t, conn, "AT+POINT_FREQ=5800\r\n")
	line = readLineContaining(t, conn, reader, 2*time.Second, "SET+OK")
	if strings.TrimSpace(line) != "SET+OK" {
		t.Fatalf("unexpected point frequency response: %q", line)
	}

	writeCommand(t, conn, "AT+DEFAULT\r\n")
	line = readLineContaining(t, conn, reader, 2*time.Second, "SET+OK")
	if strings.TrimSpace(line) != "SET+OK" {
		t.Fatalf("unexpected reset response: %q", line)
	}

	line = readLineContaining(t, conn, reader, 2*time.Second, "Waring,Freq")
	if !strings.Contains(line, "RSSI") {
		t.Fatalf("unexpected warning payload: %q", line)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if module.ConnectedCount() == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("expected FPV connection to remain active, got %d", module.ConnectedCount())
}

func writeCommand(t *testing.T, conn net.Conn, command string) {
	t.Helper()
	if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set write deadline failed: %v", err)
	}
	if _, err := conn.Write([]byte(command)); err != nil {
		t.Fatalf("write command %q failed: %v", command, err)
	}
}

func readLineContaining(t *testing.T, conn net.Conn, reader *bufio.Reader, timeout time.Duration, expected string) string {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline failed: %v", err)
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			t.Fatalf("read line failed: %v", err)
		}
		if strings.Contains(line, expected) {
			return line
		}
	}

	t.Fatalf("timed out waiting for line containing %q", expected)
	return ""
}
