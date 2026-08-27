package modules

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"virtual-device-ui/internal/protocol"
)

const directedStrikeMaxFrameSize = 4096

var directedStrikeSourceAddresses = []byte{0x02, 0x03, 0x04, 0x05}
var directedStrikeNarrowbandAddresses = []byte{0x01, 0x02, 0x03, 0x04, 0x05}

// DirectedStrikeSourceStatus is one wideband signal source's live state.
type DirectedStrikeSourceStatus struct {
	Address string `json:"address"`
	Enabled bool   `json:"enabled"`
}

// DirectedStrikeFrequencyStatus is the last accepted frequency range for a source.
type DirectedStrikeFrequencyStatus struct {
	Address       string `json:"address"`
	StartFreqMHz  uint16 `json:"startFreqMHz"`
	EndFreqMHz    uint16 `json:"endFreqMHz"`
	LastUpdatedAt string `json:"lastUpdatedAt,omitempty"`
}

// DirectedStrikeAmpStatus is one narrowband amplifier's live state.
type DirectedStrikeAmpStatus struct {
	Address string `json:"address"`
	Enabled bool   `json:"enabled"`
}

// DirectedStrikePTZStatus is the simulated PTZ position and motion state.
type DirectedStrikePTZStatus struct {
	HorizontalAngle       float64 `json:"horizontalAngle"`
	PitchAngle            float64 `json:"pitchAngle"`
	Action                string  `json:"action,omitempty"`
	HorizontalSpeed       uint16  `json:"horizontalSpeed"`
	VerticalSpeed         uint16  `json:"verticalSpeed"`
	LocateHorizontalSpeed byte    `json:"locateHorizontalSpeed"`
	LocateVerticalSpeed   byte    `json:"locateVerticalSpeed"`
	PresetSaved           bool    `json:"presetSaved"`
}

// DirectedStrikeSnapshot is the operator-facing status of the simulator.
type DirectedStrikeSnapshot struct {
	Listening         bool                            `json:"listening"`
	ListenAddress     string                          `json:"listenAddress"`
	ActiveConnections int                             `json:"activeConnections"`
	ClientAddresses   []string                        `json:"clientAddresses"`
	HeartbeatCount    uint64                          `json:"heartbeatCount"`
	ReceivedFrames    uint64                          `json:"receivedFrames"`
	SentFrames        uint64                          `json:"sentFrames"`
	LastCommand       string                          `json:"lastCommand,omitempty"`
	LastCommandDetail string                          `json:"lastCommandDetail,omitempty"`
	LastActivityAt    string                          `json:"lastActivityAt,omitempty"`
	LastError         string                          `json:"lastError,omitempty"`
	SignalSources     []DirectedStrikeSourceStatus    `json:"signalSources"`
	WidebandConfigs   []DirectedStrikeFrequencyStatus `json:"widebandConfigs"`
	NarrowbandAmps    []DirectedStrikeAmpStatus       `json:"narrowbandAmps"`
	PTZ               DirectedStrikePTZStatus         `json:"ptz"`
}

// DirectedStrikeModule simulates the TCP server exposed by a directed-strike device.
type DirectedStrikeModule interface {
	Start()
	Stop()
	Snapshot() DirectedStrikeSnapshot
}

type directedStrikeFrequency struct {
	startFreqMHz uint16
	endFreqMHz   uint16
	updatedAt    time.Time
}

type directedStrikePTZ struct {
	horizontalAngle       float64
	pitchAngle            float64
	action                byte
	horizontalSpeed       uint16
	verticalSpeed         uint16
	locateHorizontalSpeed byte
	locateVerticalSpeed   byte
	horizontalLocate      bool
	horizontalTarget      float64
	pitchLocate           bool
	pitchTarget           float64
	presetSaved           bool
	presetHorizontal      float64
	presetPitch           float64
	motionUpdatedAt       time.Time
}

type directedStrike struct {
	ctx           context.Context
	cancel        context.CancelFunc
	listener      net.Listener
	listenAddress string
	responseDelay time.Duration
	stopOnce      sync.Once
	connectionsWG sync.WaitGroup

	mu                sync.Mutex
	listening         bool
	connections       map[net.Conn]string
	heartbeatCount    uint64
	receivedFrames    uint64
	sentFrames        uint64
	lastCommand       string
	lastCommandDetail string
	lastActivityAt    time.Time
	lastError         string
	signalSources     map[byte]bool
	widebandConfigs   map[byte]directedStrikeFrequency
	narrowbandAmps    map[byte]bool
	ptz               directedStrikePTZ
	random            *rand.Rand
}

