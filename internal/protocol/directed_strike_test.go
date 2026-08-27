package protocol

import (
	"bytes"
	"errors"
	"testing"
)

func TestDirectedStrikeFrameRoundTrip(t *testing.T) {
	t.Parallel()

	payload := []byte{0x00, DirectedFrameStart, directedEscapeByte, 0x01}
	raw := BuildDirectedStrikeFrame(DirectedFuncBroadcast, 0x05, DirectedCmdWriteFrequency, 0x00, payload)

	frame, err := ParseDirectedStrikeFrame(raw)
	if err != nil {
		t.Fatalf("ParseDirectedStrikeFrame returned error: %v", err)
	}
	if frame.DeviceAddr != 0x05 || frame.Command != DirectedCmdWriteFrequency {
		t.Fatalf("unexpected frame header: %#v", frame)
	}
	if !bytes.Equal(frame.Data, payload) {
		t.Fatalf("unexpected payload: got=% X want=% X", frame.Data, payload)
	}
}

func TestIsDirectedStrikeFrameCompleteIgnoresEmbeddedFrameEnd(t *testing.T) {
	t.Parallel()

	raw := BuildDirectedStrikeResponse(0x02, DirectedCmdAmpStatusQuery, DirectedResponseSuccess, []byte{
		0x7F, 0x01, 0x06, 0x23,
		0x6D, 0x60,
		0x13, 0x88,
		0x01, 0x2C,
		0x00, 0x05, 0xDB, 0x06, 0x05, 0x02, 0x00, 0x64, 0x00, 0x00,
	})
	embeddedEnd := bytes.IndexByte(raw[1:], DirectedFrameEnd)
	if embeddedEnd < 0 {
		t.Fatal("expected frame payload to contain an embedded frame-end byte")
	}
	embeddedEnd++

	if IsDirectedStrikeFrameComplete(raw[:embeddedEnd+1]) {
		t.Fatal("embedded frame-end byte must not complete the frame")
	}
	if !IsDirectedStrikeFrameComplete(raw) {
		t.Fatal("expected the full frame to be complete")
	}
}

func TestParseDirectedStrikeFrameRejectsChecksumMismatch(t *testing.T) {
	t.Parallel()

	raw := BuildDirectedStrikeResponse(0x02, DirectedCmdHeartbeat, DirectedResponseSuccess, nil)
	raw[len(raw)-2]++

	_, err := ParseDirectedStrikeFrame(raw)
	if !errors.Is(err, ErrDirectedFrameChecksum) {
		t.Fatalf("expected checksum error, got %v", err)
	}
}
