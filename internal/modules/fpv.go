package modules

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/mocks"

	"github.com/sourcegraph/conc"
)

type FPVModule interface {
	Start()
	Stop()
	ConnectedCount() int
}

type fpvClientSession struct {
	conn     net.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	lastSeen time.Time
	isActive bool
	writeMu  sync.Mutex
}

type fpv struct {
	deviceId int
	title    string

	config  *config.Config
	mock    mocks.MockDataGenerator
	ctx     context.Context
	cancel  context.CancelFunc
	clients map[string]*fpvClientSession
	mu      sync.RWMutex
}

func NewFPVModule(deviceId int, mock mocks.MockDataGenerator) FPVModule {
	ctx, cancel := context.WithCancel(context.Background())

	return &fpv{
		deviceId: deviceId,
		title:    "FPV模块",
		config:   config.GetConfig(),
		mock:     mock,
		ctx:      ctx,
		cancel:   cancel,
		clients:  make(map[string]*fpvClientSession),
	}
}

func (f *fpv) ConnectedCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	count := 0
	for _, s := range f.clients {
		if s.isActive && s.conn != nil {
			count++
		}
	}
	return count
}

func (f *fpv) Stop() {
	f.cancel()
	f.mu.Lock()
	for addr, session := range f.clients {
		session.isActive = false
		session.cancel()
		if session.conn != nil {
			session.conn.Close()
			session.conn = nil
		}
		delete(f.clients, addr)
	}
	f.mu.Unlock()
	log.Printf("设备ID=%d [%s] 模块已停止", f.deviceId, f.title)
}

func (f *fpv) Start() {
	log.Printf("设备ID=%d [%s] 模块启动", f.deviceId, f.title)
	// 立即尝试连接一次（在启动goroutine之前）
	f.tryConnect()

	var wg conc.WaitGroup
	defer wg.Wait()

	// 启动连接到多个客户端
	wg.Go(f.connectToClients)

	// 清理不活跃的客户端
	wg.Go(f.cleanupInactiveClients)
}

// getOrCreateSession 获取或创建客户端会话
func (f *fpv) getOrCreateSession(clientAddr string) (*fpvClientSession, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	session, exists := f.clients[clientAddr]
	if exists && session.isActive {
		session.lastSeen = time.Now()
		return session, false
	}

	// 创建新会话
	ctx, cancel := context.WithCancel(f.ctx)
	session = &fpvClientSession{
		ctx:      ctx,
		cancel:   cancel,
		lastSeen: time.Now(),
		isActive: true,
	}
	f.clients[clientAddr] = session

	return session, true
}

func (f *fpv) hasActiveSession(clientAddr string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	session, exists := f.clients[clientAddr]
	return exists && session != nil && session.isActive && session.conn != nil
}

func (f *fpv) attachSessionConn(clientAddr string, session *fpvClientSession, conn net.Conn) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	current, exists := f.clients[clientAddr]
	if !exists || current != session {
		return false
	}

	session.conn = conn
	session.isActive = true
	session.lastSeen = time.Now()
	return true
}

func (f *fpv) snapshotSession(clientAddr string, session *fpvClientSession) (net.Conn, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	current, exists := f.clients[clientAddr]
	if !exists || current != session {
		return nil, false
	}

	return session.conn, session.isActive && session.conn != nil
}

func (f *fpv) markSessionDisconnected(clientAddr string, session *fpvClientSession) {
	f.mu.Lock()
	defer f.mu.Unlock()

	current, exists := f.clients[clientAddr]
	if !exists || current != session {
		return
	}

	session.isActive = false
	session.cancel()
	if session.conn != nil {
		session.conn.Close()
		session.conn = nil
	}
	delete(f.clients, clientAddr)
}

// connectToClients 连接到配置的服务器
func (f *fpv) connectToClients() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			f.tryConnect()
		}
	}
}