// NewDirectedStrikeModule binds the listener before engine startup so address errors are immediate.
func NewDirectedStrikeModule(listenAddress string, responseDelay time.Duration) (DirectedStrikeModule, error) {
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return nil, fmt.Errorf("listen for directed strike simulator on %s: %w", listenAddress, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	displayAddress := listenAddress
	if _, requestedPort, splitErr := net.SplitHostPort(listenAddress); splitErr == nil && requestedPort == "0" {
		displayAddress = listener.Addr().String()
	}
	module := &directedStrike{
		ctx:             ctx,
		cancel:          cancel,
		listener:        listener,
		listenAddress:   displayAddress,
		responseDelay:   responseDelay,
		connections:     make(map[net.Conn]string),
		signalSources:   make(map[byte]bool, len(directedStrikeSourceAddresses)),
		widebandConfigs: make(map[byte]directedStrikeFrequency),
		narrowbandAmps:  make(map[byte]bool, len(directedStrikeNarrowbandAddresses)),
		random:          rand.New(rand.NewSource(time.Now().UnixNano())),
		ptz: directedStrikePTZ{
			locateHorizontalSpeed: 32,
			locateVerticalSpeed:   34,
			motionUpdatedAt:       time.Now(),
		},
	}
	for _, address := range directedStrikeSourceAddresses {
		module.signalSources[address] = false
	}
	for _, address := range directedStrikeNarrowbandAddresses {
		module.narrowbandAmps[address] = false
	}
	return module, nil
}

func (module *directedStrike) Start() {
	module.mu.Lock()
	module.listening = true
	module.mu.Unlock()
	log.Printf("[定向打击] 模拟器开始监听 address=%s delay=%s", module.listenAddress, module.responseDelay)

	for {
		connection, err := module.listener.Accept()
		if err != nil {
			if module.ctx.Err() == nil {
				module.recordError(fmt.Errorf("accept directed strike client: %w", err))
				log.Printf("[定向打击] 接收客户端失败 error=%v", err)
			}
			break
		}

		if !module.registerConnection(connection) {
			continue
		}
		module.connectionsWG.Add(1)
		go func() {
			defer module.connectionsWG.Done()
			module.handleConnection(connection)
		}()
	}

	module.connectionsWG.Wait()
	module.mu.Lock()
	module.listening = false
	module.mu.Unlock()
	log.Printf("[定向打击] 模拟器已停止")
}

func (module *directedStrike) Stop() {
	module.stopOnce.Do(func() {
		module.cancel()
		if err := module.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("[定向打击] 关闭监听失败 error=%v", err)
		}

		module.mu.Lock()
		connections := make([]net.Conn, 0, len(module.connections))
		for connection := range module.connections {
			connections = append(connections, connection)
		}
		module.mu.Unlock()
		for _, connection := range connections {
			if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				log.Printf("[定向打击] 关闭客户端连接失败 remote=%s error=%v", connection.RemoteAddr(), err)
			}
		}
	})
}

func (module *directedStrike) registerConnection(connection net.Conn) bool {
	remoteAddress := connection.RemoteAddr().String()
	module.mu.Lock()
	if module.ctx.Err() != nil {
		module.mu.Unlock()
		_ = connection.Close()
		return false
	}
	module.connections[connection] = remoteAddress
	module.lastActivityAt = time.Now()
	module.mu.Unlock()
	log.Printf("[定向打击] 客户端已连接 remote=%s", remoteAddress)
	return true
}

func (module *directedStrike) unregisterConnection(connection net.Conn) {
	remoteAddress := connection.RemoteAddr().String()
	module.mu.Lock()
	delete(module.connections, connection)
	module.lastActivityAt = time.Now()
	module.mu.Unlock()
	if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Printf("[定向打击] 关闭客户端连接失败 remote=%s error=%v", remoteAddress, err)
	}
	log.Printf("[定向打击] 客户端已断开 remote=%s", remoteAddress)
}

