package mocks

import (
	"fmt"
	"log"
	"math/rand"
	"time"
	"virtual-device-ui/internal/protocol"
)

type JammingDataGenerator struct {
	deviceID    int
	moduleCount int
	modules     []protocol.ModuleInfo
	addressList []uint16
	lastUpdate  time.Time
}

func NewJammingDataGenerator(deviceID int, moduleCount int) *JammingDataGenerator {
	log.Printf("[打击] 创建干扰打击数据生成器: 设备ID=%d, 模块数=%d", deviceID, moduleCount)

	generator := &JammingDataGenerator{
		deviceID:    deviceID,
		moduleCount: moduleCount,
		modules:     make([]protocol.ModuleInfo, moduleCount),
		addressList: []uint16{1, 2, 3}, // 默认地址列表
		lastUpdate:  time.Now(),
	}

	// 初始化模块数据
	generator.initModules()
	return generator
}

func (g *JammingDataGenerator) initModules() {
	for i := 0; i < g.moduleCount; i++ {
		g.modules[i] = protocol.ModuleInfo{
			FreqStart:     uint16(1800 + rand.Intn(400)), // 1800-2200MHz
			FreqEnd:       uint16(2200 + rand.Intn(400)), // 2200-2600MHz
			PowerDisp:     uint8(20 + rand.Intn(80)),     // 20-100功率显示
			TempDisp:      uint8(25 + rand.Intn(50)),     // 25-75温度显示
			SwitchSetting: uint8(rand.Intn(2)),           // 0或1
			PowerSetting:  uint8(0x21),                   // 固定功率设置
			Alarm:         0,                             // 无告警
			Reserved:      0,
		}
		log.Printf("[打击] 初始化模块%d: 频率=%d-%dMHz, 开关=%d",
			i+1, g.modules[i].FreqStart, g.modules[i].FreqEnd, g.modules[i].SwitchSetting)
	}
}

// 生成模块信息数据包
func (g *JammingDataGenerator) GenerateModuleInfoPacket() *protocol.JammingPacket {
	log.Printf("[打击] 设备ID=%d 生成模块信息数据包", g.deviceID)

	// 只有当距离上次更新超过一定时间时才更新参数（降低更新频率）
	if time.Since(g.lastUpdate) > 500*time.Millisecond {
		// 每次调用时随机更新一些参数
		for i := range g.modules {
			// 随机更新温度
			g.modules[i].TempDisp = uint8(25 + rand.Intn(50))
			// 随机更新功率显示
			g.modules[i].PowerDisp = uint8(20 + rand.Intn(80))
			// 偶尔产生告警
			if rand.Float32() < 0.1 { // 10%概率产生告警
				g.modules[i].Alarm = uint8(1 << rand.Intn(4)) // 随机告警类型
			} else {
				g.modules[i].Alarm = 0
			}
		}
		log.Printf("[打击] 设备ID=%d 更新模块数据完成", g.deviceID)
		g.lastUpdate = time.Now()
	}

	packet := protocol.CreateModuleInfoResponse(g.deviceID, g.modules)
	log.Printf("[打击] 设备ID=%d 生成的数据包: %s", g.deviceID, packet.String())
	return packet
}

// 处理设置功率命令
func (g *JammingDataGenerator) HandleSetPowerCommand(packet *protocol.JammingPacket) *protocol.JammingPacket {
	log.Printf("[打击] 设备ID=%d 处理设置功率命令，数据长度: %d", g.deviceID, len(packet.Data))

	// 打印接收到的原始数据（完整）
	recvBytes := packet.ToBytes()
	log.Printf("[打击-原始数据] 接收字节(完整): %02X", recvBytes)
	log.Printf("[打击-原始数据] 接收解析: Header=0x%02X ID=%d Len=%d Cmd=0x%02X Data=%02X Tail=0x%02X",
		packet.Header, packet.ID, packet.Length, packet.Cmd, packet.Data, packet.Tail)

	// 解析设置数据 - 即使数据格式不正确也继续处理
	if len(packet.Data)%2 != 0 {
		log.Printf("[打击] 设备ID=%d 数据长度不正确，但仍返回成功", g.deviceID)
		// 数据长度不正确，但返回成功
		return protocol.CreateSetPowerResponse(g.deviceID, 0)
	}

	settingCount := len(packet.Data) / 2
	if settingCount > g.moduleCount {
		settingCount = g.moduleCount
	}

	log.Printf("[打击] 设备ID=%d 设置 %d 个模块的功率", g.deviceID, settingCount)

	// 更新模块设置
	for i := 0; i < settingCount; i++ {
		switchSetting := packet.Data[i*2]
		powerSetting := packet.Data[i*2+1]

		log.Printf("[打击] 设备ID=%d 模块%d: 开关=%d, 功率=0x%02X", g.deviceID, i+1, switchSetting, powerSetting)

		g.modules[i].SwitchSetting = switchSetting
		g.modules[i].PowerSetting = powerSetting
	}

	// 返回成功
	response := protocol.CreateSetPowerResponse(g.deviceID, 0)
	log.Printf("[打击] 设备ID=%d 设置功率响应: 成功", g.deviceID)

	// 打印响应的原始数据（完整）
	respBytes := response.ToBytes()
	log.Printf("[打击-原始数据] 响应字节(完整): %02X", respBytes)
	log.Printf("[打击-原始数据] 响应解析: Header=0x%02X ID=%d Len=%d Cmd=0x%02X Data=%02X Tail=0x%02X",
		response.Header, response.ID, response.Length, response.Cmd, response.Data, response.Tail)

	return response
}

