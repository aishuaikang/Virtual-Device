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

	"github.com/sourcegraph/conc"
)

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
	deviceId int
	title    string

	config  *config.Config
	mock    mocks.MockDataGenerator
	ctx     context.Context
	cancel  context.CancelFunc
	clients map[string]*analysisClientSession
	mu      sync.RWMutex
}

func NewAnalysisModule(deviceId int, mock mocks.MockDataGenerator) AnalysisModule {
	ctx, cancel := context.WithCancel(context.Background())

	return &analysis{
		deviceId: deviceId,
		title:    "解析模块",
		config:   config.GetConfig(),
		mock:     mock,
		ctx:      ctx,
		cancel:   cancel,
		clients:  make(map[string]*analysisClientSession),
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
	log.Printf("设备ID=%d [%s] 模块已停止", a.deviceId, a.title)
}

func (a *analysis) Start() {
	log.Printf("设备ID=%d [%s] 模块启动", a.deviceId, a.title)
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
			log.Printf("设备ID=%d\thost=%s\tport=%d [%s] 连接失败: %v", a.deviceId, host, a.config.Analysis.Port, a.title, err)
			continue
		}

		log.Printf("设备ID=%d\thost=%s\tport=%d [%s] 连接成功", a.deviceId, host, a.config.Analysis.Port, a.title)

		session, isNew := a.getOrCreateSession(serverAddr)
		log.Printf("设备ID=%d [%s] getOrCreateSession返回: isNew=%v, serverAddr=%s", a.deviceId, a.title, isNew, serverAddr)
		if !isNew {
			log.Printf("设备ID=%d [%s] 会话已存在，关闭新连接: %s", a.deviceId, a.title, serverAddr)
			conn.Close()
			continue
		}

		if !a.attachSessionConn(serverAddr, session, conn) {
			conn.Close()
			continue
		}

		log.Printf("设备ID=%d [%s] 启动数据发送协程到 %s", a.deviceId, a.title, serverAddr)
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
					log.Printf("设备ID=%d [%s] 清理不活跃客户端: %s", a.deviceId, a.title, addr)
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

	log.Printf("设备ID=%d [%s] 数据发送协程开始运行，目标: %s, 发送间隔: %d-%dms", a.deviceId, a.title, clientAddr, minSpeed, maxSpeed)

	sendCount := 0
	for {
		randomInterval := minSpeed + rand.Intn(maxSpeed-minSpeed+1)

		select {
		case <-session.ctx.Done():
			log.Printf("设备ID=%d [%s] 数据发送协程退出（session取消），目标: %s, 已发送: %d条", a.deviceId, a.title, clientAddr, sendCount)
			return
		case <-a.ctx.Done():
			log.Printf("设备ID=%d [%s] 数据发送协程退出（模块停止），目标: %s, 已发送: %d条", a.deviceId, a.title, clientAddr, sendCount)
			return
		case <-time.After(time.Duration(randomInterval) * time.Millisecond):
			conn, active := a.snapshotSession(clientAddr, session)
			if !active || conn == nil {
				log.Printf("设备ID=%d [%s] 数据发送协程退出（连接无效），目标: %s", a.deviceId, a.title, clientAddr)
				return
			}

			data := a.mock.GenerateAnalysisData(a.deviceId)
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := conn.Write([]byte(data.String()))
			if err != nil {
				log.Printf("设备ID=%d [%s] 发送数据到 %s 失败: %v", a.deviceId, a.title, clientAddr, err)
				a.markSessionDisconnected(clientAddr, session)
				return
			}

			sendCount++
			// 每100条数据打印一次日志，避免日志过多
			if sendCount%100 == 0 {
				log.Printf("设备ID=%d [%s] 已发送 %d 条数据到 %s", a.deviceId, a.title, sendCount, clientAddr)
			}

			// 更新最后活跃时间
			a.mu.Lock()
			session.lastSeen = time.Now()
			a.mu.Unlock()
		}
	}
}