func (module *directedStrike) handleConnection(connection net.Conn) {
	defer module.unregisterConnection(connection)
	remoteAddress := connection.RemoteAddr().String()
	reader := bufio.NewReader(connection)

	for {
		select {
		case <-module.ctx.Done():
			return
		default:
		}

		raw, err := readDirectedStrikeFrame(reader)
		if err != nil {
			if module.ctx.Err() != nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			module.recordError(fmt.Errorf("read directed strike frame from %s: %w", remoteAddress, err))
			log.Printf("[定向打击] 读取报文失败 remote=%s error=%v", remoteAddress, err)
			return
		}

		log.Printf("[定向打击] RX remote=%s frame=%s", remoteAddress, formatDirectedStrikeHex(raw))
		frame, err := protocol.ParseDirectedStrikeFrame(raw)
		if err != nil {
			module.recordReceivedFrame(err)
			log.Printf("[定向打击] 解析报文失败 remote=%s error=%v", remoteAddress, err)
			continue
		}

		module.recordReceivedFrame(nil)
		responses, err := module.handleFrame(frame)
		if err != nil {
			module.recordError(err)
			log.Printf("[定向打击] 处理命令失败 remote=%s command=0x%02X error=%v", remoteAddress, frame.Command, err)
			continue
		}

		for _, response := range responses {
			if err := module.waitResponseDelay(); err != nil {
				return
			}
			if _, err := connection.Write(response); err != nil {
				module.recordError(fmt.Errorf("write directed strike response to %s: %w", remoteAddress, err))
				log.Printf("[定向打击] 发送回执失败 remote=%s error=%v", remoteAddress, err)
				return
			}
			module.recordSentFrame()
			log.Printf("[定向打击] TX remote=%s frame=%s", remoteAddress, formatDirectedStrikeHex(response))
		}
	}
}

func (module *directedStrike) waitResponseDelay() error {
	if module.responseDelay <= 0 {
		return nil
	}
	timer := time.NewTimer(module.responseDelay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-module.ctx.Done():
		return module.ctx.Err()
	}
}

func readDirectedStrikeFrame(reader *bufio.Reader) ([]byte, error) {
	for {
		value, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if value != protocol.DirectedFrameStart {
			continue
		}

		frame := []byte{value}
		for len(frame) <= directedStrikeMaxFrameSize {
			next, err := reader.ReadByte()
			if err != nil {
				return nil, err
			}
			frame = append(frame, next)
			if next == protocol.DirectedFrameEnd && protocol.IsDirectedStrikeFrameComplete(frame) {
				return frame, nil
			}
		}
		return nil, fmt.Errorf("directed strike frame exceeds %d bytes", directedStrikeMaxFrameSize)
	}
}

func (module *directedStrike) recordReceivedFrame(err error) {
	module.mu.Lock()
	defer module.mu.Unlock()
	module.receivedFrames++
	module.lastActivityAt = time.Now()
	if err != nil {
		module.lastError = err.Error()
	}
}

func (module *directedStrike) recordSentFrame() {
	module.mu.Lock()
	defer module.mu.Unlock()
	module.sentFrames++
	module.lastActivityAt = time.Now()
}

func (module *directedStrike) recordError(err error) {
	module.mu.Lock()
	defer module.mu.Unlock()
	module.lastError = err.Error()
	module.lastActivityAt = time.Now()
}

