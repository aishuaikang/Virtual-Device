package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

// Receiver v0.22 uses big-endian ASCII command envelopes and little-endian data.
func EncodePrecisionDFCommand(command string) ([]byte, error) {
	command = strings.TrimSuffix(command, "\n") + "\n"
	if len(command) > 1004 || strings.Count(command, "\n") > 100 {
		return nil, fmt.Errorf("command exceeds receiver frame limit")
	}
	for _, b := range []byte(command) {
		if b > 127 || (b < 32 && b != '\n') {
			return nil, fmt.Errorf("command must be ASCII with LF terminators")
		}
	}
	frame := make([]byte, len(command)+20)
	copy(frame, bytes.Repeat([]byte{0x55}, 8))
	binary.BigEndian.PutUint32(frame[8:12], uint32(len(command)))
	copy(frame[12:], command)
	copy(frame[12+len(command):], bytes.Repeat([]byte{0xaa}, 8))
	return frame, nil
}

func ReadPrecisionDFCommand(r io.Reader) (string, error) {
	var header [12]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return "", err
	}
	if !bytes.Equal(header[:8], bytes.Repeat([]byte{0x55}, 8)) {
		return "", fmt.Errorf("invalid receiver command header")
	}
	n := binary.BigEndian.Uint32(header[8:])
	if n == 0 || n > 1004 {
		return "", fmt.Errorf("invalid receiver command length: %d", n)
	}
	frame := make([]byte, int(n)+8)
	if _, err := io.ReadFull(r, frame); err != nil {
		return "", err
	}
	if !bytes.Equal(frame[n:], bytes.Repeat([]byte{0xaa}, 8)) {
		return "", fmt.Errorf("invalid receiver command tail")
	}
	text := string(frame[:n])
	if !strings.HasSuffix(text, "\n") {
		return "", fmt.Errorf("receiver command requires LF terminator")
	}
	if _, err := EncodePrecisionDFCommand(text); err != nil {
		return "", err
	}
	return text, nil
}

type PrecisionDFFrame struct {
	Sequence, Business, DataType, TaskID, Antenna, SampleRate, FFTSize uint32
	BusinessSequence                                                   uint32
	FrequencyHz                                                        uint64
	BandwidthHz                                                        uint32
	Total, Index                                                       uint64
	StartHz, EndHz                                                     float64
	Payload                                                            []byte
	Timestamp                                                          time.Time
}

// EncodePrecisionDFFrame builds the 288-byte header, payload and 32-byte tail.
func EncodePrecisionDFFrame(f PrecisionDFFrame) []byte {
	frame := make([]byte, 288+len(f.Payload)+32)
	copy(frame, []byte{0, 0x80, 0, 0x80, 0, 0x80, 0, 0x80})
	u := binary.LittleEndian.PutUint32
	u(frame[8:12], f.Sequence)
	u(frame[12:16], uint32(len(f.Payload)))
	u(frame[16:20], 1)
	u(frame[20:24], f.Business)
	u(frame[24:28], f.BandwidthHz)
	u(frame[28:32], f.DataType)
	u(frame[32:36], uint32(f.FrequencyHz))
	u(frame[36:40], uint32(f.FrequencyHz>>32)&15)
	bits := uint32(16)
	if f.DataType >= 101 {
		bits = 32
	}
	u(frame[40:44], bits)
	u(frame[44:48], f.BusinessSequence)
	u(frame[68:72], f.SampleRate)
	if f.FFTSize > 0 {
		u(frame[72:76], f.SampleRate/f.FFTSize)
	}
	if !f.Timestamp.IsZero() {
		u(frame[80:84], uint32(f.Timestamp.Unix()))
		u(frame[84:88], uint32(f.Timestamp.Nanosecond()))
	}
	binary.LittleEndian.PutUint64(frame[92:100], f.Total)
	binary.LittleEndian.PutUint64(frame[100:108], f.Index)
	u(frame[108:112], f.TaskID)
	u(frame[132:136], f.Antenna)
	u(frame[140:144], f.FFTSize)
	binary.LittleEndian.PutUint64(frame[184:192], math.Float64bits(f.StartHz))
	binary.LittleEndian.PutUint64(frame[192:200], math.Float64bits(f.EndHz))
	flags := uint32(0)
	if f.Index == 0 {
		flags = 1
	}
	pointSize := uint64(2)
	if f.DataType == 104 {
		pointSize = 12
	} else if f.DataType == 1 {
		pointSize = 4
	}
	if f.Index+uint64(len(f.Payload))/pointSize >= f.Total {
		flags |= 4
	} else if f.Index > 0 {
		flags |= 2
	}
	u(frame[88:92], flags)
	if f.Index == 0 {
		u(frame[272:276], 1)
	}
	copy(frame[288:], f.Payload)
	copy(frame[len(frame)-8:], []byte{0xff, 0x7f, 0xff, 0x7f, 0xff, 0x7f, 0xff, 0x7f})
	return frame
}

// PrecisionDFSampleRate follows the bandwidth table in receiver manual v0.22.
// Fractional rates are preserved for scan frequency grids.
func PrecisionDFSampleRate(bandwidth uint32) float64 {
	return map[uint32]float64{
		320000000: 409600000, 160000000: 204800000, 80000000: 102400000,
		40000000: 51200000, 20000000: 25600000, 10000000: 12800000,
		5000000: 6400000, 2000000: 3200000, 1000000: 1600000,
		500000: 800000, 300000: 400000, 250000: 400000,
		150000: 200000, 120000: 200000, 50000: 100000,
		30000: 50000, 15000: 25000, 10000: 12500,
		5000: 6250, 2500: 3125, 1000: 1562.5, 600: 1562.5,
	}[bandwidth]
}
