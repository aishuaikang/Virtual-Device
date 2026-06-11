package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// 协议常量
const (
	ProtocolHeader = 0xAA
	ProtocolTail   = 0x55

	// 命令类型
	CmdSetPower     = 0x04 // 设置N路开关功率
	CmdQueryModules = 0x05 // 查询N路模块信息
	CmdDownloadAddr = 0x10 // 下载地址列表
	CmdQueryAddr    = 0x11 // 查询地址列表
)

// 模块设置结构
type ModuleSetting struct {
	SwitchSetting uint8 // 开关设置 1:开 0:关
	PowerSetting  uint8 // 功率设置
}

// 模块信息结构 (新协议)
type ModuleInfo struct {
	FreqStart     uint16 // 频率起始
	FreqEnd       uint16 // 频率结束
	PowerDisp     uint8  // 功率显示
	TempDisp      uint8  // 温度显示
	SwitchSetting uint8  // 当前开关设置 1:开 0:关
	PowerSetting  uint8  // 当前功率设置
	Alarm         uint8  // 告警状态
	Reserved      uint16 // 保留字节
}

// 协议包结构
type JammingPacket struct {
	Header uint8
	ID     uint16
	Length uint8
	Cmd    uint8
	Data   []byte
	Tail   uint8
}

// 序列化为字节数组（优化版本：减少内存分配和函数调用）
func (p *JammingPacket) ToBytes() []byte {
	// 预分配准确大小：Header(1) + ID(2) + Length(1) + Cmd(1) + Data(n) + Tail(1)
	totalLen := 6 + len(p.Data)
	buf := make([]byte, totalLen)

	buf[0] = p.Header
	buf[1] = byte(p.ID >> 8)   // ID高字节
	buf[2] = byte(p.ID & 0xFF) // ID低字节
	buf[3] = p.Length
	buf[4] = p.Cmd

	if len(p.Data) > 0 {
		copy(buf[5:], p.Data)
	}

	buf[totalLen-1] = p.Tail

	return buf
}

// 从字节数组解析
func ParseJammingPacket(data []byte) (*JammingPacket, error) {
	// 固定头部长度: Header(1) + ID(2) + Length(1) + Cmd(1) + Tail(1) = 6
	if len(data) < 6 {
		return nil, fmt.Errorf("数据长度不足")
	}

	packet := &JammingPacket{}

	// 逐字段解析，避免因平台相关类型大小导致偏移错误
	packet.Header = data[0]
	if packet.Header != ProtocolHeader {
		return nil, fmt.Errorf("无效的协议头: 0x%02X", packet.Header)
	}

	packet.ID = binary.BigEndian.Uint16(data[1:3])
	packet.Length = data[3]
	packet.Cmd = data[4]

	// 期望的总长度校验
	expectedTotal := int(packet.Length)
	if expectedTotal < 6 {
		return nil, fmt.Errorf("长度字段非法: %d", packet.Length)
	}
	if len(data) != expectedTotal {
		return nil, fmt.Errorf("实际长度(%d)与长度字段(%d)不一致", len(data), expectedTotal)
	}

	dataLen := expectedTotal - 6
	if dataLen > 0 {
		packet.Data = data[5 : 5+dataLen]
	} else {
		packet.Data = nil
	}
	packet.Tail = data[5+dataLen]

	if packet.Tail != ProtocolTail {
		return nil, fmt.Errorf("无效的协议尾: 0x%02X", packet.Tail)
	}

	return packet, nil
}

// 创建查询模块信息响应包
func CreateModuleInfoResponse(deviceID int, modules []ModuleInfo) *JammingPacket {
	// 预分配缓冲区大小: 每个模块11字节 (2+2+1+1+1+1+1+2)
	buf := bytes.NewBuffer(make([]byte, 0, len(modules)*11))

	for _, module := range modules {
		binary.Write(buf, binary.BigEndian, module.FreqStart)
		binary.Write(buf, binary.BigEndian, module.FreqEnd)
		binary.Write(buf, binary.BigEndian, module.PowerDisp)
		binary.Write(buf, binary.BigEndian, module.TempDisp)
		binary.Write(buf, binary.BigEndian, module.SwitchSetting)
		binary.Write(buf, binary.BigEndian, module.PowerSetting)
		binary.Write(buf, binary.BigEndian, module.Alarm)
		binary.Write(buf, binary.BigEndian, module.Reserved)
	}

	data := buf.Bytes()
	length := uint8(6 + len(data)) // 固定头部长度 + 数据长度

	return &JammingPacket{
		Header: ProtocolHeader,
		ID:     uint16(deviceID),
		Length: length,
		Cmd:    CmdQueryModules,
		Data:   data,
		Tail:   ProtocolTail,
	}
}

// 创建设置功率响应包
func CreateSetPowerResponse(deviceID int, result uint8) *JammingPacket {
	return &JammingPacket{
		Header: ProtocolHeader,
		ID:     uint16(deviceID),
		Length: 7, // 6字节固定 + 1字节结果
		Cmd:    CmdSetPower,
		Data:   []byte{result},
		Tail:   ProtocolTail,
	}
}

// 创建地址列表响应包
func CreateAddrListResponse(deviceID int, cmd uint8, addresses []uint16) *JammingPacket {
	buf := new(bytes.Buffer)

	for _, addr := range addresses {
		binary.Write(buf, binary.BigEndian, addr)
	}

	data := buf.Bytes()
	length := uint8(6 + len(data))

	return &JammingPacket{
		Header: ProtocolHeader,
		ID:     uint16(deviceID),
		Length: length,
		Cmd:    cmd,
		Data:   data,
		Tail:   ProtocolTail,
	}
}

func (p *JammingPacket) String() string {
	dataHex := ""
	if len(p.Data) > 0 {
		// 完整显示所有数据，不截断
		dataHex = fmt.Sprintf("%02X (共%d字节)", p.Data, len(p.Data))
	} else {
		dataHex = "无"
	}
	return fmt.Sprintf("Header:0x%02X ID:%d Len:%d Cmd:0x%02X Data:%s Tail:0x%02X",
		p.Header, p.ID, p.Length, p.Cmd, dataHex, p.Tail)
}