// tryConnect 尝试连接到所有配置的服务器
func (f *fpv) tryConnect() {
	// 遍历所有配置的主机
	for _, host := range f.config.FPV.Hosts {
		serverAddr := net.JoinHostPort(host, fmt.Sprintf("%d", f.config.FPV.Port))

		// 检查是否已经有活跃连接
		if f.hasActiveSession(serverAddr) {
			continue
		}

		// 需要建立连接，使用写锁防止并发
		f.mu.Lock()
		// 再次检查，防止竞态
		session, exists := f.clients[serverAddr]
		if exists && session != nil && session.isActive && session.conn != nil {
			f.mu.Unlock()
			continue
		}
		f.mu.Unlock()

		conn, err := net.DialTimeout("tcp", serverAddr, 1*time.Second)
		if err != nil {
			log.Printf("设备ID=%d\thost=%s\tport=%d [%s] 连接失败: %v", f.deviceId, host, f.config.FPV.Port, f.title, err)
			continue
		}

		log.Printf("设备ID=%d\thost=%s\tport=%d [%s] 连接成功", f.deviceId, host, f.config.FPV.Port, f.title)

		session, isNew := f.getOrCreateSession(serverAddr)
		log.Printf("设备ID=%d [%s] getOrCreateSession返回: isNew=%v, serverAddr=%s", f.deviceId, f.title, isNew, serverAddr)
		if !isNew {
			log.Printf("设备ID=%d [%s] 会话已存在，关闭新连接: %s", f.deviceId, f.title, serverAddr)
			conn.Close()
			continue
		}

		if !f.attachSessionConn(serverAddr, session, conn) {
			conn.Close()
			continue
		}

		// uav_defender 平台建立连接后会忽略第一条响应，因此这里主动发送一个初始化回包，
		// 让后续 AT / 点频 / 恢复默认 等同步命令不被“首包忽略”吞掉。
		if err := f.writeSessionPayload(serverAddr, session, "AT+OK\r\n"); err != nil {
			log.Printf("设备ID=%d [%s] 发送初始化响应到 %s 失败: %v", f.deviceId, f.title, serverAddr, err)
			f.markSessionDisconnected(serverAddr, session)
			continue
		}
		f.touchSession(serverAddr, session)

		log.Printf("设备ID=%d [%s] 启动数据发送与命令接收协程到 %s", f.deviceId, f.title, serverAddr)
		go f.handleReportData(serverAddr, session)
		go f.handleDataReception(serverAddr, session)
	}
}

// cleanupInactiveClients 清理不活跃的客户端连接
func (f *fpv) cleanupInactiveClients() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			f.mu.Lock()
			now := time.Now()
			for addr, session := range f.clients {
				if now.Sub(session.lastSeen) > 60*time.Second {
					session.isActive = false
					session.cancel()
					if session.conn != nil {
						session.conn.Close()
						session.conn = nil
					}
					delete(f.clients, addr)
					log.Printf("设备ID=%d [%s] 清理不活跃客户端: %s", f.deviceId, f.title, addr)
				}
			}
			f.mu.Unlock()
		}
	}
}

// handleReportData 处理上报告警数据
func (f *fpv) handleReportData(clientAddr string, session *fpvClientSession) {
	minSpeed := f.config.MinPushSpeed
	maxSpeed := f.config.MaxPushSpeed

	log.Printf("设备ID=%d [%s] 数据发送协程开始运行，目标: %s, 发送间隔: %d-%dms", f.deviceId, f.title, clientAddr, minSpeed, maxSpeed)

	sendCount := 0
	for {
		randomInterval := minSpeed + rand.Intn(maxSpeed-minSpeed+1)

		select {
		case <-session.ctx.Done():
			log.Printf("设备ID=%d [%s] 数据发送协程退出，目标: %s, 已发送: %d条", f.deviceId, f.title, clientAddr, sendCount)
			return
		case <-f.ctx.Done():
			log.Printf("设备ID=%d [%s] 数据发送协程退出，目标: %s, 已发送: %d条", f.deviceId, f.title, clientAddr, sendCount)
			return
		case <-time.After(time.Duration(randomInterval) * time.Millisecond):
			if _, active := f.snapshotSession(clientAddr, session); !active {
				log.Printf("设备ID=%d [%s] 数据发送协程退出（连接无效），目标: %s", f.deviceId, f.title, clientAddr)
				return
			}

			data := f.mock.GenerateFPVWarningData()
			if err := f.writeSessionPayload(clientAddr, session, data.String()); err != nil {
				log.Printf("设备ID=%d [%s] 发送告警数据到 %s 失败: %v", f.deviceId, f.title, clientAddr, err)
				f.markSessionDisconnected(clientAddr, session)
				return
			}

			sendCount++
			if sendCount%100 == 0 {
				log.Printf("设备ID=%d [%s] 已发送 %d 条告警数据到 %s", f.deviceId, f.title, sendCount, clientAddr)
			}

			// 更新最后活跃时间
			f.touchSession(clientAddr, session)
		}
	}
}