func (module *directedStrike) handleFrame(frame protocol.DirectedStrikeFrame) ([][]byte, error) {
	module.mu.Lock()
	defer module.mu.Unlock()

	now := time.Now()
	module.advancePTZLocked(now)
	module.lastActivityAt = now
	module.lastCommand = directedStrikeCommandName(frame.Command)
	module.lastCommandDetail = ""
	module.lastError = ""

	var responses [][]byte
	var responseData []byte
	var responseCommand = frame.Command
	shouldRespond := true

	switch frame.Command {
	case protocol.DirectedCmdHeartbeat:
		module.heartbeatCount++
		module.lastCommandDetail = fmt.Sprintf("第 %d 次心跳，设备地址 %s", module.heartbeatCount, formatDirectedStrikeAddress(frame.DeviceAddr))
	case protocol.DirectedCmdAmpStatusQuery:
		responseData = module.ampStatusPayloadLocked(frame.DeviceAddr)
		module.lastCommandDetail = fmt.Sprintf("查询 %s 功放状态", formatDirectedStrikeAddress(frame.DeviceAddr))
	case protocol.DirectedCmdSignalSource:
		data, detail, err := module.handleSignalSourceLocked(frame.Data)
		if err != nil {
			return nil, err
		}
		responseData = data
		module.lastCommandDetail = detail
	case protocol.DirectedCmdWriteFrequency:
		response, detail, err := module.handleFrequencyLocked(frame, now)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
		module.lastCommandDetail = detail
	case protocol.DirectedCmdNarrowbandAmpEnable:
		detail, err := module.handleNarrowbandLocked(frame.Data)
		if err != nil {
			return nil, err
		}
		module.lastCommandDetail = detail
	case protocol.DirectedCmdPTZStop, protocol.DirectedCmdPTZUp, protocol.DirectedCmdPTZDown, protocol.DirectedCmdPTZLeft, protocol.DirectedCmdPTZRight:
		detail, err := module.handlePTZMotionLocked(frame, now)
		if err != nil {
			return nil, err
		}
		module.lastCommandDetail = detail
		shouldRespond = false
	case protocol.DirectedCmdPTZSetPreset0:
		module.ptz.presetSaved = true
		module.ptz.presetHorizontal = module.ptz.horizontalAngle
		module.ptz.presetPitch = module.ptz.pitchAngle
		module.lastCommandDetail = fmt.Sprintf("保存预置位：水平 %.2f° / 俯仰 %.2f°", module.ptz.horizontalAngle, module.ptz.pitchAngle)
		shouldRespond = false
	case protocol.DirectedCmdPTZHorizontalLocate:
		if len(frame.Data) < 2 {
			return nil, errors.New("horizontal PTZ locate payload is too short")
		}
		target := float64(binary.BigEndian.Uint16(frame.Data[:2])) / 100
		module.ptz.action = 0
		module.ptz.horizontalLocate = true
		module.ptz.horizontalTarget = normalizeDirectedStrikeHorizontal(target)
		module.ptz.motionUpdatedAt = now
		module.lastCommandDetail = fmt.Sprintf("水平定位至 %.2f°", target)
		shouldRespond = false
	case protocol.DirectedCmdPTZPitchLocate:
		if len(frame.Data) < 2 {
			return nil, errors.New("pitch PTZ locate payload is too short")
		}
		target := decodeDirectedStrikePitch(binary.BigEndian.Uint16(frame.Data[:2]))
		module.ptz.action = 0
		module.ptz.pitchLocate = true
		module.ptz.pitchTarget = clampDirectedStrikePitch(target)
		module.ptz.motionUpdatedAt = now
		module.lastCommandDetail = fmt.Sprintf("俯仰定位至 %.2f°", target)
		shouldRespond = false
	case protocol.DirectedCmdPTZHorizontalQuery:
		responseCommand = protocol.DirectedCmdPTZHorizontalResponse
		responseData = make([]byte, 2)
		binary.BigEndian.PutUint16(responseData, uint16(math.Round(normalizeDirectedStrikeHorizontal(module.ptz.horizontalAngle)*100)))
		module.lastCommandDetail = fmt.Sprintf("当前水平角 %.2f°", module.ptz.horizontalAngle)
	case protocol.DirectedCmdPTZPitchQuery:
		responseCommand = protocol.DirectedCmdPTZPitchResponse
		responseData = encodeDirectedStrikePitch(module.ptz.pitchAngle)
		module.lastCommandDetail = fmt.Sprintf("当前俯仰角 %.2f°", module.ptz.pitchAngle)
	case protocol.DirectedCmdPTZSpeedSet:
		if len(frame.Data) < 2 {
			return nil, errors.New("PTZ speed payload is too short")
		}
		module.ptz.locateHorizontalSpeed = frame.Data[0]
		module.ptz.locateVerticalSpeed = frame.Data[1]
		module.lastCommandDetail = fmt.Sprintf("定位速度 H%d / V%d", frame.Data[0], frame.Data[1])
		shouldRespond = false
	case protocol.DirectedCmdPTZSpeedQuery:
		responseData = []byte{module.ptz.locateHorizontalSpeed, module.ptz.locateVerticalSpeed}
		module.lastCommandDetail = fmt.Sprintf("当前定位速度 H%d / V%d", responseData[0], responseData[1])
	default:
		return nil, fmt.Errorf("unsupported directed strike command: 0x%02X", frame.Command)
	}

	if !shouldRespond {
		return nil, nil
	}
	if len(responses) == 0 {
		responses = append(responses, protocol.BuildDirectedStrikeResponse(frame.DeviceAddr, responseCommand, protocol.DirectedResponseSuccess, responseData))
	}
	return responses, nil
}

