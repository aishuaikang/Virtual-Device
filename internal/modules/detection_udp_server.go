package modules

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/mocks"
	"virtual-device-ui/internal/utils"

	"github.com/sourcegraph/conc"
)

type DetectionUDPServerModule interface {
	Start() error
	Stop()
	ClientCount() int
	Snapshot() DetectionStatusSnapshot
}

type DetectionStatusSnapshot struct {
	LastActivity    time.Time
	ClientAddresses []string
}

// clientSession 客户端会话
type clientSession struct {
	addr        *net.UDPAddr
	ctx         context.Context
	cancel      context.CancelFunc
	lastSeen    time.Time
	isActive    bool
	mu          sync.Mutex              // 保护会话的并发操作
	isRunning   bool                    // 标记任务是否已启动
	lastCommand *utils.DetectionCommand // 保存最后一次命令
}

type detectionUDPServer struct {
	deviceId     int
	title        string
	detectionCfg config.DetectionConfig

	config *config.Config
	mock   mocks.MockDataGenerator

	conns    []*net.UDPConn
	clients  map[string]*clientSession // key: addr.String()
	clientMu sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewDetectionUDPModule(detectionCfg config.DetectionConfig, mock mocks.MockDataGenerator) DetectionUDPServerModule {
	ctx, cancel := context.WithCancel(context.Background())

	return &detectionUDPServer{
		deviceId:     detectionCfg.DeviceID,
		title:        "侦测模块_UDP服务",
		detectionCfg: detectionCfg,
		config:       config.GetConfig(),
		mock:         mock,
		clients:      make(map[string]*clientSession),
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (f *detectionUDPServer) ClientCount() int {
	f.clientMu.RLock()
	defer f.clientMu.RUnlock()
	count := 0
	for _, s := range f.clients {
		if s.isActive {
			count++
		}
	}
	return count
}

func (f *detectionUDPServer) Snapshot() DetectionStatusSnapshot {
	f.clientMu.RLock()
	defer f.clientMu.RUnlock()

	snapshot := DetectionStatusSnapshot{
		ClientAddresses: make([]string, 0, len(f.clients)),
	}

	for addr, session := range f.clients {
		if !session.isActive {
			continue
		}

		snapshot.ClientAddresses = append(snapshot.ClientAddresses, fmt.Sprintf("device=%d %s:%d <- %s", f.deviceId, f.detectionCfg.Host, f.detectionCfg.Port, addr))
		if session.lastSeen.After(snapshot.LastActivity) {
			snapshot.LastActivity = session.lastSeen
		}
	}

	sort.Strings(snapshot.ClientAddresses)
	return snapshot
}

func (f *detectionUDPServer) touchSession(session *clientSession) {
	f.clientMu.Lock()
	defer f.clientMu.Unlock()

	session.lastSeen = time.Now()
	session.isActive = true
}

func (f *detectionUDPServer) Stop() {
	f.cancel()

	sessions := make([]*clientSession, 0)
	f.clientMu.Lock()
	for _, session := range f.clients {
		session.isActive = false
		sessions = append(sessions, session)
	}
	f.clientMu.Unlock()

	for _, session := range sessions {
		session.mu.Lock()
		session.isRunning = false
		if session.cancel != nil {
			session.cancel()
		}
		session.ctx = nil
		session.cancel = nil
		session.lastCommand = nil
		session.mu.Unlock()
	}

	for _, conn := range f.conns {
		conn.Close()
	}
	f.conns = nil
	log.Printf("设备ID=%d [%s] 模块已停止", f.deviceId, f.title)
}

func (f *detectionUDPServer) Start() error {
	var wg conc.WaitGroup
	defer wg.Wait()

	// 只监听一个IP地址
	resolvedAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(f.detectionCfg.Host, strconv.Itoa(f.detectionCfg.Port)))
	if err != nil {
		log.Printf("设备ID=%d\thost=%s\tport=%d [%s] 解析地址失败: %v", f.deviceId, f.detectionCfg.Host, f.detectionCfg.Port, f.title, err)
		return fmt.Errorf("解析地址失败: %v", err)
	}

	conn, err := net.ListenUDP("udp", resolvedAddr)
	if err != nil {
		log.Printf("设备ID=%d\thost=%s\tport=%d [%s] 启动服务失败: %v", f.deviceId, f.detectionCfg.Host, f.detectionCfg.Port, f.title, err)
		return fmt.Errorf("启动UDP监听失败: %v", err)
	}

	f.conns = append(f.conns, conn)
	log.Printf("设备ID=%d\thost=%s\tport=%d [%s] 服务启动成功", f.deviceId, f.detectionCfg.Host, f.detectionCfg.Port, f.title)

	// 启动接收协程
	wg.Go(func() {
		f.handleUDPConnection(conn, f.detectionCfg.Host)
	})

	// 启动客户端清理协程
	wg.Go(func() {
		f.cleanupInactiveClients()
	})

	return nil
}

// handleUDPConnection 处理特定 UDP 连接的数据接收
func (f *detectionUDPServer) handleUDPConnection(conn *net.UDPConn, host string) {
	var wg conc.WaitGroup
	defer wg.Wait()

	buf := make([]byte, 1024)
	for {
		select {
		case <-f.ctx.Done():
			return
		default:
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				if f.ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
					return
				}
				log.Printf("[%s-%d] 从 %s 接收数据失败: %v", f.title, f.deviceId, host, err)
				continue
			}

			addrStr := addr.String()
			log.Printf("[%s-%d] 从 %s 接收到来自 %s 的数据: %s", f.title, f.deviceId, host, addrStr, string(buf[:n]))

			// 收到任意数据包都视为客户端活跃，避免首包解析失败时连接数一直为 0。
			session := f.getOrCreateSession(addr)
			f.touchSession(session)

			// 解析命令
			dataStr := utils.DetectionCommandString(buf[:n])
			command, err := dataStr.ParseCommand()
			if err != nil {
				log.Printf("[%s-%d-%s] 解析命令失败: %v", f.title, f.deviceId, addrStr, err)
				// 接收到后回复对方
				conn.WriteToUDP(buf[:n], addr)
				continue
			}

			ack := buildDetectorCommandAck(f.deviceId, strings.TrimSpace(string(dataStr)))
			if _, err := conn.WriteToUDP([]byte(ack), addr); err != nil {
				log.Printf("[%s-%d] 回复回执到 %s 失败: %v", f.title, f.deviceId, addrStr, err)
				continue
			}
			log.Printf("[%s-%d-%s] 已回复命令回执: %s", f.title, f.deviceId, addrStr, ack)

			// 使用会话锁防止并发操作
			session.mu.Lock()

			if command.Action == utils.ActionStop {
				if session.cancel != nil {
					session.cancel()
					log.Printf("[%s-%d-%s] 收到停止命令，停止任务", f.title, f.deviceId, addrStr)
				} else {
					log.Printf("[%s-%d-%s] 收到停止命令，当前没有运行中的任务", f.title, f.deviceId, addrStr)
				}
				session.ctx = nil
				session.cancel = nil
				session.isRunning = false
				session.lastCommand = nil
				session.mu.Unlock()
				continue
			}

			// 检查是否需要重启任务
			needRestart := false
			if !session.isRunning {
				// 首次启动
				needRestart = true
				log.Printf("[%s-%d-%s] 首次启动任务", f.title, f.deviceId, addrStr)
			} else if session.lastCommand != nil && command.TaskKey() != session.lastCommand.TaskKey() {
				// 命令变化，需要重启
				needRestart = true
				log.Printf("[%s-%d-%s] 命令改变 (%s -> %s)，重启任务", f.title, f.deviceId, addrStr, session.lastCommand.TaskKey(), command.TaskKey())
				if session.cancel != nil {
					session.cancel()
					time.Sleep(10 * time.Millisecond) // 等待旧任务退出
				}
			} else {
				// 相同命令，只更新时间，不重启
				log.Printf("[%s-%d-%s] 相同命令，保持运行", f.title, f.deviceId, addrStr)
				session.mu.Unlock()
				continue
			}

			// 启动或重启任务
			if needRestart {
				taskCtx, taskCancel := context.WithCancel(f.ctx)
				session.ctx = taskCtx
				session.cancel = taskCancel
				session.isRunning = true
				session.lastCommand = command

				f.clientMu.RLock()
				clientCount := len(f.clients)
				f.clientMu.RUnlock()
				log.Printf("[%s-%d-%s] 启动任务 goroutine，当前客户端总数: %d", f.title, f.deviceId, addrStr, clientCount)

				// 启动心跳和命令处理 goroutine
				wg.Go(func() {
					f.handleHeartbeat(taskCtx, session, addr, conn)
					log.Printf("[%s-%d-%s] 心跳 goroutine 退出", f.title, f.deviceId, addrStr)
				})

				wg.Go(func() {
					f.handleCommand(taskCtx, session, command, addr, conn)
					log.Printf("[%s-%d-%s] 命令处理 goroutine 退出", f.title, f.deviceId, addrStr)
				})
			}

			session.mu.Unlock()
		}
	}
}

