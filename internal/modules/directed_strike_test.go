package modules

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"virtual-device-ui/internal/protocol"
)

func TestReadDirectedStrikeFrameKeepsEmbeddedFrameEndByte(t *testing.T) {
	t.Parallel()

	raw := protocol.BuildDirectedStrikeResponse(0x02, protocol.DirectedCmdAmpStatusQuery, protocol.DirectedResponseSuccess, []byte{
		0x7F, 0x01, 0x06, 0x23,
		0x6D, 0x60,
		0x13, 0x88,
		0x01, 0x2C,
		0x00, 0x05, 0xDB, 0x06, 0x05, 0x02, 0x00, 0x64, 0x00, 0x00,
	})

	got, err := readDirectedStrikeFrame(bufio.NewReader(bytes.NewReader(raw)))
	if err != nil {
		t.Fatalf("readDirectedStrikeFrame returned error: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("read frame mismatch:\n got: % X\nwant: % X", got, raw)
	}
}

func TestDirectedStrikeModuleRejectsConnectionAfterStop(t *testing.T) {
	moduleValue, err := NewDirectedStrikeModule("127.0.0.1:0", 0)
	if err != nil {
		t.Fatalf("NewDirectedStrikeModule returned error: %v", err)
	}
	module := moduleValue.(*directedStrike)
	module.Stop()

	client, server := net.Pipe()
	defer client.Close()
	if module.registerConnection(server) {
		t.Fatal("expected connection registration after stop to be rejected")
	}
	if snapshot := module.Snapshot(); snapshot.ActiveConnections != 0 {
		t.Fatalf("expected no registered connections after stop, got %d", snapshot.ActiveConnections)
	}
}

func TestDirectedStrikeModuleProtocolFlow(t *testing.T) {
	moduleValue, err := NewDirectedStrikeModule("127.0.0.1:0", 0)
	if err != nil {
		t.Fatalf("NewDirectedStrikeModule returned error: %v", err)
	}
	module := moduleValue.(*directedStrike)
	done := make(chan struct{})
	go func() {
		module.Start()
		close(done)
	}()
	t.Cleanup(func() {
		module.Stop()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("directed strike module did not stop")
		}
	})

	connection, err := net.DialTimeout("tcp", module.Snapshot().ListenAddress, time.Second)
	if err != nil {
		t.Fatalf("dial directed strike module: %v", err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set connection deadline: %v", err)
	}
	reader := bufio.NewReader(connection)

	tests := []struct {
		name            string
		command         byte
		deviceAddress   byte
		data            []byte
		responseCommand byte
	}{
		{
			name:            "heartbeat",
			command:         protocol.DirectedCmdHeartbeat,
			deviceAddress:   0x00,
			responseCommand: protocol.DirectedCmdHeartbeat,
		},
		{
			name:            "enable signal source",
			command:         protocol.DirectedCmdSignalSource,
			deviceAddress:   0xFF,
			data:            []byte{protocol.DirectedOperationPowerLoss, 0x05, protocol.DirectedSwitchOn},
			responseCommand: protocol.DirectedCmdSignalSource,
		},
		{
			name:            "write wideband frequency",
			command:         protocol.DirectedCmdWriteFrequency,
			deviceAddress:   0xFF,
			data:            []byte{protocol.DirectedOperationPowerRetain, 0x05, 0x07, 0x08, 0x07, 0x08},
			responseCommand: protocol.DirectedCmdWriteFrequency,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := protocol.BuildDirectedStrikeFrame(
				protocol.DirectedFuncBroadcast,
				test.deviceAddress,
				test.command,
				0x00,
				test.data,
			)
			if _, err := connection.Write(request); err != nil {
				t.Fatalf("write request: %v", err)
			}
			responseRaw, err := readDirectedStrikeFrame(reader)
			if err != nil {
				t.Fatalf("read response: %v", err)
			}
			response, err := protocol.ParseDirectedStrikeFrame(responseRaw)
			if err != nil {
				t.Fatalf("parse response: %v", err)
			}
			if response.Command != test.responseCommand || response.Response != protocol.DirectedResponseSuccess {
				t.Fatalf("unexpected response: %#v", response)
			}
		})
	}

	snapshot := module.Snapshot()
	if snapshot.HeartbeatCount != 1 {
		t.Fatalf("expected one heartbeat, got %d", snapshot.HeartbeatCount)
	}
	if snapshot.ReceivedFrames != 3 || snapshot.SentFrames != 3 {
		t.Fatalf("unexpected frame counts: rx=%d tx=%d", snapshot.ReceivedFrames, snapshot.SentFrames)
	}
	if len(snapshot.SignalSources) != 4 || !snapshot.SignalSources[3].Enabled {
		t.Fatalf("expected source 0x05 to be enabled: %#v", snapshot.SignalSources)
	}
	if len(snapshot.WidebandConfigs) != 1 || snapshot.WidebandConfigs[0].StartFreqMHz != 6800 {
		t.Fatalf("unexpected frequency snapshot: %#v", snapshot.WidebandConfigs)
	}
}

