package engine

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/mocks"
	"virtual-device-ui/internal/modules"
)

// ModuleStatus 单个模块的连接状态
type ModuleStatus struct {
	Name            string   `json:"name"`
	Connected       bool     `json:"connected"`
	ConnectionCount int      `json:"connectionCount"`
	SentCount       int64    `json:"sentCount"`
	LastActivityAt  string   `json:"lastActivityAt,omitempty"`
	ClientAddresses []string `json:"clientAddresses,omitempty"`
}

// Status 整体引擎状态
type Status struct {
	Running        bool                            `json:"running"`
	Modules        []ModuleStatus                  `json:"modules"`
	DroneCount     int                             `json:"droneCount"`
	DirectedStrike *modules.DirectedStrikeSnapshot `json:"directedStrike,omitempty"`
}

// Engine 统一管理所有模拟模块的生命周期
type Engine struct {
	mu      sync.Mutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	analysis       modules.AnalysisModule
	fpv            modules.FPVModule
	detections     []modules.DetectionUDPServerModule
	jamming        modules.JammingModule
	directedStrike modules.DirectedStrikeModule

	analysisMock   mocks.MockDataGenerator
	detectionMocks []mocks.MockDataGenerator
}

func New() *Engine {
	return &Engine{}
}

// Start 使用给定配置启动所有模块
func (e *Engine) Start(cfg *config.Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	config.NormalizeConfig(cfg)
	config.SetGlobalConfig(cfg)
	e.analysisMock = nil
	e.detectionMocks = nil
	e.analysis = nil
	e.fpv = nil
	e.detections = nil
	e.jamming = nil
	e.directedStrike = nil

	if cfg.Analysis.Enabled {
		log.Printf("[Engine] 创建解析模块，设备ID=%d", cfg.Analysis.DeviceID)
		e.analysisMock = mocks.NewMockDataGenerator(cfg.Analysis.DroneCount)
		e.analysis = modules.NewAnalysisModule(cfg.Analysis.DeviceID, e.analysisMock)
	} else {
		log.Printf("[Engine] 解析模块未启用")
	}
	for i := range cfg.Detections {
		detectionCfg := cfg.Detections[i]
		if !detectionCfg.Enabled {
			log.Printf("[Engine] 侦测模块[%d]未启用", i)
			continue
		}
		log.Printf("[Engine] 创建侦测模块[%d]，设备ID=%d host=%s port=%d", i, detectionCfg.DeviceID, detectionCfg.Host, detectionCfg.Port)
		detectionMock := mocks.NewMockDataGenerator(detectionCfg.DroneCount)
		e.detectionMocks = append(e.detectionMocks, detectionMock)
		e.detections = append(e.detections, modules.NewDetectionUDPModule(detectionCfg, detectionMock))
	}
	if cfg.FPV.Enabled {
		log.Printf("[Engine] 创建FPV模块，设备ID=%d", cfg.FPV.DeviceID)
		fpvMock := e.analysisMock
		if fpvMock == nil && len(e.detectionMocks) > 0 {
			fpvMock = e.detectionMocks[0]
		}
		if fpvMock == nil {
			fpvMock = mocks.NewMockDataGenerator(0)
		}
		e.fpv = modules.NewFPVModule(cfg.FPV.DeviceID, fpvMock)
	}
	if cfg.Jamming.Enabled {
		log.Printf("[Engine] 创建干扰打击模块，设备ID=%d", cfg.Jamming.DeviceID)
		e.jamming = modules.NewJammingModule(cfg.Jamming.DeviceID)
	}
	if cfg.DirectedStrike.Enabled {
		listenAddress := net.JoinHostPort(cfg.DirectedStrike.Host, fmt.Sprintf("%d", cfg.DirectedStrike.Port))
		log.Printf("[Engine] 创建定向打击模拟器，监听地址=%s", listenAddress)
		directedStrike, err := modules.NewDirectedStrikeModule(
			listenAddress,
			time.Duration(cfg.DirectedStrike.ResponseDelayMS)*time.Millisecond,
		)
		if err != nil {
			return fmt.Errorf("启动定向打击模拟器: %w", err)
		}
		e.directedStrike = directedStrike
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	e.cancel = cancel
	e.running = true

	if e.analysis != nil {
		e.wg.Add(1)
		go func() { defer e.wg.Done(); e.analysis.Start() }()
	}
	if e.fpv != nil {
		e.wg.Add(1)
		go func() { defer e.wg.Done(); e.fpv.Start() }()
	}
	for i, detection := range e.detections {
		e.wg.Add(1)
		go func(index int, module modules.DetectionUDPServerModule) {
			defer e.wg.Done()
			if err := module.Start(); err != nil {
				log.Printf("[侦测模块%d] 启动失败: %v", index, err)
			}
		}(i, detection)
	}
	if e.jamming != nil {
		e.wg.Add(1)
		go func() { defer e.wg.Done(); e.jamming.Start() }()
	}
	if e.directedStrike != nil {
		e.wg.Add(1)
		go func() { defer e.wg.Done(); e.directedStrike.Start() }()
	}

	if cfg.RandomDroneRefreshInterval > 0 {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			ticker := time.NewTicker(time.Duration(cfg.RandomDroneRefreshInterval) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if e.analysisMock != nil {
						e.analysisMock.RefreshRandomDrones()
					}
					for _, detectionMock := range e.detectionMocks {
						detectionMock.RefreshRandomDrones()
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	log.Printf("[Engine] 所有模块已启动")
	return nil
}

// Stop 停止所有模块
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	e.cancel()
	if e.analysis != nil {
		e.analysis.Stop()
	}
	if e.fpv != nil {
		e.fpv.Stop()
	}
	for _, detection := range e.detections {
		detection.Stop()
	}
	if e.jamming != nil {
		e.jamming.Stop()
	}
	if e.directedStrike != nil {
		e.directedStrike.Stop()
	}
	e.running = false

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Printf("[Engine] 等待模块停止超时")
	}
	log.Printf("[Engine] 所有模块已停止")
}

// IsRunning 返回引擎是否运行中
func (e *Engine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// GetStatus 返回引擎及各模块的实时状态
func (e *Engine) GetStatus() Status {
	e.mu.Lock()
	running := e.running
	analysis := e.analysis
	fpv := e.fpv
	detections := append([]modules.DetectionUDPServerModule(nil), e.detections...)
	jamming := e.jamming
	directedStrike := e.directedStrike
	analysisMock := e.analysisMock
	detectionMocks := append([]mocks.MockDataGenerator(nil), e.detectionMocks...)
	e.mu.Unlock()

	if !running {
		return Status{
			Running: false,
			Modules: []ModuleStatus{
				{Name: "analysis"},
				{Name: "detection"},
				{Name: "fpv"},
				{Name: "jamming"},
				{Name: "directed_strike"},
			},
		}
	}

	droneCount := 0
	if analysisMock != nil {
		droneCount += analysisMock.TotalDroneCount()
	}
	for _, detectionMock := range detectionMocks {
		droneCount += detectionMock.TotalDroneCount()
	}

	analysisCount := 0
	if analysis != nil {
		analysisCount = analysis.ConnectedCount()
	}
	fpvCount := 0
	if fpv != nil {
		fpvCount = fpv.ConnectedCount()
	}
	detectionCount := 0
	detectionLastActivity := ""
	var detectionLastActivityAt time.Time
	var detectionClients []string
	for _, detection := range detections {
		snapshot := detection.Snapshot()
		detectionClients = append(detectionClients, snapshot.ClientAddresses...)
		if !snapshot.LastActivity.IsZero() && snapshot.LastActivity.After(detectionLastActivityAt) {
			detectionLastActivityAt = snapshot.LastActivity
		}
	}
	detectionCount = len(detectionClients)
	if !detectionLastActivityAt.IsZero() {
		detectionLastActivity = detectionLastActivityAt.Format(time.RFC3339)
	}
	jammingCount := 0
	if jamming != nil {
		jammingCount = jamming.ConnectedCount()
	}
	var directedStrikeSnapshot *modules.DirectedStrikeSnapshot
	directedStrikeStatus := ModuleStatus{Name: "directed_strike"}
	if directedStrike != nil {
		snapshot := directedStrike.Snapshot()
		directedStrikeSnapshot = &snapshot
		directedStrikeStatus.Connected = snapshot.ActiveConnections > 0
		directedStrikeStatus.ConnectionCount = snapshot.ActiveConnections
		directedStrikeStatus.SentCount = int64(snapshot.SentFrames)
		directedStrikeStatus.LastActivityAt = snapshot.LastActivityAt
		directedStrikeStatus.ClientAddresses = append([]string(nil), snapshot.ClientAddresses...)
	}

	return Status{
		Running:        true,
		DroneCount:     droneCount,
		DirectedStrike: directedStrikeSnapshot,
		Modules: []ModuleStatus{
			{Name: "analysis", Connected: analysisCount > 0, ConnectionCount: analysisCount},
			{Name: "detection", Connected: detectionCount > 0, ConnectionCount: detectionCount, LastActivityAt: detectionLastActivity, ClientAddresses: detectionClients},
			{Name: "fpv", Connected: fpvCount > 0, ConnectionCount: fpvCount},
			{Name: "jamming", Connected: jammingCount > 0, ConnectionCount: jammingCount},
			directedStrikeStatus,
		},
	}
}
