package modules

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"regexp"
	"testing"
	"time"
	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/mocks"
)

type analysisReportGenerator struct {
	stubMockGenerator
	report func(generation, deviceID int) string
	calls  int
}

func (g *analysisReportGenerator) GenerateAnalysisData(deviceID int) fmt.Stringer {
	g.calls++
	return stubStringer(g.report(g.calls, deviceID))
}

func TestAnalysisSendsSameReportForEachDeviceID(t *testing.T) {
	for _, tc := range []struct {
		name      string
		deviceIDs []int
		report    func(generation, deviceID int) string
	}{
		{
			name:      "single ID",
			deviceIDs: []int{2222},
			report: func(generation, deviceID int) string {
				return fmt.Sprintf("num=%d, device=%d, serial=drone-1, freq=2437.0, rssi=-65,\r\n", generation, deviceID)
			},
		},
		{
			name:      "multiple IDs",
			deviceIDs: []int{2222, 3333},
			report: func(generation, deviceID int) string {
				return fmt.Sprintf("num=%d, device=%d, serial=drone-1, freq=2437.0, rssi=-65,\r\n", generation, deviceID)
			},
		},
		{
			name:      "encrypted report",
			deviceIDs: []int{2222, 3333, 4444},
			report: func(generation, deviceID int) string {
				return fmt.Sprintf("#=%d, device=%d, Encypted Mavic_O4_ID=30a958e6, freq=5796.5, rssi=-84, byte,55,c6,9b,a7\r\n", generation, deviceID)
			},
		},
		{
			name:      "mixed packet",
			deviceIDs: []int{2222, 3333},
			report: func(generation, deviceID int) string {
				return fmt.Sprintf("wifi mac=60:60:1f:34:be:99, ssid=Hidden, rssi=-60, freq=2437\r\nnum=%d, device=%d, serial=drone-1, freq=2437.0, rssi=-65,\r\n#=%d, device=%d, Encypted Mavic_O4_ID=30a958e6, freq=5796.5, rssi=-84, byte,55,c6,9b,a7\r\n", generation, deviceID, generation, deviceID)
			},
		},
		{
			name:      "reports without device fields preserve their format",
			deviceIDs: []int{2222, 3333},
			report: func(generation, _ int) string {
				return fmt.Sprintf("RID ssid=RID-drone-%d, serial=drone-1, rssi=-65, freq=2437.0\r\nEmpty packet, freq=2414.5, rssi=-47\r\nwifi mac=60:60:1f:34:be:99, ssid=Hidden, rssi=-60, freq=2437\r\n", generation)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			generator := &analysisReportGenerator{report: tc.report}
			serverConn := startAnalysisSender(t, tc.deviceIDs, generator, 20, time.Hour)
			for _, deviceID := range tc.deviceIDs {
				want := tc.report(1, deviceID)
				got := make([]byte, len(want))
				if _, err := io.ReadFull(serverConn, got); err != nil {
					t.Fatalf("read report for device %d: %v", deviceID, err)
				}
				if string(got) != want {
					t.Fatalf("report for device %d = %q, want %q", deviceID, got, want)
				}
			}
		})
	}
}

func TestAnalysisSendsPeriodicHeartbeatsOnExistingConnection(t *testing.T) {
	for _, tc := range []struct {
		name      string
		deviceIDs []int
	}{
		{"single ID", []int{54221}},
		{"multiple IDs", []int{2222, 3333}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serverConn := startAnalysisSender(t, tc.deviceIDs, stubMockGenerator{}, 60_000, 10*time.Millisecond)
			reader := bufio.NewReader(serverConn)
			for sequence := 1; sequence <= 2; sequence++ {
				for _, deviceID := range tc.deviceIDs {
					heartbeat, err := reader.ReadString('\n')
					if err != nil {
						t.Fatalf("read heartbeat %d for device %d: %v", sequence, deviceID, err)
					}
					pattern := regexp.MustCompile(fmt.Sprintf(
						`^#=%d, device=%d, Heart Beat, 19[0-5][0-9]-(?:1[89]|[2-4][0-9]|5[0-5])\r\n$`,
						sequence, deviceID,
					))
					if !pattern.MatchString(heartbeat) {
						t.Fatalf("unexpected heartbeat %d for device %d: %q", sequence, deviceID, heartbeat)
					}
				}
			}
		})
	}
}

func startAnalysisSender(t *testing.T, deviceIDs []int, mock mocks.MockDataGenerator, reportIntervalMS int, heartbeatInterval time.Duration) net.Conn {
	t.Helper()
	moduleConn, serverConn := net.Pipe()
	module := NewAnalysisModule(deviceIDs, mock).(*analysis)
	module.config = &config.Config{
		MinPushSpeed: reportIntervalMS,
		MaxPushSpeed: reportIntervalMS,
	}
	module.heartbeatInterval = heartbeatInterval

	const clientAddr = "127.0.0.1:10002"
	session, _ := module.getOrCreateSession(clientAddr)
	if !module.attachSessionConn(clientAddr, session, moduleConn) {
		t.Fatal("attach analysis connection")
	}
	done := make(chan struct{})
	go func() {
		module.handleReportData(clientAddr, session)
		close(done)
	}()
	t.Cleanup(func() {
		module.Stop()
		serverConn.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("analysis sender did not stop after cancellation")
		}
	})
	if err := serverConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	return serverConn
}

func TestFormatAnalysisHeartbeatMatchesReportedProtocol(t *testing.T) {
	got := formatAnalysisHeartbeat(54221, 140, 1940, 30)
	want := "#=140, device=54221, Heart Beat, 1940-30\r\n"
	if got != want {
		t.Fatalf("formatAnalysisHeartbeat() = %q, want %q", got, want)
	}
}
