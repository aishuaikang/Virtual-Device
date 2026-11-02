package modules

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"slices"
	"sync"
	"time"
	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/mocks"
	"virtual-device-ui/internal/protocol"

	"github.com/sourcegraph/conc"
)

// JammingModule TCP客户端接口
type JammingModule interface {
	Start()
	Stop()
	ConnectedCount() int
	UpdateSimulation(settings config.JammingConfig) error
}

type jammingClientSession struct {
	mu       sync.RWMutex
	writeMu  sync.Mutex
	conn     net.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	isActive bool
}

func (s *jammingClientSession) activeConn() net.Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.isActive || s.ctx.Err() != nil {
		return nil
	}
	return s.conn
}

func (s *jammingClientSession) write(data []byte) error {
	// Serialize packets without holding the session lock during network I/O,
	// so Stop can close the connection and interrupt a blocked read or write.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	connection := s.activeConn()
	if connection == nil {
		return net.ErrClosed
	}
	if err := connection.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	_, err := connection.Write(data)
	return err
}

type jamming struct {
	deviceId int
	title    string

	config     *config.Config
	generator  *mocks.JammingDataGenerator
	ctx        context.Context
	cancel     context.CancelFunc
	clients    map[string]*jammingClientSession
	simulation config.JammingConfig
	mu         sync.RWMutex
}

func NewJammingModule(deviceID int) JammingModule {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.GetConfig()
	module := &jamming{
		deviceId:   deviceID,
		title:      "干扰打击模块",
		config:     cfg,
		generator:  mocks.NewJammingDataGenerator(deviceID, 10),
		ctx:        ctx,
		cancel:     cancel,
		clients:    make(map[string]*jammingClientSession),
		simulation: cfg.Jamming,
	}
	module.simulation.Hosts = slices.Clone(cfg.Jamming.Hosts)
	if module.simulation.FaultMode == config.JammingFaultStuckOn {
		module.generator.SetStuckChannel(module.simulation.FaultChannel)
	}
	return module
}

// UpdateSimulation changes faults without resetting physical channel state.
func (j *jamming) UpdateSimulation(settings config.JammingConfig) error {
	if err := config.ValidateJammingSimulation(settings); err != nil {
		return err
	}
	j.mu.Lock()
	current := j.simulation
	if settings.Enabled != current.Enabled || settings.DeviceID != current.DeviceID ||
		settings.Port != current.Port || !slices.Equal(settings.Hosts, current.Hosts) {
		j.mu.Unlock()
		return fmt.Errorf("请先停止模拟器再修改全向连接配置")
	}
	j.simulation = settings
	j.simulation.Hosts = slices.Clone(settings.Hosts)
	channel := 0
	if settings.FaultMode == config.JammingFaultStuckOn {
		channel = settings.FaultChannel
	}
	j.generator.SetStuckChannel(channel)
	var disconnected []*jammingClientSession
	if settings.FaultMode == config.JammingFaultDisconnected {
		for _, session := range j.clients {
			disconnected = append(disconnected, session)
		}
		j.clients = make(map[string]*jammingClientSession)
	}
	j.mu.Unlock()
	for _, session := range disconnected {
		closeJammingSession(session)
	}
	return nil
}

func (j *jamming) ConnectedCount() int {
	j.mu.RLock()
	defer j.mu.RUnlock()
	count := 0
	for _, s := range j.clients {
		s.mu.RLock()
		active := s.isActive && s.conn != nil
		s.mu.RUnlock()
		if active {
			count++
		}
	}
	return count
}

func (j *jamming) Start() {
	log.Printf("设备ID=%d [%s] 模块启动", j.deviceId, j.title)
	// 立即尝试连接一次（在启动goroutine之前）
	j.tryConnect()

	var wg conc.WaitGroup

	// 启动连接到多个服务端
	wg.Go(j.connectToServers)

	// 无人值守允许长时间没有控制命令；连接由实际读写错误或 Stop 清理。

	// 等待context取消信号
	<-j.ctx.Done()
	log.Printf("[打击] 设备ID=%d 收到停止信号，等待goroutines结束", j.deviceId)

	// 等待所有goroutines结束
	wg.Wait()
	log.Printf("[打击] 设备ID=%d 所有goroutines已结束", j.deviceId)
}