// getOrCreateSession 获取或创建客户端会话
func (f *detectionUDPServer) getOrCreateSession(addr *net.UDPAddr) *clientSession {
	addrStr := addr.String()

	f.clientMu.Lock()
	defer f.clientMu.Unlock()

	session, exists := f.clients[addrStr]
	if !exists {
		session = &clientSession{
			addr:     addr,
			lastSeen: time.Now(),
			isActive: true,
		}
		f.clients[addrStr] = session
		log.Printf("[%s-%d] 新客户端连接: %s, 当前客户端数: %d", f.title, f.deviceId, addrStr, len(f.clients))
	}

	return session
}

// cleanupInactiveClients 清理不活跃的客户端
func (f *detectionUDPServer) cleanupInactiveClients() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			sessionsToCancel := make([]*clientSession, 0)

			f.clientMu.Lock()
			now := time.Now()
			for addrStr, session := range f.clients {
				// 如果客户端超过60秒没有活动，则清理
				if now.Sub(session.lastSeen) > 60*time.Second {
					session.isActive = false
					delete(f.clients, addrStr)
					sessionsToCancel = append(sessionsToCancel, session)
					log.Printf("[%s-%d] 清理不活跃客户端: %s, 剩余客户端数: %d", f.title, f.deviceId, addrStr, len(f.clients))
				}
			}
			f.clientMu.Unlock()

			for _, session := range sessionsToCancel {
				session.mu.Lock()
				session.isRunning = false
				if session.cancel != nil {
					session.cancel()
				}
				session.ctx = nil
				session.cancel = nil
				session.lastCommand = nil
				session.mu.Unlock()
			}
		}
	}
}

