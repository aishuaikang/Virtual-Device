package modules

import (
	"context"
	"fmt"
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

const analysisHeartbeatInterval = 10 * time.Second

type AnalysisModule interface {
	Start()
	Stop()
	ConnectedCount() int
}

type analysisClientSession struct {
	conn     net.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	lastSeen time.Time
	isActive bool
}

type analysis struct {
	deviceIDs         []int
	title             string
	heartbeatInterval time.Duration

	config  *config.Config
	mock    mocks.MockDataGenerator
	ctx     context.Context
	cancel  context.CancelFunc
	clients map[string]*analysisClientSession
	mu      sync.RWMutex
}

func NewAnalysisModule(deviceIDs []int, mock mocks.MockDataGenerator) AnalysisModule {
	ctx, cancel := context.WithCancel(context.Background())

	return &analysis{
		deviceIDs:         deviceIDs,
		title:             "解析模块",
		heartbeatInterval: analysisHeartbeatInterval,
		config:            config.GetConfig(),
		mock:              mock,
		ctx:               ctx,
		cancel:            cancel,
		clients:           make(map[string]*analysisClientSession),
	}
}

func (a *analysis) ConnectedCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	count := 0
	for _, s := range a.clients {
		if s.isActive && s.conn != nil {
			count++
		}
	}
	return count
}

func (a *analysis) Stop() {
	a.cancel()
	a.mu.Lock()
	for addr, session := range a.clients {
		session.isActive = false
		session.cancel()
		if session.conn != nil {
			session.conn.Close()
			session.conn = nil
		}
		delete(a.clients, addr)
	}
	a.mu.Unlock()
	log.Printf("设备ID=%v [%s] 模块已停止", a.deviceIDs, a.title)
}

func (a *analysis) Start() {
	log.Printf("设备ID=%v [%s] 模块启动", a.deviceIDs, a.title)
	// 立即尝试连接一次（在启动goroutine之前）
	a.tryConnect()

	var wg conc.WaitGroup
	defer wg.Wait()

	// 启动连接到多个客户端
	wg.Go(a.connectToClients)

	// 清理不活跃的客户端
	wg.Go(a.cleanupInactiveClients)
}

// getOrCreateSession 获取或创建客户端会话
func (a *analysis) getOrCreateSession(clientAddr string) (*analysisClientSession, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, exists := a.clients[clientAddr]
	if exists && session.isActive {
		session.lastSeen = time.Now()
		return session, false
	}

	// 创建新会话
	ctx, cancel := context.WithCancel(a.ctx)
	session = &analysisClientSession{
		ctx:      ctx,
		cancel:   cancel,
		lastSeen: time.Now(),
		isActive: true,
	}
	a.clients[clientAddr] = session

	return session, true
}

func (a *analysis) hasActiveSession(clientAddr string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	session, exists := a.clients[clientAddr]
	return exists && session != nil && session.isActive && session.conn != nil
}

func (a *analysis) attachSessionConn(clientAddr string, session *analysisClientSession, conn net.Conn) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	current, exists := a.clients[clientAddr]
	if !exists || current != session {
		return false
	}

	session.conn = conn
	session.isActive = true
	session.lastSeen = time.Now()
	return true
}

func (a *analysis) snapshotSession(clientAddr string, session *analysisClientSession) (net.Conn, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	current, exists := a.clients[clientAddr]
	if !exists || current != session {
		return nil, false
	}

	return session.conn, session.isActive && session.conn != nil
}

func (a *analysis) markSessionDisconnected(clientAddr string, session *analysisClientSession) {
	a.mu.Lock()
	defer a.mu.Unlock()

	current, exists := a.clients[clientAddr]
	if !exists || current != session {
		return
	}

	session.isActive = false
	session.cancel()
	if session.conn != nil {
		session.conn.Close()
		session.conn = nil
	}
	delete(a.clients, clientAddr)
}

// connectToClients 连接到配置的服务器
func (a *analysis) connectToClients() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.tryConnect()
		}
	}
}

// tryConnect 尝试连接到所有配置的服务器
func (a *analysis) tryConnect() {
	// 遍历所有配置的主机
	for _, host := range a.config.Analysis.Hosts {
		serverAddr := net.JoinHostPort(host, fmt.Sprintf("%d", a.config.Analysis.Port))

		// 检查是否已经有活跃连接
		if a.hasActiveSession(serverAddr) {
			continue
		}

		// 需要建立连接，使用写锁防止并发
		a.mu.Lock()
		// 再次检查，防止竞态
		session, exists := a.clients[serverAddr]
		if exists && session != nil && session.isActive && session.conn != nil {
			a.mu.Unlock()
			continue
		}
		a.mu.Unlock()

		conn, err := net.DialTimeout("tcp", serverAddr, 1*time.Second)
		if err != nil {
			log.Printf("设备ID=%v\thost=%s\tport=%d [%s] 连接失败: %v", a.deviceIDs, host, a.config.Analysis.Port, a.title, err)
			continue
		}

		log.Printf("设备ID=%v\thost=%s\tport=%d [%s] 连接成功", a.deviceIDs, host, a.config.Analysis.Port, a.title)

		session, isNew := a.getOrCreateSession(serverAddr)
		log.Printf("设备ID=%v [%s] getOrCreateSession返回: isNew=%v, serverAddr=%s", a.deviceIDs, a.title, isNew, serverAddr)
		if !isNew {
			log.Printf("设备ID=%v [%s] 会话已存在，关闭新连接: %s", a.deviceIDs, a.title, serverAddr)
			conn.Close()
			continue
		}

		if !a.attachSessionConn(serverAddr, session, conn) {
			conn.Close()
			continue
		}

		log.Printf("设备ID=%v [%s] 启动数据发送协程到 %s", a.deviceIDs, a.title, serverAddr)
		// 为每个客户端启动数据发送协程
		go a.handleReportData(serverAddr, session)
	}
}