// getOrCreateSession 获取或创建客户端会话
func (j *jamming) getOrCreateSession(serverAddr string) (*jammingClientSession, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.ctx.Err() != nil || j.simulation.FaultMode == config.JammingFaultDisconnected {
		return nil, false
	}

	session, exists := j.clients[serverAddr]
	if exists {
		session.mu.Lock()
		if session.isActive {
			session.mu.Unlock()
			return session, false
		}
		session.mu.Unlock()
	}

	// 创建新会话
	ctx, cancel := context.WithCancel(j.ctx)
	session = &jammingClientSession{
		ctx:      ctx,
		cancel:   cancel,
		isActive: true,
	}
	j.clients[serverAddr] = session

	return session, true
}

func (j *jamming) hasActiveSession(serverAddr string) bool {
	j.mu.RLock()
	defer j.mu.RUnlock()

	session, exists := j.clients[serverAddr]
	if !exists || session == nil {
		return false
	}

	session.mu.RLock()
	defer session.mu.RUnlock()
	return session.isActive && session.conn != nil
}

func (j *jamming) attachSessionConn(serverAddr string, session *jammingClientSession, conn net.Conn) bool {
	j.mu.RLock()
	current, exists := j.clients[serverAddr]
	if !exists || current != session || j.simulation.FaultMode == config.JammingFaultDisconnected {
		j.mu.RUnlock()
		return false
	}

	session.mu.Lock()
	j.mu.RUnlock()
	defer session.mu.Unlock()
	if !session.isActive || session.ctx.Err() != nil || j.ctx.Err() != nil {
		return false
	}

	session.conn = conn
	session.isActive = true
	return true
}

// connectToServers 连接到配置的服务器
func (j *jamming) connectToServers() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-j.ctx.Done():
			return
		case <-ticker.C:
			j.tryConnect()
		}
	}
}

// tryConnect 尝试连接到所有配置的服务器
func (j *jamming) tryConnect() {
	j.mu.RLock()
	disconnected := j.simulation.FaultMode == config.JammingFaultDisconnected
	j.mu.RUnlock()
	if disconnected || j.ctx.Err() != nil {
		return
	}
	for _, host := range j.config.Jamming.Hosts {
		serverAddr := net.JoinHostPort(host, fmt.Sprintf("%d", j.config.Jamming.Port))

		// 检查是否已经有活跃连接
		if j.hasActiveSession(serverAddr) {
			continue
		}

		// 需要建立连接，使用写锁防止并发
		j.mu.Lock()
		// 再次检查，防止竞态
		session, exists := j.clients[serverAddr]
		if exists && session != nil {
			session.mu.RLock()
			active := session.isActive && session.conn != nil
			session.mu.RUnlock()
			if active {
				j.mu.Unlock()
				continue
			}
		}
		j.mu.Unlock()

		dialer := net.Dialer{Timeout: time.Second}
		conn, err := dialer.DialContext(j.ctx, "tcp", serverAddr)
		if err != nil {
			log.Printf("设备ID=%d\thost=%s\tport=%d [%s] 连接失败: %v", j.deviceId, host, j.config.Jamming.Port, j.title, err)
			continue
		}

		log.Printf("设备ID=%d\thost=%s\tport=%d [%s] 连接成功", j.deviceId, host, j.config.Jamming.Port, j.title)

		session, isNew := j.getOrCreateSession(serverAddr)
		if !isNew {
			conn.Close()
			continue
		}

		if !j.attachSessionConn(serverAddr, session, conn) {
			conn.Close()
			continue
		}

		// 为每个服务器启动数据接收协程（已禁用实时上报）
		// go j.handleReportData(serverAddr, session)  // 已注释：去掉实时上报
		go j.handleDataReception(serverAddr, session)
	}
}

// markSessionDisconnected 标记会话断开
func (j *jamming) markSessionDisconnected(serverAddr string, session *jammingClientSession) {
	j.mu.Lock()
	if s, exists := j.clients[serverAddr]; exists && s == session {
		delete(j.clients, serverAddr)
	}
	j.mu.Unlock()
	closeJammingSession(session)
}

func closeJammingSession(session *jammingClientSession) {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.isActive = false
	session.cancel()
	if session.conn != nil {
		session.conn.Close()
		session.conn = nil
	}
}