func (module *directedStrike) handleSignalSourceLocked(data []byte) ([]byte, string, error) {
	if len(data) < 3 || (len(data)-1)%2 != 0 {
		return nil, "", errors.New("invalid signal source payload length")
	}
	if data[0] != protocol.DirectedOperationPowerLoss {
		return nil, "", fmt.Errorf("unsupported signal source operation: 0x%02X", data[0])
	}

	response := make([]byte, len(data))
	response[0] = 0x00
	details := make([]string, 0, (len(data)-1)/2)
	for index := 1; index < len(data); index += 2 {
		address := data[index]
		if !isDirectedStrikeSourceAddress(address) {
			return nil, "", fmt.Errorf("unsupported signal source address: 0x%02X", address)
		}
		if data[index+1] != 0x00 && data[index+1] != protocol.DirectedSwitchOn {
			return nil, "", fmt.Errorf("invalid signal source switch value: 0x%02X", data[index+1])
		}
		enabled := data[index+1] == protocol.DirectedSwitchOn
		module.signalSources[address] = enabled
		response[index] = address
		response[index+1] = protocol.DirectedResponseSuccess
		details = append(details, fmt.Sprintf("%s %s", formatDirectedStrikeAddress(address), directedStrikeOnOff(enabled)))
	}
	return response, strings.Join(details, "，"), nil
}

func (module *directedStrike) handleFrequencyLocked(frame protocol.DirectedStrikeFrame, now time.Time) ([]byte, string, error) {
	if len(frame.Data) < 1 {
		return nil, "", errors.New("frequency payload is empty")
	}

	switch frame.Data[0] {
	case protocol.DirectedOperationPowerRetain:
		if len(frame.Data) < 6 || (len(frame.Data)-1)%5 != 0 {
			return nil, "", errors.New("invalid frequency write payload length")
		}
		count := (len(frame.Data) - 1) / 5
		response := make([]byte, 1+count*2)
		response[0] = 0x00
		details := make([]string, 0, count)
		for index := 0; index < count; index++ {
			offset := 1 + index*5
			address := frame.Data[offset]
			if !isDirectedStrikeSourceAddress(address) {
				return nil, "", fmt.Errorf("unsupported frequency source address: 0x%02X", address)
			}
			startFrequency := decodeDirectedStrikeFrequency(address, binary.BigEndian.Uint16(frame.Data[offset+1:offset+3]))
			endFrequency := decodeDirectedStrikeFrequency(address, binary.BigEndian.Uint16(frame.Data[offset+3:offset+5]))
			module.widebandConfigs[address] = directedStrikeFrequency{startFreqMHz: startFrequency, endFreqMHz: endFrequency, updatedAt: now}
			response[1+index*2] = address
			response[2+index*2] = protocol.DirectedResponseSuccess
			details = append(details, fmt.Sprintf("%s %d-%dMHz", formatDirectedStrikeAddress(address), startFrequency, endFrequency))
		}
		return protocol.BuildDirectedStrikeResponse(frame.DeviceAddr, frame.Command, protocol.DirectedResponseSuccess, response), strings.Join(details, "，"), nil
	case 0x00:
		if len(frame.Data) != 6 {
			return nil, "", errors.New("invalid frequency query payload length")
		}
		address := frame.DeviceAddr
		if frame.Data[1] != 0x00 {
			address = frame.Data[1]
		}
		if !isDirectedStrikeSourceAddress(address) {
			return nil, "", fmt.Errorf("unsupported frequency query address: 0x%02X", address)
		}
		frequency := module.widebandConfigs[address]
		responseData := make([]byte, 5)
		binary.BigEndian.PutUint16(responseData[1:3], encodeDirectedStrikeFrequency(address, frequency.startFreqMHz))
		binary.BigEndian.PutUint16(responseData[3:5], encodeDirectedStrikeFrequency(address, frequency.endFreqMHz))
		response := protocol.BuildDirectedStrikeResponse(address, frame.Command, protocol.DirectedResponseSuccess, responseData)
		return response, fmt.Sprintf("查询 %s：%d-%dMHz", formatDirectedStrikeAddress(address), frequency.startFreqMHz, frequency.endFreqMHz), nil
	default:
		return nil, "", fmt.Errorf("unsupported frequency operation: 0x%02X", frame.Data[0])
	}
}