// cleanupInactiveClients 清理不活跃的客户端连接
func (a *analysis) cleanupInactiveClients() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.mu.Lock()
			now := time.Now()
			for addr, session := range a.clients {
				if now.Sub(session.lastSeen) > 60*time.Second {
					session.isActive = false
					session.cancel()
					if session.conn != nil {
						session.conn.Close()
						session.conn = nil
					}
					delete(a.clients, addr)
					log.Printf("设备ID=%v [%s] 清理不活跃客户端: %s", a.deviceIDs, a.title, addr)
				}
			}
			a.mu.Unlock()
		}
	}
}

// handleReportData 处理上报解析数据
func (a *analysis) handleReportData(clientAddr string, session *analysisClientSession) {
	minSpeed := a.config.MinPushSpeed
	maxSpeed := a.config.MaxPushSpeed
	heartbeatInterval := a.heartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = analysisHeartbeatInterval
	}

	log.Printf("设备ID=%v [%s] 数据发送协程开始运行，目标: %s, 发送间隔: %d-%dms, 心跳间隔: %s", a.deviceIDs, a.title, clientAddr, minSpeed, maxSpeed, heartbeatInterval)

	sendCount := 0
	heartbeatSequence := 0
	reportTimer := time.NewTimer(a.nextReportInterval(minSpeed, maxSpeed))
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer reportTimer.Stop()
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-session.ctx.Done():
			log.Printf("设备ID=%v [%s] 数据发送协程退出（session取消），目标: %s, 已发送: %d条", a.deviceIDs, a.title, clientAddr, sendCount)
			return
		case <-a.ctx.Done():
			log.Printf("设备ID=%v [%s] 数据发送协程退出（模块停止），目标: %s, 已发送: %d条", a.deviceIDs, a.title, clientAddr, sendCount)
			return
		case <-reportTimer.C:
			// 每轮只生成一次，逐设备复制报文，保持无人机数据与加密内容一致。
			data := a.mock.GenerateAnalysisData(a.deviceIDs[0]).String()
			deviceField := fmt.Sprintf("device=%d,", a.deviceIDs[0])
			for _, deviceID := range a.deviceIDs {
				payload := strings.ReplaceAll(data, deviceField, fmt.Sprintf("device=%d,", deviceID))
				if !a.sendSessionData(clientAddr, session, "业务数据", payload) {
					return
				}

				sendCount++
				// 每100条数据打印一次日志，避免日志过多
				if sendCount%100 == 0 {
					log.Printf("设备ID=%v [%s] 已发送 %d 条数据到 %s", a.deviceIDs, a.title, sendCount, clientAddr)
				}
			}
			reportTimer.Reset(a.nextReportInterval(minSpeed, maxSpeed))
		case <-heartbeatTicker.C:
			heartbeatSequence++
			frequencyMHz := 1900 + rand.Intn(51)
			temperature := 18 + rand.Intn(38)
			for _, deviceID := range a.deviceIDs {
				heartbeat := formatAnalysisHeartbeat(deviceID, heartbeatSequence, frequencyMHz, temperature)
				if !a.sendSessionData(clientAddr, session, "心跳", heartbeat) {
					return
				}
			}
		}
	}
}

func (a *analysis) nextReportInterval(minSpeed, maxSpeed int) time.Duration {
	return time.Duration(minSpeed+rand.Intn(maxSpeed-minSpeed+1)) * time.Millisecond
}

func (a *analysis) sendSessionData(clientAddr string, session *analysisClientSession, dataType, payload string) bool {
	conn, active := a.snapshotSession(clientAddr, session)
	if !active || conn == nil {
		log.Printf("设备ID=%v [%s] 数据发送协程退出（连接无效），目标: %s", a.deviceIDs, a.title, clientAddr)
		return false
	}

	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		log.Printf("设备ID=%v [%s] 设置%s写入超时失败，目标: %s, 错误: %v", a.deviceIDs, a.title, dataType, clientAddr, err)
		a.markSessionDisconnected(clientAddr, session)
		return false
	}
	if _, err := conn.Write([]byte(payload)); err != nil {
		log.Printf("设备ID=%v [%s] 发送%s到 %s 失败: %v", a.deviceIDs, a.title, dataType, clientAddr, err)
		a.markSessionDisconnected(clientAddr, session)
		return false
	}

	a.mu.Lock()
	current, exists := a.clients[clientAddr]
	if exists && current == session {
		session.lastSeen = time.Now()
	}
	a.mu.Unlock()
	return true
}

func formatAnalysisHeartbeat(deviceID, sequence, frequencyMHz, temperature int) string {
	return fmt.Sprintf("#=%d, device=%d, Heart Beat, %d-%d\r\n", sequence, deviceID, frequencyMHz, temperature)
}
