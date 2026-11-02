package mocks

import (
	"bytes"
	"sync"
	"testing"

	"virtual-device-ui/internal/protocol"
)

func jammingCommand(cmd byte, data []byte) *protocol.JammingPacket {
	return &protocol.JammingPacket{
		Header: protocol.ProtocolHeader, ID: 1, Length: uint8(6 + len(data)),
		Cmd: cmd, Data: data, Tail: protocol.ProtocolTail,
	}
}

func TestJammingInitialStatus(t *testing.T) {
	t.Parallel()
	g := NewJammingDataGenerator(1, 10)
	packet := g.GenerateModuleInfoPacket()
	if packet.Length != 116 || len(packet.Data) != 110 {
		t.Fatalf("expected ten channels: %+v", packet)
	}
	for i := range 10 {
		data := packet.Data[i*11 : (i+1)*11]
		if data[4] != 0 || data[6] != 0 || data[8] != 0x08 {
			t.Errorf("channel %d must start off, with zero power and normal alarm: %x", i+1, data)
		}
	}
	if next := g.GenerateModuleInfoPacket(); !bytes.Equal(packet.Data, next.Data) {
		t.Fatal("status changed without a command")
	}
}

func TestJammingSetPowerValidation(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"odd length", []byte{1}},
		{"invalid switch after valid channel", []byte{1, 0x21, 2, 0x21}},
		{"too many channels", bytes.Repeat([]byte{1, 0x21}, 11)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewJammingDataGenerator(1, 10)
			before := g.GenerateModuleInfoPacket()
			response := g.HandleSetPowerCommand(jammingCommand(protocol.CmdSetPower, tt.data))
			if response == nil || len(response.Data) != 1 || response.Data[0] == 0 {
				t.Fatalf("invalid settings must fail: %+v", response)
			}
			if after := g.GenerateModuleInfoPacket(); !bytes.Equal(before.Data, after.Data) {
				t.Fatal("invalid settings partially changed channels")
			}
		})
	}
}

func TestJammingSetPowerStatus(t *testing.T) {
	t.Parallel()
	g := NewJammingDataGenerator(1, 10)
	for _, enabled := range []byte{1, 0} {
		data := bytes.Repeat([]byte{enabled, 0x30}, 10)
		ack := g.HandleSetPowerCommand(jammingCommand(protocol.CmdSetPower, data))
		if ack == nil || ack.Data[0] != 0 {
			t.Fatalf("settings rejected: %+v", ack)
		}
		status := g.GenerateModuleInfoPacket()
		for i := range 10 {
			channel := status.Data[i*11 : (i+1)*11]
			if channel[6] != enabled || channel[7] != 0x30 || channel[4] != enabled*0x30 || channel[8] != 8 {
				t.Errorf("channel %d does not reflect command: %x", i+1, channel)
			}
		}
	}
}

func TestJammingAddressCommands(t *testing.T) {
	t.Parallel()
	g := NewJammingDataGenerator(1, 10)
	addresses := []byte{1, 0, 0x34, 0x12}
	ack := g.HandleAddressListCommand(jammingCommand(protocol.CmdDownloadAddr, addresses))
	if ack == nil || ack.Cmd != protocol.CmdQueryModules || len(ack.Data) != 1 || ack.Data[0] != 0 {
		t.Fatalf("address download must use 0x05 result ACK: %+v", ack)
	}
	query := jammingCommand(protocol.CmdQueryAddr, nil)
	if got := g.HandleAddressListCommand(query); !bytes.Equal(got.Data, addresses) {
		t.Fatalf("address query = %x, want %x", got.Data, addresses)
	}
	for _, invalid := range [][]byte{nil, {1}} {
		ack = g.HandleAddressListCommand(jammingCommand(protocol.CmdDownloadAddr, invalid))
		if ack == nil || ack.Data[0] == 0 {
			t.Fatalf("invalid address download accepted: %+v", ack)
		}
	}
	if got := g.HandleAddressListCommand(query); !bytes.Equal(got.Data, addresses) {
		t.Fatal("invalid download replaced saved addresses")
	}
}

func TestJammingConcurrentCommandsAndSnapshots(t *testing.T) {
	t.Parallel()
	g := NewJammingDataGenerator(1, 10)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 100 {
				g.HandleSetPowerCommand(jammingCommand(4, bytes.Repeat([]byte{1, 0x30}, 10)))
				g.HandleAddressListCommand(jammingCommand(0x10, []byte{0x34, 0x12}))
				g.GenerateModuleInfoPacket()
				snapshot := g.GenerateJammingData().(*JammingDataString)
				snapshot.Modules[0].SwitchSetting = 2
				g.HandleSetPowerCommand(jammingCommand(4, bytes.Repeat([]byte{0, 0x30}, 10)))
			}
		})
	}
	wg.Wait()
	if status := g.GenerateModuleInfoPacket(); status.Data[6] > 1 {
		t.Fatal("editing a snapshot mutated the device")
	}
}