func (module *directedStrike) handleNarrowbandLocked(data []byte) (string, error) {
	if len(data) < 2 || data[0] != protocol.DirectedOperationPowerRetain {
		return "", errors.New("invalid narrowband amplifier payload")
	}
	count := int(data[1])
	if len(data) != 2+count*2 {
		return "", errors.New("invalid narrowband amplifier payload length")
	}

	details := make([]string, 0, count)
	for index := 0; index < count; index++ {
		offset := 2 + index*2
		address := data[offset]
		if !isDirectedStrikeNarrowbandAddress(address) {
			return "", fmt.Errorf("unsupported narrowband amplifier address: 0x%02X", address)
		}
		enabled := data[offset+1] == protocol.DirectedSwitchOn
		module.narrowbandAmps[address] = enabled
		details = append(details, fmt.Sprintf("%s %s", formatDirectedStrikeAddress(address), directedStrikeOnOff(enabled)))
	}
	return strings.Join(details, "，"), nil
}

func (module *directedStrike) handlePTZMotionLocked(frame protocol.DirectedStrikeFrame, now time.Time) (string, error) {
	if len(frame.Data) < 2 {
		return "", errors.New("PTZ control payload is too short")
	}
	module.ptz.motionUpdatedAt = now
	module.ptz.horizontalLocate = false
	module.ptz.pitchLocate = false
	if frame.Command == protocol.DirectedCmdPTZStop {
		module.ptz.action = 0
		module.ptz.horizontalSpeed = 0
		module.ptz.verticalSpeed = 0
		return "停止云台运动", nil
	}
	module.ptz.action = frame.Command
	module.ptz.horizontalSpeed = uint16(frame.Data[0])
	module.ptz.verticalSpeed = uint16(frame.Data[1])
	return fmt.Sprintf("%s，速度 H%d / V%d", directedStrikePTZAction(frame.Command), frame.Data[0], frame.Data[1]), nil
}

func (module *directedStrike) ampStatusPayloadLocked(address byte) []byte {
	enabled := module.signalSources[address]
	temperature := module.randomRangeLocked(25, 45)
	voltage := module.randomRangeLocked(26800, 28200)
	current := module.randomRangeLocked(0, 120)
	power := module.randomRangeLocked(0, 20)
	fault := byte(0)
	if enabled {
		temperature = module.randomRangeLocked(30, 68)
		voltage = module.randomRangeLocked(27200, 28900)
		current = module.randomRangeLocked(5000, 9000)
		power = module.randomRangeLocked(180, 320)
		faultOptions := []byte{0, 0, 0, protocol.DirectedAmpFaultOverTemperature, protocol.DirectedAmpFaultStandingWave, protocol.DirectedAmpFaultCommunicationLost}
		fault = faultOptions[module.random.Intn(len(faultOptions))]
		if fault&protocol.DirectedAmpFaultOverTemperature != 0 {
			temperature = module.randomRangeLocked(70, 95)
		}
	}

	payload := make([]byte, 20)
	payload[0] = byte(module.heartbeatCount)
	if enabled {
		payload[1] = protocol.DirectedSwitchOn
	}
	payload[2] = fault
	payload[3] = byte(temperature)
	binary.BigEndian.PutUint16(payload[4:6], uint16(voltage))
	binary.BigEndian.PutUint16(payload[6:8], uint16(current))
	binary.BigEndian.PutUint16(payload[8:10], uint16(power))
	copy(payload[10:], []byte{0x00, 0x05, 0xDB, 0x06, 0x05, 0x02, 0x00, 0x64, 0x00, 0x00})
	return payload
}

func (module *directedStrike) randomRangeLocked(minimum, maximum int) int {
	if maximum <= minimum {
		return minimum
	}
	return minimum + module.random.Intn(maximum-minimum+1)
}

