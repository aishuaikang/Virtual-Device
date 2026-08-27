package protocol

import (
	"bytes"
	"errors"
	"fmt"
)

const (
	DirectedFrameStart byte = 0x7E
	DirectedFrameEnd   byte = 0x7F

	DirectedFuncBroadcast byte = 0x15

	DirectedCmdPTZStop                byte = 0x00
	DirectedCmdPTZRight               byte = 0x02
	DirectedCmdPTZSetPreset0          byte = 0x03
	DirectedCmdPTZLeft                byte = 0x04
	DirectedCmdPTZUp                  byte = 0x08
	DirectedCmdHeartbeat              byte = 0x0A
	DirectedCmdPTZDown                byte = 0x10
	DirectedCmdAmpStatusQuery         byte = 0x13
	DirectedCmdSignalSource           byte = 0x21
	DirectedCmdWriteFrequency         byte = 0x28
	DirectedCmdNarrowbandAmpEnable    byte = 0x2C
	DirectedCmdPTZHorizontalLocate    byte = 0x4B
	DirectedCmdPTZPitchLocate         byte = 0x4D
	DirectedCmdPTZHorizontalQuery     byte = 0x51
	DirectedCmdPTZPitchQuery          byte = 0x53
	DirectedCmdPTZHorizontalResponse  byte = 0x59
	DirectedCmdPTZPitchResponse       byte = 0x5B
	DirectedCmdPTZSpeedSet            byte = 0x5F
	DirectedCmdPTZSpeedQuery          byte = 0x73
	DirectedResponseSuccess           byte = 0x00
	DirectedOperationPowerRetain      byte = 0x01
	DirectedOperationPowerLoss        byte = 0x02
	DirectedSwitchOn                  byte = 0x01
	DirectedAmpFaultOverTemperature   byte = 1 << 0
	DirectedAmpFaultStandingWave      byte = 1 << 1
	DirectedAmpFaultCommunicationLost byte = 1 << 2

	directedEscapeByte    byte = 0x5E
	directedEscapedStart  byte = 0x7D
	directedEscapedEscape byte = 0x5D
)

var (
	ErrDirectedFrameTooShort = errors.New("directed strike frame is too short")
	ErrDirectedFrameHeader   = errors.New("invalid directed strike frame header")
	ErrDirectedFrameTail     = errors.New("invalid directed strike frame tail")
	ErrDirectedFrameLength   = errors.New("invalid directed strike frame length")
	ErrDirectedFrameChecksum = errors.New("invalid directed strike frame checksum")
)

// DirectedStrikeFrame is the decoded 0x7E/0x7F directed-strike wire frame.
type DirectedStrikeFrame struct {
	Function   byte
	DeviceAddr byte
	Command    byte
	Response   byte
	Data       []byte
}

// IsDirectedStrikeFrameComplete reports whether a candidate ending in 0x7F
// contains the number of unescaped bytes declared by its length field.
// Payload and checksum bytes may also be 0x7F, so the marker alone is not
// sufficient to identify the end of a frame on a TCP stream.
func IsDirectedStrikeFrameComplete(raw []byte) bool {
	if len(raw) < 2 || raw[0] != DirectedFrameStart || raw[len(raw)-1] != DirectedFrameEnd {
		return false
	}

	middle := unescapeDirectedStrike(raw[1 : len(raw)-1])
	if len(middle) < 5 {
		return false
	}

	expectedLength := 6 + int(middle[4])
	return len(middle) >= expectedLength
}

// ParseDirectedStrikeFrame validates escaping, payload length, and checksum.
func ParseDirectedStrikeFrame(raw []byte) (DirectedStrikeFrame, error) {
	if len(raw) < 8 {
		return DirectedStrikeFrame{}, ErrDirectedFrameTooShort
	}
	if raw[0] != DirectedFrameStart {
		return DirectedStrikeFrame{}, ErrDirectedFrameHeader
	}
	if raw[len(raw)-1] != DirectedFrameEnd {
		return DirectedStrikeFrame{}, ErrDirectedFrameTail
	}

	middle := unescapeDirectedStrike(raw[1 : len(raw)-1])
	if len(middle) < 6 {
		return DirectedStrikeFrame{}, ErrDirectedFrameTooShort
	}

	body := middle[:len(middle)-1]
	checksum := middle[len(middle)-1]
	if DirectedStrikeChecksum(body) != checksum {
		return DirectedStrikeFrame{}, ErrDirectedFrameChecksum
	}

	dataLength := int(body[4])
	if len(body) != 5+dataLength {
		return DirectedStrikeFrame{}, fmt.Errorf("%w: declared=%d actual=%d", ErrDirectedFrameLength, dataLength, len(body)-5)
	}

	data := make([]byte, dataLength)
	copy(data, body[5:])
	return DirectedStrikeFrame{
		Function:   body[0],
		DeviceAddr: body[1],
		Command:    body[2],
		Response:   body[3],
		Data:       data,
	}, nil
}

// BuildDirectedStrikeFrame builds a wire frame and is shared by responses and tests.
func BuildDirectedStrikeFrame(function, deviceAddr, command, response byte, data []byte) []byte {
	if len(data) > 0xFF {
		return nil
	}

	body := make([]byte, 0, 6+len(data))
	body = append(body, function, deviceAddr, command, response, byte(len(data)))
	body = append(body, data...)
	body = append(body, DirectedStrikeChecksum(body))

	escaped := escapeDirectedStrike(body)
	raw := make([]byte, 0, len(escaped)+2)
	raw = append(raw, DirectedFrameStart)
	raw = append(raw, escaped...)
	raw = append(raw, DirectedFrameEnd)
	return raw
}

// BuildDirectedStrikeResponse builds a normal broadcast response frame.
func BuildDirectedStrikeResponse(deviceAddr, command, response byte, data []byte) []byte {
	return BuildDirectedStrikeFrame(DirectedFuncBroadcast, deviceAddr, command, response, data)
}

// DirectedStrikeChecksum returns the one-byte two's-complement checksum.
func DirectedStrikeChecksum(data []byte) byte {
	var sum byte
	for _, value := range data {
		sum += value
	}
	return -sum
}

func escapeDirectedStrike(data []byte) []byte {
	var result bytes.Buffer
	result.Grow(len(data))
	for _, value := range data {
		switch value {
		case DirectedFrameStart:
			result.WriteByte(directedEscapeByte)
			result.WriteByte(directedEscapedStart)
		case directedEscapeByte:
			result.WriteByte(directedEscapeByte)
			result.WriteByte(directedEscapedEscape)
		default:
			result.WriteByte(value)
		}
	}
	return result.Bytes()
}

func unescapeDirectedStrike(data []byte) []byte {
	var result bytes.Buffer
	result.Grow(len(data))
	for index := 0; index < len(data); index++ {
		if data[index] != directedEscapeByte || index+1 >= len(data) {
			result.WriteByte(data[index])
			continue
		}

		switch data[index+1] {
		case directedEscapedStart:
			result.WriteByte(DirectedFrameStart)
			index++
		case directedEscapedEscape:
			result.WriteByte(directedEscapeByte)
			index++
		default:
			result.WriteByte(data[index])
		}
	}
	return result.Bytes()
}
