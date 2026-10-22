package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

func TestPrecisionDFCommandBytes(t *testing.T) {
	frame, err := EncodePrecisionDFCommand("CLR")
	if err != nil {
		t.Fatal(err)
	}
	const expected = "555555555555555500000004434c520aaaaaaaaaaaaaaaaa"
	if hex.EncodeToString(frame) != expected {
		t.Fatalf("unexpected command bytes: %x", frame)
	}
	// A reader may return one byte at a time, or multiple consecutive frames.
	r := &precisionDFByteReader{data: append(append([]byte{}, frame...), frame...)}
	for i := 0; i < 2; i++ {
		got, err := ReadPrecisionDFCommand(r)
		if err != nil || got != "CLR\n" {
			t.Fatalf("command %d: %q, %v", i, got, err)
		}
	}
}

type precisionDFByteReader struct{ data []byte }

func (r *precisionDFByteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	p[0] = r.data[0]
	r.data = r.data[1:]
	return 1, nil
}

func TestPrecisionDFCommandRejectsInvalidFrames(t *testing.T) {
	valid, _ := EncodePrecisionDFCommand("MODE SFDF")
	for _, tc := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"header", func(b []byte) []byte { b[0] = 0; return b }},
		{"tail", func(b []byte) []byte { b[len(b)-1] = 0; return b }},
		{"oversize", func(b []byte) []byte { binary.BigEndian.PutUint32(b[8:12], 1005); return b }},
		{"zero length", func(b []byte) []byte { binary.BigEndian.PutUint32(b[8:12], 0); return b }},
		{"missing LF", func(b []byte) []byte { b[len(b)-9] = ' '; return b }},
		{"truncated", func(b []byte) []byte { return b[:len(b)-1] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ReadPrecisionDFCommand(bytes.NewReader(tc.mutate(bytes.Clone(valid)))); err == nil {
				t.Fatal("invalid frame was accepted")
			}
		})
	}
	for _, command := range []string{strings.Repeat("x", 1004), "MODE\r\n", "中文"} {
		if _, err := EncodePrecisionDFCommand(command); err == nil {
			t.Fatalf("invalid command accepted: %q", command)
		}
	}
}

func TestPrecisionDFDataLayout(t *testing.T) {
	f := EncodePrecisionDFFrame(PrecisionDFFrame{
		Sequence: 9, Business: 0xd5, DataType: 104, TaskID: 123, Antenna: 2,
		SampleRate: 102400000, FFTSize: 1024, BandwidthHz: 80000000,
		FrequencyHz: 5800000000, Total: 257, Index: 256, StartHz: 5700000000, EndHz: 5900000000,
		Timestamp: time.Unix(1700000000, 123456789), Payload: make([]byte, 12),
	})
	if len(f) != 332 || !bytes.Equal(f[:8], []byte{0, 128, 0, 128, 0, 128, 0, 128}) ||
		!bytes.Equal(f[len(f)-8:], []byte{255, 127, 255, 127, 255, 127, 255, 127}) {
		t.Fatalf("invalid data envelope: %x", f)
	}
	for _, tc := range []struct {
		name   string
		offset int
		value  uint32
	}{
		{"sequence", 8, 9}, {"payload length", 12, 12}, {"channel", 16, 1},
		{"business", 20, 0xd5}, {"bandwidth", 24, 80000000}, {"type", 28, 104},
		{"frequency low", 32, uint32(uint64(5800000000) & 0xffffffff)}, {"frequency high", 36, 1},
		{"bit depth", 40, 32}, {"sample rate", 68, 102400000}, {"resolution", 72, 100000},
		{"seconds", 80, 1700000000}, {"nanoseconds", 84, 123456789},
		{"final scan segment", 88, 4}, {"task", 108, 123}, {"antenna", 132, 2},
		{"fft size", 140, 1024}, {"not first frame", 272, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := binary.LittleEndian.Uint32(f[tc.offset:]); got != tc.value {
				t.Fatalf("offset %d = %d, want %d", tc.offset, got, tc.value)
			}
		})
	}
	if binary.LittleEndian.Uint64(f[92:]) != 257 || binary.LittleEndian.Uint64(f[100:]) != 256 ||
		math.Float64frombits(binary.LittleEndian.Uint64(f[184:])) != 5700000000 ||
		math.Float64frombits(binary.LittleEndian.Uint64(f[192:])) != 5900000000 {
		t.Fatal("invalid scan indices or frequency range")
	}
}