func (module *directedStrike) advancePTZLocked(now time.Time) {
	if module.ptz.motionUpdatedAt.IsZero() || !now.After(module.ptz.motionUpdatedAt) {
		module.ptz.motionUpdatedAt = now
		return
	}
	deltaSeconds := now.Sub(module.ptz.motionUpdatedAt).Seconds()
	horizontalStep := deltaSeconds * float64(module.ptz.horizontalSpeed) / 4
	verticalStep := deltaSeconds * float64(module.ptz.verticalSpeed) / 4

	switch module.ptz.action {
	case protocol.DirectedCmdPTZLeft:
		module.ptz.horizontalAngle = normalizeDirectedStrikeHorizontal(module.ptz.horizontalAngle - horizontalStep)
	case protocol.DirectedCmdPTZRight:
		module.ptz.horizontalAngle = normalizeDirectedStrikeHorizontal(module.ptz.horizontalAngle + horizontalStep)
	case protocol.DirectedCmdPTZUp:
		module.ptz.pitchAngle = clampDirectedStrikePitch(module.ptz.pitchAngle + verticalStep)
	case protocol.DirectedCmdPTZDown:
		module.ptz.pitchAngle = clampDirectedStrikePitch(module.ptz.pitchAngle - verticalStep)
	}
	if module.ptz.horizontalLocate {
		module.ptz.horizontalAngle, module.ptz.horizontalLocate = moveDirectedStrikeHorizontal(
			module.ptz.horizontalAngle,
			module.ptz.horizontalTarget,
			deltaSeconds*float64(module.ptz.locateHorizontalSpeed)/4,
		)
	}
	if module.ptz.pitchLocate {
		module.ptz.pitchAngle, module.ptz.pitchLocate = moveDirectedStrikePitch(
			module.ptz.pitchAngle,
			module.ptz.pitchTarget,
			deltaSeconds*float64(module.ptz.locateVerticalSpeed)/4,
		)
	}
	module.ptz.motionUpdatedAt = now
}

func (module *directedStrike) Snapshot() DirectedStrikeSnapshot {
	module.mu.Lock()
	defer module.mu.Unlock()
	module.advancePTZLocked(time.Now())

	clients := make([]string, 0, len(module.connections))
	for _, address := range module.connections {
		clients = append(clients, address)
	}
	sort.Strings(clients)

	sources := make([]DirectedStrikeSourceStatus, 0, len(module.signalSources))
	for address, enabled := range module.signalSources {
		sources = append(sources, DirectedStrikeSourceStatus{Address: formatDirectedStrikeAddress(address), Enabled: enabled})
	}
	sort.Slice(sources, func(left, right int) bool { return sources[left].Address < sources[right].Address })

	frequencies := make([]DirectedStrikeFrequencyStatus, 0, len(module.widebandConfigs))
	for address, frequency := range module.widebandConfigs {
		frequencies = append(frequencies, DirectedStrikeFrequencyStatus{
			Address:       formatDirectedStrikeAddress(address),
			StartFreqMHz:  frequency.startFreqMHz,
			EndFreqMHz:    frequency.endFreqMHz,
			LastUpdatedAt: frequency.updatedAt.Format(time.RFC3339Nano),
		})
	}
	sort.Slice(frequencies, func(left, right int) bool { return frequencies[left].Address < frequencies[right].Address })

	amps := make([]DirectedStrikeAmpStatus, 0, len(module.narrowbandAmps))
	for address, enabled := range module.narrowbandAmps {
		amps = append(amps, DirectedStrikeAmpStatus{Address: formatDirectedStrikeAddress(address), Enabled: enabled})
	}
	sort.Slice(amps, func(left, right int) bool { return amps[left].Address < amps[right].Address })

	lastActivity := ""
	if !module.lastActivityAt.IsZero() {
		lastActivity = module.lastActivityAt.Format(time.RFC3339Nano)
	}
	return DirectedStrikeSnapshot{
		Listening:         module.listening,
		ListenAddress:     module.listenAddress,
		ActiveConnections: len(module.connections),
		ClientAddresses:   clients,
		HeartbeatCount:    module.heartbeatCount,
		ReceivedFrames:    module.receivedFrames,
		SentFrames:        module.sentFrames,
		LastCommand:       module.lastCommand,
		LastCommandDetail: module.lastCommandDetail,
		LastActivityAt:    lastActivity,
		LastError:         module.lastError,
		SignalSources:     sources,
		WidebandConfigs:   frequencies,
		NarrowbandAmps:    amps,
		PTZ: DirectedStrikePTZStatus{
			HorizontalAngle:       roundDirectedStrike(module.ptz.horizontalAngle),
			PitchAngle:            roundDirectedStrike(module.ptz.pitchAngle),
			Action:                directedStrikePTZAction(module.ptz.action),
			HorizontalSpeed:       module.ptz.horizontalSpeed,
			VerticalSpeed:         module.ptz.verticalSpeed,
			LocateHorizontalSpeed: module.ptz.locateHorizontalSpeed,
			LocateVerticalSpeed:   module.ptz.locateVerticalSpeed,
			PresetSaved:           module.ptz.presetSaved,
		},
	}
}

