package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type GPS struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// PredefinedDrone 预定义无人机配置
type PredefinedDrone struct {
	Serial   string  `json:"serial"`    // 无人机序列号
	Model    string  `json:"model"`     // 无人机型号
	Freq     float64 `json:"freq"`      // 频率
	RSSI     int     `json:"rssi"`      // 信号强度
	DroneGPS GPS     `json:"drone_gps"` // 无人机GPS坐标
	PilotGPS GPS     `json:"pilot_gps"` // 飞手GPS坐标
	Type     string  `json:"type"`      // 类型: "DID" 或 "RID" 或 "AUTO"(自动)
}

// AnalysisConfigPushType 分析配置推送类型
type AnalysisConfigPushType string

// BaseConfig 基础配置（用于FPV和Jamming模块）
type BaseConfig struct {
	Enabled  bool     `json:"enabled"`  // 是否启用模块
	DeviceID int      `json:"deviceID"` // 设备ID
	Hosts    []string `json:"hosts"`    // 多个服务器地址
	Port     int      `json:"port"`     // 端口
}

// AnalysisConfig 解析模块配置（带空包概率）
type AnalysisConfig struct {
	Enabled                bool     `json:"enabled"`                  // 是否启用模块
	DeviceID               int      `json:"deviceID"`                 // 设备ID
	DroneCount             int      `json:"drone_count"`              // 解析模块无人机总数
	Hosts                  []string `json:"hosts"`                    // 多个服务器地址
	Port                   int      `json:"port"`                     // 端口
	EmptyPacketProbability int      `json:"empty_packet_probability"` // 空数据包出现概率（0-100）
	O3PlusO4DataFile       *string  `json:"o3_plus_o4_data_file"`     // O3+/O4预定义数据文件路径（可选，如果配置则按顺序发送）
}

// DetectionConfig 侦测模块配置
type DetectionConfig struct {
	Enabled           bool   `json:"enabled"`            // 是否启用模块
	DeviceID          int    `json:"deviceID"`           // 设备ID
	DroneCount        int    `json:"drone_count"`        // 侦测模块无人机总数
	Host              string `json:"host"`               // 服务器地址（单个）
	Port              int    `json:"port"`               // 端口
	HeartbeatInterval int    `json:"heartbeat_interval"` // 心跳间隔，单位秒
}

