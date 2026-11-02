package mocks

import (
	"encoding/binary"
	"fmt"
	"sync"

	"virtual-device-ui/internal/protocol"
)

type JammingDataGenerator struct {
	mu           sync.RWMutex
	deviceID     int
	modules      []protocol.ModuleInfo
	addressList  []uint16
	stuckChannel int // 1-based; zero disables the fault without changing channel state.
}

func NewJammingDataGenerator(deviceID int, moduleCount int) *JammingDataGenerator {
	g := &JammingDataGenerator{
		deviceID:    deviceID,
		modules:     make([]protocol.ModuleInfo, moduleCount),
		addressList: []uint16{1, 2, 3},
	}
	// Stable sample bands follow the backend's channel order; they are not measured values.
	bands := [][2]uint16{
		{430, 440}, {900, 930}, {1100, 1300}, {1300, 1450}, {1450, 1600},
		{1700, 1900}, {3200, 3400}, {5150, 5350}, {2400, 2500}, {5725, 5875},
	}
	for i := range g.modules {
		band := bands[i%len(bands)]
		g.modules[i] = protocol.ModuleInfo{
			FreqStart: band[0], FreqEnd: band[1], TempDisp: 30,
			PowerSetting: 0x30, Alarm: 0x08,
		}
	}
	return g
}

func (g *JammingDataGenerator) moduleSnapshot() []protocol.ModuleInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return append([]protocol.ModuleInfo(nil), g.modules...)
}

func (g *JammingDataGenerator) GenerateModuleInfoPacket() *protocol.JammingPacket {
	return protocol.CreateModuleInfoResponse(g.deviceID, g.moduleSnapshot())
}

func (g *JammingDataGenerator) SetStuckChannel(channel int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stuckChannel = channel
	if channel > 0 && channel <= len(g.modules) {
		module := &g.modules[channel-1]
		module.SwitchSetting = 1
		if module.PowerSetting == 0 {
			module.PowerSetting = 0x30
		}
		module.PowerDisp = module.PowerSetting
	}
}

// Validate the entire command before changing any channel.
func (g *JammingDataGenerator) HandleSetPowerCommand(packet *protocol.JammingPacket) *protocol.JammingPacket {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(packet.Data) == 0 || len(packet.Data)%2 != 0 || len(packet.Data)/2 > len(g.modules) {
		return protocol.CreateSetPowerResponse(g.deviceID, 1)
	}
	for i := 0; i < len(packet.Data); i += 2 {
		if packet.Data[i] > 1 {
			return protocol.CreateSetPowerResponse(g.deviceID, 1)
		}
	}
	for i := 0; i < len(packet.Data)/2; i++ {
		module := &g.modules[i]
		if i+1 == g.stuckChannel && packet.Data[i*2] == 0 {
			continue
		}
		module.SwitchSetting = packet.Data[i*2]
		module.PowerSetting = packet.Data[i*2+1]
		module.PowerDisp = module.SwitchSetting * module.PowerSetting
	}
	return protocol.CreateSetPowerResponse(g.deviceID, 0)
}

func (g *JammingDataGenerator) HandleAddressListCommand(packet *protocol.JammingPacket) *protocol.JammingPacket {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch packet.Cmd {
	case protocol.CmdDownloadAddr:
		// The protocol uses a 0x05 result ACK for address downloads.
		response := protocol.CreateSetPowerResponse(g.deviceID, 1)
		response.Cmd = protocol.CmdQueryModules
		if len(packet.Data) == 0 || len(packet.Data)%2 != 0 {
			return response
		}
		g.addressList = make([]uint16, len(packet.Data)/2)
		for i := range g.addressList {
			g.addressList[i] = binary.LittleEndian.Uint16(packet.Data[i*2:])
		}
		response.Data[0] = 0
		return response
	case protocol.CmdQueryAddr:
		if len(packet.Data) == 0 {
			return protocol.CreateAddrListResponse(g.deviceID, protocol.CmdQueryAddr, g.addressList)
		}
	}
	return nil
}

func (g *JammingDataGenerator) GenerateJammingData() fmt.Stringer {
	modules := g.moduleSnapshot()
	return &JammingDataString{
		DeviceID: g.deviceID,
		Packet:   protocol.CreateModuleInfoResponse(g.deviceID, modules),
		Modules:  modules,
	}
}

type JammingDataString struct {
	DeviceID int
	Packet   *protocol.JammingPacket
	Modules  []protocol.ModuleInfo
}

func (j *JammingDataString) String() string {
	return fmt.Sprintf("干扰打击设备[ID:%d] 模块数:%d 数据包:%s",
		j.DeviceID, len(j.Modules), j.Packet.String())
}