// handleHeartbeat 处理心跳
func (f *detectionUDPServer) handleHeartbeat(ctx context.Context, session *clientSession, addr *net.UDPAddr, conn *net.UDPConn) {
	addrStr := addr.String()
	for {
		timer := time.NewTimer(time.Duration(f.detectionCfg.HeartbeatInterval) * time.Second)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
			data := f.mock.GenerateDirectionHeartbeatData(f.deviceId)
			if _, err := conn.WriteToUDP([]byte(data.String()), addr); err != nil {
				log.Printf("[%s-%d-%s] 发送心跳数据失败: %v", f.title, f.deviceId, addrStr, err)
				continue
			}
			f.touchSession(session)
		}
	}
}

// 处理命令
func (f *detectionUDPServer) handleCommand(ctx context.Context, session *clientSession, command *utils.DetectionCommand, addr *net.UDPAddr, conn *net.UDPConn) {
	addrStr := addr.String()
	minSpeed := f.config.MinPushSpeed
	maxSpeed := f.config.MaxPushSpeed
	for {
		randomInterval := nextDetectionPushInterval(command, minSpeed, maxSpeed)

		timer := time.NewTimer(time.Duration(randomInterval) * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
			if command.IsSpectrumAnalysis() {
				data := f.mock.GenerateSpectrumFrame(command)
				if _, err := conn.WriteToUDP(data, addr); err != nil {
					log.Printf("[%s-%d-%s] 发送频谱分析数据失败: %v", f.title, f.deviceId, addrStr, err)
					continue
				}
				log.Printf("[%s-%d-%s] 发送频谱分析数据: len=%d band=%d-%dMHz", f.title, f.deviceId, addrStr, len(data), command.BandStart, command.BandStop)
				f.touchSession(session)
				continue
			}

			data := f.mock.GenerateDetectionData(f.deviceId, command)
			if _, err := conn.WriteToUDP([]byte(data.String()), addr); err != nil {
				log.Printf("[%s-%d-%s] 发送侦测数据失败: %v", f.title, f.deviceId, addrStr, err)
				continue
			}
			log.Printf("[%s-%d-%s] 发送侦测数据: %s", f.title, f.deviceId, addrStr, data.String())
			f.touchSession(session)
		}
	}
}

func nextDetectionPushInterval(command *utils.DetectionCommand, minSpeed int, maxSpeed int) int {
	if command != nil && command.IsSpectrumAnalysis() {
		// Real detector FFT packets are usually around 38-45ms apart, with
		// occasional longer gaps around 70-90ms.
		if rand.Intn(10) < 2 {
			return 70 + rand.Intn(21)
		}
		return 38 + rand.Intn(8)
	}

	if maxSpeed < minSpeed {
		maxSpeed = minSpeed
	}
	return minSpeed + rand.Intn(maxSpeed-minSpeed+1)
}