// DirectedStrikeConfig configures the simulated directed-strike TCP device.
type DirectedStrikeConfig struct {
	Enabled         bool   `json:"enabled"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	ResponseDelayMS int    `json:"response_delay_ms"`
}

func (c *BaseConfig) UnmarshalJSON(data []byte) error {
	type rawBaseConfig struct {
		Enabled  *bool    `json:"enabled"`
		DeviceID int      `json:"deviceID"`
		Hosts    []string `json:"hosts"`
		Port     int      `json:"port"`
	}

	var raw rawBaseConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	c.Enabled = true
	if raw.Enabled != nil {
		c.Enabled = *raw.Enabled
	}
	c.DeviceID = raw.DeviceID
	c.Hosts = raw.Hosts
	c.Port = raw.Port
	return nil
}

func (c *AnalysisConfig) UnmarshalJSON(data []byte) error {
	type rawAnalysisConfig struct {
		Enabled                *bool    `json:"enabled"`
		DeviceID               int      `json:"deviceID"`
		DroneCount             int      `json:"drone_count"`
		Hosts                  []string `json:"hosts"`
		Port                   int      `json:"port"`
		EmptyPacketProbability int      `json:"empty_packet_probability"`
		O3PlusO4DataFile       *string  `json:"o3_plus_o4_data_file"`
	}

	var raw rawAnalysisConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	c.Enabled = true
	if raw.Enabled != nil {
		c.Enabled = *raw.Enabled
	}
	c.DeviceID = raw.DeviceID
	c.DroneCount = raw.DroneCount
	c.Hosts = raw.Hosts
	c.Port = raw.Port
	c.EmptyPacketProbability = raw.EmptyPacketProbability
	c.O3PlusO4DataFile = raw.O3PlusO4DataFile
	return nil
}

func (c *DetectionConfig) UnmarshalJSON(data []byte) error {
	type rawDetectionConfig struct {
		Enabled           *bool  `json:"enabled"`
		DeviceID          int    `json:"deviceID"`
		DroneCount        int    `json:"drone_count"`
		Host              string `json:"host"`
		Port              int    `json:"port"`
		HeartbeatInterval int    `json:"heartbeat_interval"`
	}

	var raw rawDetectionConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	c.Enabled = true
	if raw.Enabled != nil {
		c.Enabled = *raw.Enabled
	}
	c.DeviceID = raw.DeviceID
	c.DroneCount = raw.DroneCount
	c.Host = raw.Host
	c.Port = raw.Port
	c.HeartbeatInterval = raw.HeartbeatInterval
	return nil
}

// JammingConfig 干扰模块配置
// type JammingConfig struct {
// 	// DeviceID int `json:"deviceID"` // 设备ID
// 	BaseConfig
// }

// Config 配置结构体
type Config struct {
	MinPushSpeed               int                  `json:"min_push_speed"`                 // 全局最小推送间隔（毫秒）
	MaxPushSpeed               int                  `json:"max_push_speed"`                 // 全局最大推送间隔（毫秒）
	PredefinedDrones           []PredefinedDrone    `json:"predefined_drones"`              // 预定义的无人机列表（与模块内drone_count可同时使用）
	RandomDroneRefreshInterval int                  `json:"random_drone_refresh_interval"`  // 随机无人机刷新间隔（秒），0表示不刷新
	MaxDirectionChange         float64              `json:"max_direction_change"`           // 最大方向变化，单位度
	MaxDistanceFromCenterPoint float64              `json:"max_distance_from_center_point"` // 最大距离中心点距离，单位米
	CenterPoint                GPS                  `json:"center_point"`                   // 中心点
	Analysis                   AnalysisConfig       `json:"analysis"`                       // 解析模块配置
	Detection                  DetectionConfig      `json:"detection"`                      // 侦测模块配置
	Detections                 []DetectionConfig    `json:"detections"`                     // 多侦测模块配置
	FPV                        BaseConfig           `json:"fpv"`                            // FPV模块配置
	Jamming                    BaseConfig           `json:"jamming"`                        // 干扰模块配置
	DirectedStrike             DirectedStrikeConfig `json:"directed_strike"`                // 定向打击设备模拟器配置
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func normalizeHosts(hosts []string) []string {
	if len(hosts) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if trimmed := strings.TrimSpace(host); trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}

	if len(normalized) == 0 {
		return nil
	}

	return normalized
}

func normalizeDroneType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "DID":
		return "DID"
	case "RID":
		return "RID"
	default:
		return "AUTO"
	}
}

func normalizeDetectionConfig(cfg *DetectionConfig) {
	cfg.Host = strings.TrimSpace(cfg.Host)
}

// NormalizeConfig trims user-provided values into a stable in-memory form.
func NormalizeConfig(cfg *Config) {
	if cfg == nil {
		return
	}

	cfg.Analysis.Hosts = normalizeHosts(cfg.Analysis.Hosts)
	normalizeDetectionConfig(&cfg.Detection)
	cfg.FPV.Hosts = normalizeHosts(cfg.FPV.Hosts)
	cfg.Jamming.Hosts = normalizeHosts(cfg.Jamming.Hosts)
	cfg.DirectedStrike.Host = strings.TrimSpace(cfg.DirectedStrike.Host)
	cfg.Analysis.O3PlusO4DataFile = normalizeOptionalString(cfg.Analysis.O3PlusO4DataFile)

	if len(cfg.Detections) == 0 {
		cfg.Detections = []DetectionConfig{cfg.Detection}
	} else {
		for i := range cfg.Detections {
			normalizeDetectionConfig(&cfg.Detections[i])
		}
		cfg.Detection = cfg.Detections[0]
	}

	for i := range cfg.PredefinedDrones {
		cfg.PredefinedDrones[i].Serial = strings.TrimSpace(cfg.PredefinedDrones[i].Serial)
		cfg.PredefinedDrones[i].Model = strings.TrimSpace(cfg.PredefinedDrones[i].Model)
		cfg.PredefinedDrones[i].Type = normalizeDroneType(cfg.PredefinedDrones[i].Type)
	}
}

// ValidateConfig validates a config after first normalizing optional values.
func ValidateConfig(cfg *Config) error {
	NormalizeConfig(cfg)
	return validateConfig(cfg)
}

func (c *Config) UnmarshalJSON(data []byte) error {
	type rawConfig struct {
		MinPushSpeed               int                   `json:"min_push_speed"`
		MaxPushSpeed               int                   `json:"max_push_speed"`
		LegacyDroneCount           *int                  `json:"drone_count"`
		PredefinedDrones           []PredefinedDrone     `json:"predefined_drones"`
		RandomDroneRefreshInterval int                   `json:"random_drone_refresh_interval"`
		MaxDirectionChange         float64               `json:"max_direction_change"`
		MaxDistanceFromCenterPoint float64               `json:"max_distance_from_center_point"`
		CenterPoint                GPS                   `json:"center_point"`
		Analysis                   json.RawMessage       `json:"analysis"`
		Detection                  json.RawMessage       `json:"detection"`
		Detections                 []json.RawMessage     `json:"detections"`
		FPV                        BaseConfig            `json:"fpv"`
		Jamming                    BaseConfig            `json:"jamming"`
		DirectedStrike             *DirectedStrikeConfig `json:"directed_strike"`
	}

	type moduleCount struct {
		DroneCount *int `json:"drone_count"`
	}

	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	c.MinPushSpeed = raw.MinPushSpeed
	c.MaxPushSpeed = raw.MaxPushSpeed
	c.PredefinedDrones = raw.PredefinedDrones
	c.RandomDroneRefreshInterval = raw.RandomDroneRefreshInterval
	c.MaxDirectionChange = raw.MaxDirectionChange
	c.MaxDistanceFromCenterPoint = raw.MaxDistanceFromCenterPoint
	c.CenterPoint = raw.CenterPoint
	c.FPV = raw.FPV
	c.Jamming = raw.Jamming
	if raw.DirectedStrike != nil {
		c.DirectedStrike = *raw.DirectedStrike
	} else {
		c.DirectedStrike = defaultDirectedStrikeConfig(false)
	}

	if len(raw.Analysis) > 0 {
		if err := json.Unmarshal(raw.Analysis, &c.Analysis); err != nil {
			return err
		}
	}

	if len(raw.Detection) > 0 {
		if err := json.Unmarshal(raw.Detection, &c.Detection); err != nil {
			return err
		}
	}

	if len(raw.Detections) > 0 {
		c.Detections = make([]DetectionConfig, 0, len(raw.Detections))
		for _, rawDetection := range raw.Detections {
			var detection DetectionConfig
			if err := json.Unmarshal(rawDetection, &detection); err != nil {
				return err
			}
			c.Detections = append(c.Detections, detection)
		}
	} else if len(raw.Detection) > 0 {
		c.Detections = []DetectionConfig{c.Detection}
	}

	if raw.LegacyDroneCount != nil {
		legacyTotal := *raw.LegacyDroneCount + len(raw.PredefinedDrones)

		var analysisCount moduleCount
		if len(raw.Analysis) > 0 && json.Unmarshal(raw.Analysis, &analysisCount) == nil && analysisCount.DroneCount == nil {
			c.Analysis.DroneCount = legacyTotal
		}

		var detectionCount moduleCount
		if len(raw.Detection) > 0 && json.Unmarshal(raw.Detection, &detectionCount) == nil && detectionCount.DroneCount == nil {
			c.Detection.DroneCount = legacyTotal
		}
		for i, rawDetection := range raw.Detections {
			var detectionCount moduleCount
			if json.Unmarshal(rawDetection, &detectionCount) == nil && detectionCount.DroneCount == nil {
				c.Detections[i].DroneCount = legacyTotal
			}
		}
	}

	if len(c.Detections) > 0 {
		c.Detection = c.Detections[0]
	}

	return nil
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	cfg := &Config{
		RandomDroneRefreshInterval: 1800,
		PredefinedDrones:           []PredefinedDrone{
			// {
			// 	Serial:   "Tm11AGd802DpIv",
			// 	Model:    "Autel_type2",
			// 	Freq:     708.64,
			// 	RSSI:     -76,
			// 	DroneGPS: GPS{Lat: 31.255322, Lng: 121.435805},
			// 	PilotGPS: GPS{Lat: 31.17249, Lng: 121.509564},
			// 	Type:     "RID",
			// },
			// {
			// 	Serial:   "Gl92rjO756XUcG",
			// 	Model:    "Lightbridge type2",
			// 	Freq:     1581.84,
			// 	RSSI:     -74,
			// 	DroneGPS: GPS{Lat: 31.178708, Lng: 121.465696},
			// 	PilotGPS: GPS{Lat: 31.220948, Lng: 121.480806},
			// 	Type:     "RID",
			// },
			// {
			// 	Serial:   "Ys39YBI809RrmD",
			// 	Model:    "PAL Analog",
			// 	Freq:     1732.28,
			// 	RSSI:     -76,
			// 	DroneGPS: GPS{Lat: 31.295621, Lng: 121.401178},
			// 	PilotGPS: GPS{Lat: 31.22873, Lng: 121.408957},
			// 	Type:     "RID",
			// },
			// {
			// 	Serial:   "1581F6A9B234C005DA2E",
			// 	Model:    "DJI Mini 3 Pro",
			// 	Freq:     2437.0,
			// 	RSSI:     -72,
			// 	DroneGPS: GPS{Lat: 31.2320, Lng: 121.4750},
			// 	PilotGPS: GPS{Lat: 31.2315, Lng: 121.4745},
			// 	Type:     "DID",
			// },
			// {
			// 	Serial:   "1581F7Z8C456D789EB3F",
			// 	Model:    "DJI_O3+",
			// 	Freq:     5800.0,
			// 	RSSI:     -78,
			// 	DroneGPS: GPS{Lat: 31.2380, Lng: 121.4820},
			// 	PilotGPS: GPS{Lat: 31.2375, Lng: 121.4815},
			// 	Type:     "DID",
			// },
			// {
			// 	Serial:   "WALK001XY",
			// 	Model:    "Walksnail",
			// 	Freq:     5658.0,
			// 	RSSI:     -68,
			// 	DroneGPS: GPS{Lat: 31.2260, Lng: 121.4700},
			// 	PilotGPS: GPS{Lat: 31.2255, Lng: 121.4695},
			// 	Type:     "AUTO",
			// },
			// {
			// 	Serial:   "1581F8M9N678P012QC4G",
			// 	Model:    "DJI_O4",
			// 	Freq:     5950.0,
			// 	RSSI:     -71,
			// 	DroneGPS: GPS{Lat: 31.2340, Lng: 121.4780},
			// 	PilotGPS: GPS{Lat: 31.2335, Lng: 121.4775},
			// 	Type:     "AUTO",
			// },
		},
		MaxDirectionChange:         120.0,
		MaxDistanceFromCenterPoint: 100.0,
		CenterPoint: GPS{
			Lat: 28.2378405,
			Lng: 117.1143221,
		},
		MinPushSpeed: 300,
		MaxPushSpeed: 1000,
		Analysis: AnalysisConfig{
			Enabled:                true,
			DeviceID:               2001,
			DroneCount:             3,
			Hosts:                  []string{"127.0.0.1"},
			Port:                   10002,
			EmptyPacketProbability: 10,
		},
		Detection: DetectionConfig{
			Enabled:           true,
			DeviceID:          2000,
			DroneCount:        3,
			Host:              "0.0.0.0",
			Port:              9024,
			HeartbeatInterval: 5,
		},
		Detections: []DetectionConfig{
			{
				Enabled:           true,
				DeviceID:          2000,
				DroneCount:        3,
				Host:              "0.0.0.0",
				Port:              9024,
				HeartbeatInterval: 5,
			},
			{
				Enabled:           true,
				DeviceID:          2001,
				DroneCount:        3,
				Host:              "0.0.0.0",
				Port:              9026,
				HeartbeatInterval: 5,
			},
			{
				Enabled:           true,
				DeviceID:          2002,
				DroneCount:        3,
				Host:              "0.0.0.0",
				Port:              9028,
				HeartbeatInterval: 5,
			},
		},
		FPV: BaseConfig{
			Enabled:  true,
			DeviceID: 2000,
			Hosts:    []string{"127.0.0.1"},
			Port:     10000,
		},
		Jamming: BaseConfig{
			Enabled:  true,
			DeviceID: 2345,
			Hosts:    []string{"127.0.0.1"},
			Port:     10003,
		},
		DirectedStrike: defaultDirectedStrikeConfig(true),
	}
	NormalizeConfig(cfg)
	return cfg
}

func defaultDirectedStrikeConfig(enabled bool) DirectedStrikeConfig {
	return DirectedStrikeConfig{
		Enabled:         enabled,
		Host:            "0.0.0.0",
		Port:            19000,
		ResponseDelayMS: 0,
	}
}

var globalConfig *Config

// SetGlobalConfig 直接设置全局配置（供外部调用，如 Wails UI）
func SetGlobalConfig(cfg *Config) {
	globalConfig = cfg
}

// LoadConfig 从文件加载配置
func LoadConfig(configPath string) (*Config, error) {
	// 如果配置文件不存在，创建默认配置文件
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("配置文件不存在，创建默认配置文件: %s\n", configPath)
		if err := createDefaultConfigFile(configPath); err != nil {
			return nil, fmt.Errorf("创建默认配置文件失败: %v", err)
		}
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	config := &Config{}
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 验证配置
	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %v", err)
	}

	globalConfig = config
	return config, nil
}

// GetConfig 获取全局配置
func GetConfig() *Config {
	if globalConfig == nil {
		return DefaultConfig()
	}
	return globalConfig
}

// SaveConfig 保存配置到文件
func SaveConfig(config *Config, configPath string) error {
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("配置验证失败: %v", err)
	}

	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	globalConfig = config
	return nil
}

// createDefaultConfigFile 创建默认配置文件
func createDefaultConfigFile(configPath string) error {
	defaultConfig := DefaultConfig()
	return SaveConfig(defaultConfig, configPath)
}

// validateConfig 验证配置
func validateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	// 验证刷新间隔
	if config.RandomDroneRefreshInterval < 0 {
		return fmt.Errorf("随机无人机刷新间隔不能为负数")
	}

	// 验证最大方向变化
	if config.MaxDirectionChange < 0 || config.MaxDirectionChange > 360 {
		return fmt.Errorf("最大方向变化必须在0到360度之间")
	}

	// 验证最大距离中心点距离
	if config.MaxDistanceFromCenterPoint <= 0 {
		return fmt.Errorf("最大距离中心点距离必须大于0")
	}

	// 验证中心点坐标
	if config.CenterPoint.Lat < -90 || config.CenterPoint.Lat > 90 {
		return fmt.Errorf("中心点纬度必须在-90到90之间")
	}
	if config.CenterPoint.Lng < -180 || config.CenterPoint.Lng > 180 {
		return fmt.Errorf("中心点经度必须在-180到180之间")
	}

	// 验证推送速度
	if config.MinPushSpeed <= 0 {
		return fmt.Errorf("最小推送速度必须大于0")
	}
	if config.MaxPushSpeed <= 0 {
		return fmt.Errorf("最大推送速度必须大于0")
	}
	if config.MinPushSpeed > config.MaxPushSpeed {
		return fmt.Errorf("最小推送速度不能大于最大推送速度")
	}

	// 验证Analysis配置
	if err := validateAnalysisConfig(&config.Analysis); err != nil {
		return fmt.Errorf("analysis配置错误: %v", err)
	}

	// 验证Detection配置
	seenDetectionAddrs := make(map[string]int)
	for i := range config.Detections {
		if err := validateDetectionConfig(&config.Detections[i]); err != nil {
			return fmt.Errorf("detection[%d]配置错误: %v", i, err)
		}
		if !config.Detections[i].Enabled {
			continue
		}
		addr := fmt.Sprintf("%s:%d", config.Detections[i].Host, config.Detections[i].Port)
		if first, ok := seenDetectionAddrs[addr]; ok {
			return fmt.Errorf("detection[%d]与detection[%d]监听地址重复: %s", i, first, addr)
		}
		seenDetectionAddrs[addr] = i
	}

	// 验证FPV配置
	if err := validateBaseConfig(&config.FPV, "FPV"); err != nil {
		return fmt.Errorf("FPV配置错误: %v", err)
	}

	// 验证Jamming配置
	if err := validateBaseConfig(&config.Jamming, "Jamming"); err != nil {
		return fmt.Errorf("jamming配置错误: %v", err)
	}

	if err := validateDirectedStrikeConfig(&config.DirectedStrike); err != nil {
		return fmt.Errorf("定向打击模拟器配置错误: %v", err)
	}

	return nil
}

func validateDirectedStrikeConfig(config *DirectedStrikeConfig) error {
	if !config.Enabled {
		return nil
	}
	if config.Host == "" {
		return fmt.Errorf("监听地址不能为空")
	}
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("监听端口必须在1到65535之间")
	}
	if config.ResponseDelayMS < 0 || config.ResponseDelayMS > 10000 {
		return fmt.Errorf("回执延迟必须在0到10000毫秒之间")
	}
	return nil
}

// validateBaseConfig 验证基础配置
func validateBaseConfig(config *BaseConfig, moduleName string) error {
	if !config.Enabled {
		return nil
	}

	if config.DeviceID <= 0 {
		return fmt.Errorf("%s模块设备ID必须大于0", moduleName)
	}

	if len(config.Hosts) == 0 {
		return fmt.Errorf("%s模块主机地址列表不能为空", moduleName)
	}

	for i, host := range config.Hosts {
		if host == "" {
			return fmt.Errorf("%s模块第%d个主机地址不能为空", moduleName, i+1)
		}
	}

	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("%s模块端口必须在1-65535范围内", moduleName)
	}

	return nil
}

// validateAnalysisConfig 验证分析模块配置
func validateAnalysisConfig(config *AnalysisConfig) error {
	if !config.Enabled {
		return nil
	}

	if config.DroneCount < 0 {
		return fmt.Errorf("Analysis模块无人机数量不能为负数")
	}

	if config.DeviceID <= 0 {
		return fmt.Errorf("Analysis模块设备ID必须大于0")
	}

	if len(config.Hosts) == 0 {
		return fmt.Errorf("Analysis模块至少需要配置一个主机地址")
	}

	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("Analysis模块端口必须在1-65535范围内")
	}

	if config.EmptyPacketProbability < 0 || config.EmptyPacketProbability > 100 {
		return fmt.Errorf("Analysis模块空包概率必须在0-100范围内")
	}

	return nil
}

// validateDetectionConfig 验证侦测模块配置
func validateDetectionConfig(config *DetectionConfig) error {
	if !config.Enabled {
		return nil
	}

	if config.DroneCount < 0 {
		return fmt.Errorf("Detection模块无人机数量不能为负数")
	}

	if config.DeviceID <= 0 {
		return fmt.Errorf("Detection模块设备ID必须大于0")
	}

	if config.Host == "" {
		return fmt.Errorf("Detection模块主机地址不能为空")
	}

	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("Detection模块端口必须在1-65535范围内")
	}

	if config.HeartbeatInterval <= 0 {
		return fmt.Errorf("心跳间隔必须大于0秒")
	}

	return nil
}