// 处理地址列表命令
func (g *JammingDataGenerator) HandleAddressListCommand(packet *protocol.JammingPacket) *protocol.JammingPacket {
	log.Printf("[打击] 设备ID=%d 处理地址列表命令: 0x%02X", g.deviceID, packet.Cmd)

	// 打印接收到的原始数据（完整）
	recvBytes := packet.ToBytes()
	log.Printf("[打击-原始数据] 接收字节(完整): %02X", recvBytes)
	log.Printf("[打击-原始数据] 接收解析: Header=0x%02X ID=%d Len=%d Cmd=0x%02X Data=%02X Tail=0x%02X",
		packet.Header, packet.ID, packet.Length, packet.Cmd, packet.Data, packet.Tail)

	switch packet.Cmd {
	case protocol.CmdDownloadAddr:
		// 下载地址列表 - 即使数据格式不正确也返回成功
		if len(packet.Data)%2 == 0 && len(packet.Data) > 0 {
			g.addressList = make([]uint16, len(packet.Data)/2)
			for i := 0; i < len(g.addressList); i++ {
				g.addressList[i] = uint16(packet.Data[i*2])<<8 | uint16(packet.Data[i*2+1])
			}
			log.Printf("[打击] 设备ID=%d 下载地址列表成功，地址数: %d", g.deviceID, len(g.addressList))
		} else {
			log.Printf("[打击] 设备ID=%d 下载地址列表数据异常，但仍返回成功", g.deviceID)
		}
		response := protocol.CreateSetPowerResponse(g.deviceID, 0) // 始终返回成功
		respBytes := response.ToBytes()
		log.Printf("[打击-原始数据] 响应字节(完整): %02X", respBytes)
		log.Printf("[打击-原始数据] 响应解析: Header=0x%02X ID=%d Len=%d Cmd=0x%02X Data=%02X Tail=0x%02X",
			response.Header, response.ID, response.Length, response.Cmd, response.Data, response.Tail)
		return response

	case protocol.CmdQueryAddr:
		// 查询地址列表
		log.Printf("[打击] 设备ID=%d 查询地址列表，地址数: %d", g.deviceID, len(g.addressList))
		response := protocol.CreateAddrListResponse(g.deviceID, protocol.CmdQueryAddr, g.addressList)
		respBytes := response.ToBytes()
		log.Printf("[打击-原始数据] 响应字节(完整): %02X", respBytes)
		log.Printf("[打击-原始数据] 响应解析: Header=0x%02X ID=%d Len=%d Cmd=0x%02X Data=%02X Tail=0x%02X",
			response.Header, response.ID, response.Length, response.Cmd, response.Data, response.Tail)
		return response
	}

	// 未知命令也返回成功
	log.Printf("[打击] 设备ID=%d 未知地址列表命令，返回成功", g.deviceID)
	response := protocol.CreateSetPowerResponse(g.deviceID, 0)
	respBytes := response.ToBytes()
	log.Printf("[打击-原始数据] 响应字节(完整): %02X", respBytes)
	log.Printf("[打击-原始数据] 响应解析: Header=0x%02X ID=%d Len=%d Cmd=0x%02X Data=%02X Tail=0x%02X",
		response.Header, response.ID, response.Length, response.Cmd, response.Data, response.Tail)
	return response
}

// 生成字符串格式的数据（用于日志）
func (g *JammingDataGenerator) GenerateJammingData() fmt.Stringer {
	packet := g.GenerateModuleInfoPacket()
	return &JammingDataString{
		DeviceID: g.deviceID,
		Packet:   packet,
		Modules:  g.modules,
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