func TestDirectedStrikeModulePTZAndAmpResponses(t *testing.T) {
	moduleValue, err := NewDirectedStrikeModule("127.0.0.1:0", 0)
	if err != nil {
		t.Fatalf("NewDirectedStrikeModule returned error: %v", err)
	}
	module := moduleValue.(*directedStrike)
	t.Cleanup(module.Stop)

	module.mu.Lock()
	module.signalSources[0x02] = true
	module.ptz.horizontalAngle = 123.45
	module.ptz.pitchAngle = -12.3
	module.ptz.motionUpdatedAt = time.Now()
	module.mu.Unlock()

	ampResponses, err := module.handleFrame(protocol.DirectedStrikeFrame{
		DeviceAddr: 0x02,
		Command:    protocol.DirectedCmdAmpStatusQuery,
	})
	if err != nil {
		t.Fatalf("handle amplifier status query: %v", err)
	}
	ampResponse := parseSingleDirectedStrikeResponse(t, ampResponses)
	if len(ampResponse.Data) != 20 || ampResponse.Data[1] != protocol.DirectedSwitchOn {
		t.Fatalf("unexpected amplifier response: %#v", ampResponse)
	}

	horizontalResponses, err := module.handleFrame(protocol.DirectedStrikeFrame{
		DeviceAddr: 0x01,
		Command:    protocol.DirectedCmdPTZHorizontalQuery,
	})
	if err != nil {
		t.Fatalf("handle horizontal query: %v", err)
	}
	horizontalResponse := parseSingleDirectedStrikeResponse(t, horizontalResponses)
	if horizontalResponse.Command != protocol.DirectedCmdPTZHorizontalResponse || len(horizontalResponse.Data) != 2 {
		t.Fatalf("unexpected horizontal response: %#v", horizontalResponse)
	}
	if got := binary.BigEndian.Uint16(horizontalResponse.Data); got != 12345 {
		t.Fatalf("unexpected horizontal angle encoding: got=%d want=12345", got)
	}

	pitchResponses, err := module.handleFrame(protocol.DirectedStrikeFrame{
		DeviceAddr: 0x01,
		Command:    protocol.DirectedCmdPTZPitchQuery,
	})
	if err != nil {
		t.Fatalf("handle pitch query: %v", err)
	}
	pitchResponse := parseSingleDirectedStrikeResponse(t, pitchResponses)
	if pitchResponse.Command != protocol.DirectedCmdPTZPitchResponse || len(pitchResponse.Data) != 2 {
		t.Fatalf("unexpected pitch response: %#v", pitchResponse)
	}
	if got := binary.BigEndian.Uint16(pitchResponse.Data); got != 1230 {
		t.Fatalf("unexpected pitch angle encoding: got=%d want=1230", got)
	}

	noResponses, err := module.handleFrame(protocol.DirectedStrikeFrame{
		DeviceAddr: 0x01,
		Command:    protocol.DirectedCmdPTZSpeedSet,
		Data:       []byte{18, 20},
	})
	if err != nil {
		t.Fatalf("handle PTZ speed set: %v", err)
	}
	if len(noResponses) != 0 {
		t.Fatalf("PTZ speed set should not respond, got %d frames", len(noResponses))
	}

	speedResponses, err := module.handleFrame(protocol.DirectedStrikeFrame{
		DeviceAddr: 0x01,
		Command:    protocol.DirectedCmdPTZSpeedQuery,
	})
	if err != nil {
		t.Fatalf("handle PTZ speed query: %v", err)
	}
	speedResponse := parseSingleDirectedStrikeResponse(t, speedResponses)
	if len(speedResponse.Data) != 2 || speedResponse.Data[0] != 18 || speedResponse.Data[1] != 20 {
		t.Fatalf("unexpected PTZ speed response: %#v", speedResponse)
	}
}

func parseSingleDirectedStrikeResponse(t *testing.T, responses [][]byte) protocol.DirectedStrikeFrame {
	t.Helper()
	if len(responses) != 1 {
		t.Fatalf("expected one response, got %d", len(responses))
	}
	response, err := protocol.ParseDirectedStrikeFrame(responses[0])
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	return response
}