func (f *fpv) handleDataReception(clientAddr string, session *fpvClientSession) {
	conn, active := f.snapshotSession(clientAddr, session)
	if !active || conn == nil {
		return
	}

	reader := bufio.NewReader(conn)
	for {
		select {
		case <-session.ctx.Done():
			return
		case <-f.ctx.Done():
			return
		default:
			_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
			line, err := reader.ReadString('\n')
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if errors.Is(err, io.EOF) {
					log.Printf("设备ID=%d [%s] 平台关闭连接: %s", f.deviceId, f.title, clientAddr)
				} else {
					log.Printf("设备ID=%d [%s] 读取平台命令从 %s 失败: %v", f.deviceId, f.title, clientAddr, err)
				}
				f.markSessionDisconnected(clientAddr, session)
				return
			}

			command := strings.TrimSpace(line)
			if command == "" {
				continue
			}

			f.touchSession(clientAddr, session)
			log.Printf("设备ID=%d [%s] 从 %s 接收到命令: %s", f.deviceId, f.title, clientAddr, command)

			response, ok := f.buildCommandResponse(command)
			if !ok {
				log.Printf("设备ID=%d [%s] 忽略未识别命令: %s", f.deviceId, f.title, command)
				continue
			}

			if err := f.writeSessionPayload(clientAddr, session, response); err != nil {
				log.Printf("设备ID=%d [%s] 发送命令响应到 %s 失败: %v", f.deviceId, f.title, clientAddr, err)
				f.markSessionDisconnected(clientAddr, session)
				return
			}

			f.touchSession(clientAddr, session)
			log.Printf("设备ID=%d [%s] 已响应命令 %s 到 %s", f.deviceId, f.title, command, clientAddr)
		}
	}
}

func (f *fpv) buildCommandResponse(command string) (string, bool) {
	switch {
	case command == "AT":
		return "AT+OK\r\n", true
	case strings.HasPrefix(command, "AT+POINT_FREQ="):
		return normalizeFPVResponse(f.mock.GenerateFPVData().String()), true
	case command == "AT+DEFAULT":
		return normalizeFPVResponse(f.mock.GenerateFPVData().String()), true
	default:
		return "", false
	}
}

func (f *fpv) writeSessionPayload(clientAddr string, session *fpvClientSession, payload string) error {
	conn, active := f.snapshotSession(clientAddr, session)
	if !active || conn == nil {
		return net.ErrClosed
	}

	session.writeMu.Lock()
	defer session.writeMu.Unlock()

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := conn.Write([]byte(payload))
	return err
}

func (f *fpv) touchSession(clientAddr string, session *fpvClientSession) {
	f.mu.Lock()
	defer f.mu.Unlock()

	current, exists := f.clients[clientAddr]
	if !exists || current != session {
		return
	}

	session.lastSeen = time.Now()
}

func normalizeFPVResponse(response string) string {
	trimmed := strings.TrimRight(response, "\r\n")
	if trimmed == "" {
		return "\r\n"
	}
	return trimmed + "\r\n"
}
