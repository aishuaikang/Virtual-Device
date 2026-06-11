package modules

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
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
}

type jammingClientSession struct {
	mu       sync.RWMutex
	conn     net.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	lastSeen time.Time
	isActive bool
}

type jamming struct {
	deviceId int
	title    string

	config    *config.Config
	generator *mocks.JammingDataGenerator
	ctx       context.Context
	cancel    context.CancelFunc
	clients   map[string]*jammingClientSession
	mu        sync.RWMutex
}

func NewJammingModule(deviceID int) JammingModule {
	ctx, cancel := context.WithCancel(context.Background())
	return &jamming{
		deviceId:  deviceID,
		title:     "干扰打击模块",
		config:    config.GetConfig(),
		generator: mocks.NewJammingDataGenerator(deviceID, 10),
		ctx:       ctx,
		cancel:    cancel,
		clients:   make(map[string]*jammingClientSession),
	}
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

	// 清理不活跃的连接
	wg.Go(j.cleanupInactiveSessions)

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

	session, exists := j.clients[serverAddr]
	if exists {
		session.mu.Lock()
		if session.isActive {
			session.lastSeen = time.Now()
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
		lastSeen: time.Now(),
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
	if !exists || current != session {
		j.mu.RUnlock()
		return false
	}

	session.mu.Lock()
	j.mu.RUnlock()
	defer session.mu.Unlock()

	session.conn = conn
	session.isActive = true
	session.lastSeen = time.Now()
	return true
}

func (j *jamming) touchSession(serverAddr string, session *jammingClientSession) {
	j.mu.RLock()
	current, exists := j.clients[serverAddr]
	if !exists || current != session {
		j.mu.RUnlock()
		return
	}

	session.mu.Lock()
	j.mu.RUnlock()
	session.lastSeen = time.Now()
	session.mu.Unlock()
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

		conn, err := net.DialTimeout("tcp", serverAddr, 1*time.Second)
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

// cleanupInactiveSessions 清理不活跃的会话连接
func (j *jamming) cleanupInactiveSessions() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-j.ctx.Done():
			return
		case <-ticker.C:
			j.mu.Lock()
			for addr, session := range j.clients {
				session.mu.Lock()
				expired := time.Since(session.lastSeen) > 60*time.Second
				if expired {
					log.Printf("设备ID=%d [%s] 清理不活跃连接: %s", j.deviceId, j.title, addr)
					session.isActive = false
					session.cancel()
					if session.conn != nil {
						session.conn.Close()
						session.conn = nil
					}
					delete(j.clients, addr)
				}
				session.mu.Unlock()
			}
			j.mu.Unlock()
		}
	}
}

// markSessionDisconnected 标记会话断开
func (j *jamming) markSessionDisconnected(serverAddr string, session *jammingClientSession) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if s, exists := j.clients[serverAddr]; exists && s == session {
		session.mu.Lock()
		session.isActive = false
		session.cancel()
		if session.conn != nil {
			session.conn.Close()
			session.conn = nil
		}
		session.mu.Unlock()
		delete(j.clients, serverAddr)
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

			// 使用锁保护连接访问
			session.mu.Lock()
			if !session.isActive || session.conn == nil {
				session.mu.Unlock()
				log.Printf("设备ID=%d [%s] 连接已失效，停止发送", j.deviceId, j.title)
				j.markSessionDisconnected(serverAddr, session)
				return
			}

			session.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, writeErr := session.conn.Write(data)
			session.lastSeen = time.Now()
			session.mu.Unlock()

			if writeErr != nil {
				log.Printf("设备ID=%d [%s] 向 %s 发送数据失败: %v", j.deviceId, j.title, serverAddr, writeErr)
				j.markSessionDisconnected(serverAddr, session)
				return
			}
		}
	}
}

