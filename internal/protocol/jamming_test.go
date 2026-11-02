package protocol

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestJammingPacketLittleEndian(t *testing.T) {
	t.Parallel()
	wire, err := hex.DecodeString("aa01000804012155")
	if err != nil {
		t.Fatal(err)
	}
	packet, err := ParseJammingPacket(wire)
	if err != nil || packet.ID != 1 {
		t.Fatalf("parse ID: packet=%+v, err=%v", packet, err)
	}
	packet.ID = 0x1234
	want := []byte{0xaa, 0x34, 0x12, 8, 4, 1, 0x21, 0x55}
	if got := packet.ToBytes(); !bytes.Equal(got, want) {
		t.Fatalf("encoded packet = %x, want %x", got, want)
	}
}

func TestJammingResponseEncoding(t *testing.T) {
	t.Parallel()
	t.Run("module fields", func(t *testing.T) {
		packet := CreateModuleInfoResponse(1, []ModuleInfo{{
			FreqStart: 430, FreqEnd: 440, PowerDisp: 48, TempDisp: 48,
			SwitchSetting: 1, PowerSetting: 0x30, Alarm: 8, Reserved: 0x1234,
		}})
		want, _ := hex.DecodeString("aa01001105ae01b8013030013008341255")
		if got := packet.ToBytes(); !bytes.Equal(got, want) {
			t.Fatalf("module response = %x, want %x", got, want)
		}
	})
	t.Run("addresses", func(t *testing.T) {
		got := CreateAddrListResponse(1, CmdQueryAddr, []uint16{1, 0x1234}).ToBytes()
		want, _ := hex.DecodeString("aa01000a110100341255")
		if !bytes.Equal(got, want) {
			t.Fatalf("address response = %x, want %x", got, want)
		}
	})
}

func TestParseJammingPacketRejectsMalformedFrames(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		wire []byte
	}{
		{"short", []byte{0xaa, 1}},
		{"header", []byte{0, 1, 0, 6, 5, 0x55}},
		{"length", []byte{0xaa, 1, 0, 5, 5, 0x55}},
		{"truncated", []byte{0xaa, 1, 0, 7, 5, 0x55}},
		{"tail", []byte{0xaa, 1, 0, 6, 5, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseJammingPacket(tt.wire); err == nil {
				t.Fatal("malformed packet was accepted")
			}
		})
	}
}

func TestJammingStreamDecoder(t *testing.T) {
	t.Parallel()
	// Header/tail bytes in the payload must not be treated as frame boundaries.
	wire := []byte{0xaa, 1, 0, 8, 0x10, 0xaa, 0x55, 0x55}
	query := []byte{0xaa, 1, 0, 6, 5, 0x55}
	stream := append(bytes.Clone(wire), query...)
	for size := 1; size <= len(stream); size++ {
		t.Run(fmt.Sprintf("chunk size %d", size), func(t *testing.T) {
			var decoder JammingStreamDecoder
			var packets []*JammingPacket
			for offset := 0; offset < len(stream); offset += size {
				packets = append(packets, decoder.Feed(stream[offset:min(offset+size, len(stream))])...)
			}
			if len(packets) != 2 || !bytes.Equal(packets[0].ToBytes(), wire) || !bytes.Equal(packets[1].ToBytes(), query) {
				t.Fatalf("frames lost or corrupted: %+v", packets)
			}
			decoder.Feed(bytes.Repeat(query, 20))
			if !bytes.Equal(packets[0].ToBytes(), wire) {
				t.Fatal("a later read changed an earlier packet")
			}
		})
	}
	t.Run("recover after noise and bad frame", func(t *testing.T) {
		var decoder JammingStreamDecoder
		garbage := []byte{0, 0x55, 0xaa, 1, 0, 5, 0xaa, 1, 0, 6, 5, 0}
		packets := decoder.Feed(append(garbage, query...))
		if len(packets) != 1 || !bytes.Equal(packets[0].ToBytes(), query) {
			t.Fatalf("could not resynchronize: %+v", packets)
		}
	})
}
