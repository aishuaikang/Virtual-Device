package modules

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestRelay16CoilRequests(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		initial uint16
		pdu     []byte
		reply   []byte
		output  uint16
	}{
		{"read empty board", 0, []byte{1, 0, 0, 0, 16}, []byte{1, 2, 0, 0}, 0},
		{"read all outputs", 0x9249, []byte{1, 0, 0, 0, 16}, []byte{1, 2, 0x49, 0x92}, 0x9249},
		{"read offset across bytes", 0x9249, []byte{1, 0, 4, 0, 9}, []byte{1, 2, 0x24, 1}, 0x9249},
		{"write combination preserves IO16", 0x8000, []byte{15, 0, 0, 0, 15, 2, 0x49, 0x12}, []byte{15, 0, 0, 0, 15}, 0x9249},
		{"stop preserves IO16", 0xffff, []byte{15, 0, 0, 0, 15, 2, 0, 0}, []byte{15, 0, 0, 0, 15}, 0x8000},
		{"partial write preserves other coils", 0x8001, []byte{15, 0, 3, 0, 3, 1, 5}, []byte{15, 0, 3, 0, 3}, 0x8029},
		{"board can report reserved output", 0, []byte{15, 0, 15, 0, 1, 1, 1}, []byte{15, 0, 15, 0, 1}, 0x8000},
		{"unsupported function", 1, []byte{5, 0, 0, 0xff, 0}, []byte{0x85, 1}, 1},
		{"zero quantity", 1, []byte{1, 0, 0, 0, 0}, []byte{0x81, 3}, 1},
		{"out of range", 1, []byte{1, 0, 15, 0, 2}, []byte{0x81, 2}, 1},
		{"address overflow", 1, []byte{15, 0xff, 0xff, 0, 2, 1, 3}, []byte{0x8f, 2}, 1},
		{"short read", 1, []byte{1, 0, 0}, []byte{0x81, 3}, 1},
		{"extra read data", 1, []byte{1, 0, 0, 0, 16, 0}, []byte{0x81, 3}, 1},
		{"missing write data", 1, []byte{15, 0, 0, 0, 1}, []byte{0x8f, 3}, 1},
		{"wrong byte count", 1, []byte{15, 0, 0, 0, 15, 1, 0xff, 0x7f}, []byte{0x8f, 3}, 1},
		{"short write data", 1, []byte{15, 0, 0, 0, 15, 2, 0xff}, []byte{0x8f, 3}, 1},
		{"extra write data", 1, []byte{15, 0, 0, 0, 1, 1, 0xff, 0}, []byte{0x8f, 3}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module := &Relay16Module{ctx: context.Background(), outputMask: tc.initial}
			if got := module.handlePDU(tc.pdu); !bytes.Equal(got, tc.reply) {
				t.Fatalf("reply = % X, want % X", got, tc.reply)
			}
			if got := module.Snapshot().OutputMask; got != tc.output {
				t.Fatalf("output = %04X, want %04X", got, tc.output)
			}
		})
	}
}

func TestRelay16StoppedBoardCannotReenableOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	module := &Relay16Module{ctx: ctx}
	reply := module.handlePDU([]byte{15, 0, 0, 0, 15, 2, 0xff, 0x7f})
	if !bytes.Equal(reply, []byte{0x8f, 4}) || module.Snapshot().OutputMask != 0 {
		t.Fatalf("stopped board accepted write: % X", reply)
	}
}

func TestRelay16ShortLivedPollingConnections(t *testing.T) {
	module := &Relay16Module{listening: true, lastActivityAt: time.Now()}
	if !module.Snapshot().Connected {
		t.Fatal("recent polling was reported as disconnected")
	}
	module.lastActivityAt = time.Now().Add(-6 * time.Second)
	if module.Snapshot().Connected {
		t.Fatal("stale polling was reported as connected")
	}
}