// handleReportData 定时上报模块信息
func (j *jamming) handleReportData(serverAddr string, session *jammingClientSession) {
	minSpeed := j.config.MinPushSpeed
	maxSpeed := j.config.MaxPushSpeed

	for {
		interval := minSpeed + rand.Intn(maxSpeed-minSpeed+1)
		select {
		case <-session.ctx.Done():
			return
		case <-time.After(time.Duration(interval) * time.Millisecond):
			session.mu.RLock()
			if !session.isActive || session.conn == nil {
				session.mu.RUnlock()
				j.markSessionDisconnected(serverAddr, session)
				return
			}
			session.mu.RUnlock()

			// 检查 generator 是否为 nil
			if j.generator == nil {
				log.Printf("设备ID=%d [%s] generator 为 nil，跳过此次发送", j.deviceId, j.title)
				continue
			}

			// 发送模块信息上报
			pkt := j.generator.GenerateModuleInfoPacket()
			if pkt == nil {
				log.Printf("设备ID=%d [%s] 生成的数据包为 nil，跳过此次发送", j.deviceId, j.title)
				continue
			}

			data := pkt.ToBytes()
			if data == nil {
				log.Printf("设备ID=%d [%s] 数据包转换字节失败，跳过此次发送", j.deviceId, j.title)
				continue
			}

			if writeErr := session.write(data); writeErr != nil {
				log.Printf("设备ID=%d [%s] 向 %s 发送数据失败: %v", j.deviceId, j.title, serverAddr, writeErr)
				j.markSessionDisconnected(serverAddr, session)
				return
			}
		}
	}
}

// handleDataReception 处理接收与响应
func (j *jamming) handleDataReception(serverAddr string, session *jammingClientSession) {
	defer j.markSessionDisconnected(serverAddr, session)
	buf := make([]byte, 2048)
	var decoder protocol.JammingStreamDecoder
	for session.ctx.Err() == nil {
		connection := session.activeConn()
		if connection == nil {
			return
		}
		if err := connection.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			return
		}
		n, readErr := connection.Read(buf)
		if n > 0 {
			log.Printf("[打击] 设备ID=%d RX %s: %X", j.deviceId, serverAddr, buf[:n])
			for _, packet := range decoder.Feed(buf[:n]) {
				if session.ctx.Err() != nil {
					return
				}
				response := j.handleCommand(packet)
				if response == nil {
					continue
				}
				j.mu.RLock()
				delay := j.simulation.ResponseDelayMS
				j.mu.RUnlock()
				if delay > 0 {
					timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
					select {
					case <-session.ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
				}
				data := response.ToBytes()
				if err := session.write(data); err != nil {
					log.Printf("[打击] 设备ID=%d 向 %s 发送响应失败: %v", j.deviceId, serverAddr, err)
					return
				}
				log.Printf("[打击] 设备ID=%d TX %s: %X", j.deviceId, serverAddr, data)
			}
		}
		if readErr != nil {
			if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("[打击] 设备ID=%d 从 %s 读取数据失败: %v", j.deviceId, serverAddr, readErr)
			return
		}
	}
}

func (j *jamming) handleCommand(packet *protocol.JammingPacket) *protocol.JammingPacket {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if j.generator == nil || j.simulation.FaultMode == config.JammingFaultDisconnected {
		return nil
	}
	var response *protocol.JammingPacket
	switch packet.Cmd {
	case protocol.CmdSetPower:
		response = j.generator.HandleSetPowerCommand(packet)
	case protocol.CmdQueryModules:
		if len(packet.Data) != 0 {
			return nil
		}
		response = j.generator.GenerateModuleInfoPacket()
		if j.simulation.FaultMode == config.JammingFaultPartialStatus {
			response.Data = response.Data[:9*11]
			response.Length = uint8(6 + len(response.Data))
		}
	case protocol.CmdDownloadAddr, protocol.CmdQueryAddr:
		response = j.generator.HandleAddressListCommand(packet)
	default:
		log.Printf("[打击] 设备ID=%d 忽略未知命令: 0x%02X", j.deviceId, packet.Cmd)
		return nil
	}
	if j.simulation.FaultMode == config.JammingFaultSilent ||
		(j.simulation.FaultMode == config.JammingFaultDropACK && packet.Cmd == protocol.CmdSetPower) ||
		(j.simulation.FaultMode == config.JammingFaultDropStatus && packet.Cmd == protocol.CmdQueryModules) {
		log.Printf("[打击] 设备ID=%d 故障模式 %s 丢弃响应: Cmd=0x%02X", j.deviceId, j.simulation.FaultMode, packet.Cmd)
		return nil
	}
	return response
}

// Stop 停止打击模块
func (j *jamming) Stop() {
	log.Printf("[打击] 设备ID=%d 开始停止模块", j.deviceId)

	// 取消context，通知所有goroutines停止
	j.cancel()

	// Detach sessions before closing sockets; network I/O never holds the module lock.
	j.mu.Lock()
	sessions := j.clients
	j.clients = make(map[string]*jammingClientSession)
	j.mu.Unlock()
	for _, session := range sessions {
		closeJammingSession(session)
	}

	log.Printf("[打击] 设备ID=%d 模块已停止", j.deviceId)
}