// handleDataReception 处理接收与响应
func (j *jamming) handleDataReception(serverAddr string, session *jammingClientSession) {
	buf := make([]byte, 2048)
	for {
		select {
		case <-session.ctx.Done():
			return
		default:
			session.mu.RLock()
			if !session.isActive || session.conn == nil {
				session.mu.RUnlock()
				j.markSessionDisconnected(serverAddr, session)
				return
			}

			session.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			n, err := session.conn.Read(buf)
			session.mu.RUnlock()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("设备ID=%d [%s] 从 %s 读取数据失败: %v", j.deviceId, j.title, serverAddr, err)
				j.markSessionDisconnected(serverAddr, session)
				return
			}

			if n <= 0 {
				continue
			}

			j.touchSession(serverAddr, session)

			log.Printf("[打击] 设备ID=%d 从 %s 接收到 %d 字节数据", j.deviceId, serverAddr, n)

			// 解析命令
			packet, err := protocol.ParseJammingPacket(buf[:n])
			if err != nil {
				log.Printf("[打击] 设备ID=%d 从 %s 解析数据包失败: %v", j.deviceId, serverAddr, err)
				continue
			}

			log.Printf("[打击] 设备ID=%d 接收到命令: Cmd=0x%02X, DataLen=%d", j.deviceId, packet.Cmd, len(packet.Data))

			// 处理命令并响应
			resp := j.handleCommand(packet)
			if resp != nil {
				data := resp.ToBytes()
				if data == nil {
					log.Printf("设备ID=%d [%s] 响应数据包转换字节失败", j.deviceId, j.title)
					continue
				}

				// 使用锁保护连接访问
				session.mu.Lock()
				if !session.isActive || session.conn == nil {
					session.mu.Unlock()
					log.Printf("设备ID=%d [%s] 连接已失效，停止发送响应", j.deviceId, j.title)
					j.markSessionDisconnected(serverAddr, session)
					return
				}

				session.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				_, writeErr := session.conn.Write(data)
				session.mu.Unlock()

				if writeErr != nil {
					log.Printf("[打击] 设备ID=%d 向 %s 发送响应失败: %v", j.deviceId, serverAddr, writeErr)
					j.markSessionDisconnected(serverAddr, session)
					return
				}
				log.Printf("[打击] 设备ID=%d 向 %s 发送响应成功，响应长度: %d 字节", j.deviceId, serverAddr, len(data))
			} else {
				log.Printf("[打击] 设备ID=%d 命令处理返回空响应，不发送数据", j.deviceId)
			}
		}
	}
}

func (j *jamming) handleCommand(packet *protocol.JammingPacket) *protocol.JammingPacket {
	if j.generator == nil {
		log.Printf("[打击] 设备ID=%d generator 为 nil，返回通用成功响应", j.deviceId)
		// 即使 generator 为 nil，也返回成功响应
		return protocol.CreateSetPowerResponse(j.deviceId, 0)
	}

	log.Printf("[打击] 设备ID=%d 开始处理命令: 0x%02X", j.deviceId, packet.Cmd)

	var resp *protocol.JammingPacket
	switch packet.Cmd {
	case protocol.CmdSetPower:
		log.Printf("[打击] 设备ID=%d 处理设置功率命令", j.deviceId)
		resp = j.generator.HandleSetPowerCommand(packet)
	case protocol.CmdQueryModules:
		log.Printf("[打击] 设备ID=%d 处理查询模块命令", j.deviceId)
		resp = j.generator.GenerateModuleInfoPacket()
	case protocol.CmdDownloadAddr, protocol.CmdQueryAddr:
		log.Printf("[打击] 设备ID=%d 处理地址列表命令", j.deviceId)
		resp = j.generator.HandleAddressListCommand(packet)
	default:
		log.Printf("[打击] 设备ID=%d 未知命令: 0x%02X，返回通用成功响应", j.deviceId, packet.Cmd)
		// 对于未知命令，也返回成功响应
		return protocol.CreateSetPowerResponse(j.deviceId, 0)
	}

	if resp != nil {
		log.Printf("[打击] 设备ID=%d 命令处理完成，生成响应: Cmd=0x%02X, DataLen=%d", j.deviceId, resp.Cmd, len(resp.Data))
	} else {
		log.Printf("[打击] 设备ID=%d 命令处理返回空响应，使用通用成功响应", j.deviceId)
		// 如果处理函数返回 nil，返回通用成功响应
		resp = protocol.CreateSetPowerResponse(j.deviceId, 0)
	}

	return resp
}

// Stop 停止打击模块
func (j *jamming) Stop() {
	log.Printf("[打击] 设备ID=%d 开始停止模块", j.deviceId)

	// 取消context，通知所有goroutines停止
	j.cancel()

	// 关闭所有连接
	j.mu.Lock()
	for addr, session := range j.clients {
		log.Printf("[打击] 设备ID=%d 关闭连接: %s", j.deviceId, addr)
		session.mu.Lock()
		session.isActive = false
		session.cancel()
		if session.conn != nil {
			session.conn.Close()
			session.conn = nil
		}
		session.mu.Unlock()
	}
	j.clients = make(map[string]*jammingClientSession)
	j.mu.Unlock()

	log.Printf("[打击] 设备ID=%d 模块已停止", j.deviceId)
}