func moveDirectedStrikeHorizontal(current, target, step float64) (float64, bool) {
	current = normalizeDirectedStrikeHorizontal(current)
	target = normalizeDirectedStrikeHorizontal(target)
	delta := math.Mod(target-current+540, 360) - 180
	if math.Abs(delta) <= step {
		return target, false
	}
	if delta > 0 {
		return normalizeDirectedStrikeHorizontal(current + step), true
	}
	return normalizeDirectedStrikeHorizontal(current - step), true
}

func moveDirectedStrikePitch(current, target, step float64) (float64, bool) {
	delta := target - current
	if math.Abs(delta) <= step {
		return target, false
	}
	if delta > 0 {
		return clampDirectedStrikePitch(current + step), true
	}
	return clampDirectedStrikePitch(current - step), true
}

func normalizeDirectedStrikeHorizontal(angle float64) float64 {
	normalized := math.Mod(angle, 360)
	if normalized < 0 {
		normalized += 360
	}
	return normalized
}

func clampDirectedStrikePitch(angle float64) float64 {
	return math.Max(-28, math.Min(33, angle))
}

func encodeDirectedStrikePitch(angle float64) []byte {
	angle = clampDirectedStrikePitch(angle)
	var raw uint16
	if angle < 0 {
		raw = uint16(math.Round(-angle * 100))
	} else {
		raw = uint16(math.Round((360 - angle) * 100))
	}
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, raw)
	return data
}

func decodeDirectedStrikePitch(raw uint16) float64 {
	if raw < 18000 {
		return -float64(raw) / 100
	}
	return 360 - float64(raw)/100
}

func decodeDirectedStrikeFrequency(address byte, frequency uint16) uint16 {
	if address == 0x05 && frequency != 0 {
		return frequency + 5000
	}
	return frequency
}

func encodeDirectedStrikeFrequency(address byte, frequency uint16) uint16 {
	if address == 0x05 && frequency >= 5000 {
		return frequency - 5000
	}
	return frequency
}

func isDirectedStrikeSourceAddress(address byte) bool {
	return address >= 0x02 && address <= 0x05
}

func isDirectedStrikeNarrowbandAddress(address byte) bool {
	return address >= 0x01 && address <= 0x05
}

func formatDirectedStrikeAddress(address byte) string {
	return fmt.Sprintf("0x%02X", address)
}

func formatDirectedStrikeHex(data []byte) string {
	parts := make([]string, len(data))
	for index, value := range data {
		parts[index] = fmt.Sprintf("%02X", value)
	}
	return strings.Join(parts, " ")
}

func roundDirectedStrike(value float64) float64 {
	return math.Round(value*100) / 100
}

func directedStrikeOnOff(enabled bool) string {
	if enabled {
		return "开启"
	}
	return "关闭"
}

func directedStrikePTZAction(command byte) string {
	switch command {
	case protocol.DirectedCmdPTZUp:
		return "上移"
	case protocol.DirectedCmdPTZDown:
		return "下移"
	case protocol.DirectedCmdPTZLeft:
		return "左移"
	case protocol.DirectedCmdPTZRight:
		return "右移"
	default:
		return ""
	}
}

func directedStrikeCommandName(command byte) string {
	switch command {
	case protocol.DirectedCmdHeartbeat:
		return "心跳"
	case protocol.DirectedCmdAmpStatusQuery:
		return "功放状态查询"
	case protocol.DirectedCmdSignalSource:
		return "信号源使能"
	case protocol.DirectedCmdWriteFrequency:
		return "宽带频率配置"
	case protocol.DirectedCmdNarrowbandAmpEnable:
		return "窄带功放控制"
	case protocol.DirectedCmdPTZStop:
		return "云台停止"
	case protocol.DirectedCmdPTZSetPreset0:
		return "保存云台预置位"
	case protocol.DirectedCmdPTZUp, protocol.DirectedCmdPTZDown, protocol.DirectedCmdPTZLeft, protocol.DirectedCmdPTZRight:
		return "云台运动"
	case protocol.DirectedCmdPTZHorizontalLocate:
		return "云台水平定位"
	case protocol.DirectedCmdPTZPitchLocate:
		return "云台俯仰定位"
	case protocol.DirectedCmdPTZHorizontalQuery:
		return "云台水平角查询"
	case protocol.DirectedCmdPTZPitchQuery:
		return "云台俯仰角查询"
	case protocol.DirectedCmdPTZSpeedSet:
		return "云台速度设置"
	case protocol.DirectedCmdPTZSpeedQuery:
		return "云台速度查询"
	default:
		return fmt.Sprintf("未知命令 0x%02X", command)
	}
}
