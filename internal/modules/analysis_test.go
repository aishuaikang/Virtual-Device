package modules

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"regexp"
	"testing"
	"time"
	"virtual-device-ui/internal/config"
)

func TestAnalysisSendsPeriodicHeartbeatsOnExistingConnection(t *testing.T) {
	moduleConn, serverConn := net.Pipe()
	defer moduleConn.Close()
	defer serverConn.Close()

	moduleCtx, cancelModule := context.WithCancel(context.Background())
	defer cancelModule()
	sessionCtx, cancelSession := context.WithCancel(moduleCtx)

	const clientAddr = "127.0.0.1:10002"
	session := &analysisClientSession{
		conn:     moduleConn,
		ctx:      sessionCtx,
		cancel:   cancelSession,
		lastSeen: time.Now(),
		isActive: true,
	}
	module := &analysis{
		deviceId:          54221,
		title:             "解析模块",
		heartbeatInterval: 10 * time.Millisecond,
		config: &config.Config{
			MinPushSpeed: 60_000,
			MaxPushSpeed: 60_000,
		},
		mock:    stubMockGenerator{},
		ctx:     moduleCtx,
		cancel:  cancelModule,
		clients: map[string]*analysisClientSession{clientAddr: session},
	}

	done := make(chan struct{})
	go func() {
		module.handleReportData(clientAddr, session)
		close(done)
	}()

	if err := serverConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	reader := bufio.NewReader(serverConn)
	for sequence := 1; sequence <= 2; sequence++ {
		heartbeat, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read heartbeat %d: %v", sequence, err)
		}
		pattern := regexp.MustCompile(fmt.Sprintf(
			`^#=%d, device=54221, Heart Beat, 19[0-5][0-9]-(?:1[89]|[2-4][0-9]|5[0-5])\r\n$`,
			sequence,
		))
		if !pattern.MatchString(heartbeat) {
			t.Fatalf("unexpected heartbeat %d: %q", sequence, heartbeat)
		}
	}

	cancelSession()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("analysis sender did not stop after session cancellation")
	}
}

func TestFormatAnalysisHeartbeatMatchesReportedProtocol(t *testing.T) {
	got := formatAnalysisHeartbeat(54221, 140, 1940, 30)
	want := "#=140, device=54221, Heart Beat, 1940-30\r\n"
	if got != want {
		t.Fatalf("formatAnalysisHeartbeat() = %q, want %q", got, want)
	}
}
