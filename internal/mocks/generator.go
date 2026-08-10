package mocks

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/rand"
	"os"
	"slices"
	"strings"
	"sync"
	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/utils"

	"github.com/brianvoe/gofakeit/v7"
)

const ridSerialPrefix = "1581"

type MockDataGenerator interface {
	GenerateAnalysisData(deviceID int) fmt.Stringer
	GenerateDetectionData(deviceID int, command *utils.DetectionCommand) fmt.Stringer
	GenerateSpectrumFrame(command *utils.DetectionCommand) []byte
	GenerateDirectionHeartbeatData(deviceID int) fmt.Stringer
	GenerateFPVData() FPVData
	GenerateFPVWarningData() fmt.Stringer
	GenerateDeviceID() int
	RefreshRandomDrones() // 刷新随机无人机
	TotalDroneCount() int // 返回总无人机数量
}

// mockDataGenerator Mock数据生成器
type mockDataGenerator struct {
	faker *gofakeit.Faker

	mu              sync.RWMutex // 保护并发访问
	droneList       []*DroneInfo
	droneIndex      int
	predefinedCount int // 预定义无人机数量
	randomCount     int // 随机无人机数量

	droneDidList []string
	droneRidList []string

	deviceGPS GPS

	directionSeq int

	spectrumScanOffsets map[string]int

	batchCounter   int // 批次计数器
	batchSeqNumber int // 当前批次内的序号

	// O3+/O4 预定义数据相关
	o3PlusO4DataList  []*O3PlusO4PredefinedData // O3+/O4 预定义数据列表
	o3PlusO4DataIndex int                       // 当前读取索引
}

// O3PlusO4PredefinedData O3+/O4 预定义数据结构
type O3PlusO4PredefinedData struct {
	EncryptedID string  // 加密ID（如 43556db6）
	Model       string  // 型号（如 Mavic_O4）
	Freq        float64 // 频率
	RSSI        int     // 信号强度
	ByteData    string  // 字节数据（逗号分隔的十六进制）
}

// TotalDroneCount 返回当前模块的无人机总数
func (m *mockDataGenerator) TotalDroneCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.predefinedCount + m.randomCount
}

// NewMockDataGenerator 创建新的Mock数据生成器
func NewMockDataGenerator(droneCount int) MockDataGenerator {
	faker := gofakeit.New(uint64(rand.Int63()))
	cfg := config.GetConfig()

	// 模块内无人机总数固定为传入值；优先使用预定义无人机，不足部分再自动生成。
	totalCount := droneCount
	predefinedCount := min(len(cfg.PredefinedDrones), totalCount)
	randomCount := totalCount - predefinedCount

	if totalCount == 0 {
		log.Printf("警告: 未配置任何无人机，将创建空的Mock数据池")
	}

	log.Printf("初始化无人机列表: 模块总数=%d, 预定义使用=%d, 自动生成=%d", totalCount, predefinedCount, randomCount)

	m := &mockDataGenerator{
		faker:               faker,
		droneList:           make([]*DroneInfo, totalCount),
		droneIndex:          0,
		predefinedCount:     predefinedCount,
		randomCount:         randomCount,
		droneDidList:        make([]string, totalCount),
		droneRidList:        make([]string, totalCount),
		deviceGPS:           GPS{Lat: cfg.CenterPoint.Lat, Lng: cfg.CenterPoint.Lng},
		spectrumScanOffsets: make(map[string]int),
		batchCounter:        1, // 初始化批次从1开始
		batchSeqNumber:      0, // 序号从0开始
	}

	// 初始化无人机列表
	m.initializeDrones()

	// 加载 O3+/O4 预定义数据
	m.loadO3PlusO4Data()

	return m
}

// initializeDrones 初始化无人机列表
func (m *mockDataGenerator) initializeDrones() {
	cfg := config.GetConfig()
	totalCount := m.predefinedCount + m.randomCount

	for i := range totalCount {
		m.droneDidList[i] = ""
		m.droneRidList[i] = ""

		var model string
		var serial string
		var freq float64
		var rssi int
		var droneGPS GPS
		var pilotGPS GPS
		var droneType string

		// 前面的是预定义无人机，后面的是随机生成无人机
		if i < m.predefinedCount {
			// 使用配置文件中的预定义数据
			predefined := cfg.PredefinedDrones[i]
			serial = predefined.Serial
			model = predefined.Model
			freq = predefined.Freq
			rssi = predefined.RSSI
			droneGPS = GPS{Lat: predefined.DroneGPS.Lat, Lng: predefined.DroneGPS.Lng}
			pilotGPS = GPS{Lat: predefined.PilotGPS.Lat, Lng: predefined.PilotGPS.Lng}
			droneType = predefined.Type // 如果序列号为空，生成一个
			if serial == "" {
				serial = m.generateSerial()
			}
			// 如果型号为空，生成一个
			if model == "" {
				model = m.generateModel()
			}
			// 如果频率为0，生成一个
			if freq == 0 {
				freq = m.generateFreq()
			}
			// 如果RSSI为0，生成一个
			if rssi == 0 {
				rssi = m.generateRSSI()
			}
			// 如果飞手GPS为空，先生成飞手位置
			if pilotGPS.Lat == 0 && pilotGPS.Lng == 0 {
				pilotGPS = m.generatePilotGPS()
			}
			// 如果无人机GPS为空，基于飞手位置生成（初始距离2-5米，模拟刚起飞）
			if droneGPS.Lat == 0 && droneGPS.Lng == 0 {
				droneGPS = m.generateDroneGPSAroundPilot(pilotGPS, 0.002, 0.005)
			}
		} else {
			// 随机生成：先生成飞手，再基于飞手生成无人机
			model = m.generateModel()
			m.batchSeqNumber++ // 递增序号
			serial = m.generateSerialWithBatch(m.batchCounter, m.batchSeqNumber)
			freq = m.generateFreq()
			rssi = m.generateRSSI()
			// 先生成飞手位置（基于中心点）
			pilotGPS = m.generatePilotGPS()
			// 再基于飞手位置生成无人机位置（初始距离2-5米，模拟刚起飞）
			droneGPS = m.generateDroneGPSAroundPilot(pilotGPS, 0.002, 0.005)
			droneType = "AUTO"
		}

		// 如果是加密型号，生成固定的加密ID
		encryptedID := ""
		if m.isEncryptedModel(model) {
			encryptedID = fmt.Sprintf("%07x", m.faker.Uint32()%0x10000000) // 7位十六进制
		}

		m.droneList[i] = &DroneInfo{
			Serial:         serial,
			Model:          model,
			DetectionModel: m.generateDetectionModel(), // 为每个无人机分配固定的侦测型号
			EncryptedID:    encryptedID,                // 为加密型号分配固定的加密ID
			Freq:           freq,
			RSSI:           rssi,
			DroneGPS:       droneGPS,
			PilotGPS:       pilotGPS,
		}

		// 根据配置的类型或自动判断来设置DID/RID列表
		if i < m.predefinedCount && droneType != "AUTO" {
			// 预定义无人机：使用配置中指定的类型
			if droneType == "DID" {
				m.droneDidList[i] = serial
				log.Printf("[DID] 预定义无人机 %d: Serial=%s, Model=%s, Freq=%.2f, RSSI=%d, DroneGPS={%.6f,%.6f}, PilotGPS={%.6f,%.6f}",
					i+1, serial, model, freq, rssi, droneGPS.Lat, droneGPS.Lng, pilotGPS.Lat, pilotGPS.Lng)
			} else if droneType == "RID" {
				m.droneRidList[i] = serial
				log.Printf("[RID] 预定义无人机 %d: Serial=%s, Model=%s, Freq=%.2f, RSSI=%d, DroneGPS={%.6f,%.6f}, PilotGPS={%.6f,%.6f}",
					i+1, serial, model, freq, rssi, droneGPS.Lat, droneGPS.Lng, pilotGPS.Lat, pilotGPS.Lng)
			}
			continue
		}

		// 自动判断类型（预定义AUTO类型或随机生成）
		var prefix string
		if i < m.predefinedCount {
			prefix = "预定义"
		} else {
			prefix = "随机"
		}

		if m.isDJIModel(model) {
			droneType := "RID"
			if m.faker.IntN(2) == 0 {
				m.droneDidList[i] = serial
				droneType = "DID"
			} else {
				m.droneRidList[i] = serial
			}

			log.Printf("[%s] %s DJI无人机 %d: Serial=%s, Model=%s, Freq=%.2f, RSSI=%d, DroneGPS={%.6f,%.6f}, PilotGPS={%.6f,%.6f}",
				droneType,
				prefix,
				i+1,
				serial,
				model,
				freq,
				rssi,
				droneGPS.Lat,
				droneGPS.Lng,
				pilotGPS.Lat,
				pilotGPS.Lng,
			)
		} else {
			m.droneRidList[i] = serial

			log.Printf("[%s] %s 无人机 %d: Serial=%s, Model=%s, Freq=%.2f, RSSI=%d, DroneGPS={%.6f,%.6f}, PilotGPS={%.6f,%.6f}",
				"RID",
				prefix,
				i+1,
				serial,
				model,
				freq,
				rssi,
				droneGPS.Lat,
				droneGPS.Lng,
				pilotGPS.Lat,
				pilotGPS.Lng,
			)
		}
	}

	log.Printf("初始化Mock数据生成器完成: 总计=%d个无人机 (预定义=%d, 随机=%d)", totalCount, m.predefinedCount, m.randomCount)
}

// RefreshRandomDrones 刷新随机生成的无人机（保留预定义的无人机）
func (m *mockDataGenerator) RefreshRandomDrones() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.randomCount == 0 {
		log.Printf("没有随机无人机需要刷新")
		return
	}

	// 增加批次号，重置序号
	m.batchCounter++
	m.batchSeqNumber = 0

	log.Printf("====== 开始第 %d 批次刷新 ======", m.batchCounter)
	log.Printf("批次 %d: 将生成 %d 条随机无人机数据", m.batchCounter, m.randomCount)

	// 只刷新随机生成的无人机部分（从 predefinedCount 开始）
	for i := m.predefinedCount; i < len(m.droneList); i++ {
		m.droneDidList[i] = ""
		m.droneRidList[i] = ""

		model := m.generateModel()
		m.batchSeqNumber++ // 递增序号
		serial := m.generateSerialWithBatch(m.batchCounter, m.batchSeqNumber)
		freq := m.generateFreq()
		rssi := m.generateRSSI()

		// 刷新时保持飞手位置不变，只更新无人机位置
		// 如果之前有飞手坐标，继续使用；否则生成新的
		var pilotGPS GPS
		if m.droneList[i] != nil && (m.droneList[i].PilotGPS.Lat != 0 || m.droneList[i].PilotGPS.Lng != 0) {
			// 飞手基本不动，保持原位置
			pilotGPS = m.droneList[i].PilotGPS
		} else {
			// 首次刷新，生成飞手位置
			pilotGPS = m.generatePilotGPS()
		}

		// 基于飞手位置生成无人机位置（刷新时距离可以更远，0.5-5公里）
		droneGPS := m.generateDroneGPSAroundPilot(pilotGPS, 0.5, 5.0)

		// 保留原有的DetectionModel和EncryptedID
		detectionModel := m.droneList[i].DetectionModel
		if detectionModel == "" {
			detectionModel = m.generateDetectionModel()
		}
		encryptedID := m.droneList[i].EncryptedID
		if encryptedID == "" && m.isEncryptedModel(model) {
			encryptedID = fmt.Sprintf("%07x", m.faker.Uint32()%0x10000000)
		}

		m.droneList[i] = &DroneInfo{
			Serial:         serial,
			Model:          model,
			DetectionModel: detectionModel, // 保持侦测型号不变
			EncryptedID:    encryptedID,    // 保持加密ID不变
			Freq:           freq,
			RSSI:           rssi,
			DroneGPS:       droneGPS,
			PilotGPS:       pilotGPS,
		}

		// 自动判断类型
		if m.isDJIModel(model) {
			droneType := "RID"
			if m.faker.IntN(2) == 0 {
				m.droneDidList[i] = serial
				droneType = "DID"
			} else {
				m.droneRidList[i] = serial
			}

			log.Printf("[%s] 刷新 随机 DJI无人机 %d: Serial=%s, Model=%s, Freq=%.2f, RSSI=%d, DroneGPS={%.6f,%.6f}, PilotGPS={%.6f,%.6f}",
				droneType,
				i+1,
				serial,
				model,
				freq,
				rssi,
				droneGPS.Lat,
				droneGPS.Lng,
				pilotGPS.Lat,
				pilotGPS.Lng,
			)
		} else {
			m.droneRidList[i] = serial

			log.Printf("[%s] 刷新 随机 无人机 %d: Serial=%s, Model=%s, Freq=%.2f, RSSI=%d, DroneGPS={%.6f,%.6f}, PilotGPS={%.6f,%.6f}",
				"RID",
				i+1,
				serial,
				model,
				freq,
				rssi,
				droneGPS.Lat,
				droneGPS.Lng,
				pilotGPS.Lat,
				pilotGPS.Lng,
			)
		}
	}

	log.Printf("====== 批次 %d 刷新完成，共生成 %d 条数据 ======", m.batchCounter, m.randomCount)
	log.Printf("批次范围: Batch%06d-Seq%04d 到 Batch%06d-Seq%04d",
		m.batchCounter, 1, m.batchCounter, m.randomCount)
}

// shuffleDroneList 打乱无人机列表
func (m *mockDataGenerator) shuffleDroneList() {
	rand.Shuffle(len(m.droneList), func(i, j int) {
		m.droneList[i], m.droneList[j] = m.droneList[j], m.droneList[i]
	})
}

// getCurrentDroneInfo 获取当前无人机信息
func (m *mockDataGenerator) getCurrentDroneInfo() *DroneInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.droneList) == 0 {
		return nil
	}

	drone := m.droneList[m.droneIndex]
	m.droneIndex++

	// 当索引到达列表末尾时，打乱列表并重置索引
	if m.droneIndex >= len(m.droneList) {
		m.shuffleDroneList()
		m.droneIndex = 0
	}

	return drone
}

// generateSerial 生成类似真实的设备序列号（不带批次序号，用于预定义无人机）
func (m *mockDataGenerator) generateSerial() string {
	serial := fmt.Sprintf("%s%s%s%03d%s",
		m.faker.LetterN(2),       // 2个字母
		m.faker.DigitN(2),        // 2个数字
		m.faker.LetterN(3),       // 3个字母
		m.faker.IntRange(0, 999), // 3位数字
		m.faker.LetterN(4),       // 4个字母
	)

	return serial
}

// generateSerialWithBatch 生成带批次号和序号的序列号（用于随机无人机）
// 格式: B{批次号6位}S{序号4位}，保持 RID 后缀为至少 12 位 ASCII 字母数字。
// 例如: B000001S0001
func (m *mockDataGenerator) generateSerialWithBatch(batchNo, seqNo int) string {
	serial := fmt.Sprintf("B%06dS%04d", batchNo, seqNo)
	return serial
}

// generateModel 生成随机的无人机产品型号（用于DID/RID数据）
func (m *mockDataGenerator) generateModel() string {
	// 从 drone_models.go 中获取所有型号
	allModels := GetAllModels()

	// 随机选择一个型号
	if len(allModels) > 0 {
		return allModels[m.faker.IntRange(0, len(allModels)-1)]
	}

	// 如果没有型号数据，返回空字符串
	return ""
}

// generateDetectionModel 生成随机的侦测技术型号（用于侦测数据）
func (m *mockDataGenerator) generateDetectionModel() string {
	modelKeys := []string{
		"Lightbridge type1",
		"Lightbridge type2",
		"eWifi_5M",
		"eWifi_10M",
		"PAL Analog",
		"NTSC Analog",
		"Autel_type1",
		"Autel_type2",
		"Autel_type3",
		"Autel_type4",
		"Datalink type1",
		"Datalink type2",
		"Datalink_type3",
		"LTE_type0",
		"LORA",
		"Walksnail",
		"DJI_O3+",
		"O3+_ofdm_datalink",
		"DJI_OC123_10M",
		"DJI_OC123_20M",
		"DJI_O4_type",
		"DJI_O4",
		// 有一定概率是空的
		"",
	}

	return modelKeys[m.faker.IntRange(0, len(modelKeys)-1)]
}

// generateRSSI 生成随机的RSSI值
func (m *mockDataGenerator) generateRSSI() int {
	// RSSI取值范围-60dBm至-90dBm
	return m.faker.IntRange(-90, -60)
}

// generateFreq 生成随机的频率值（真实频段）
func (m *mockDataGenerator) generateFreq() float64 {
	// 真实无人机频段：2.4GHz (2400-2500) 和 5GHz (5150-5900)
	if m.faker.IntRange(0, 10) < 4 { // 40% 概率2.4GHz
		return m.faker.Float64Range(2400.0, 2500.0)
	}
	// 60% 概率5GHz
	return m.faker.Float64Range(5150.0, 5900.0)
}

// generateHeight 生成随机的高度值
func (m *mockDataGenerator) generateHeight() int {
	// 无人机高度在50~300m内随机选择
	return m.faker.IntRange(50, 300)
}

// generateGPSAroundCenter 基于配置中心点生成指定范围内的GPS坐标
func (m *mockDataGenerator) generateGPSAroundCenter(radiusKm float64) GPS {
	centerPoint := config.GetConfig().CenterPoint

	// 生成随机角度 (0-360度)
	angle := m.faker.Float64Range(0, 360) * 3.14159265359 / 180 // 转换为弧度

	// 生成随机距离 (0到radiusKm之间)
	distance := m.faker.Float64Range(0, radiusKm)

	// 1度约等于111公里
	latOffset := (distance * math.Cos(angle)) / 111.0
	lngOffset := (distance * math.Sin(angle)) / (111.0 * math.Cos(centerPoint.Lat*3.14159265359/180))

	lat := centerPoint.Lat + latOffset
	lng := centerPoint.Lng + lngOffset
	return GPS{Lat: lat, Lng: lng}
}

// generatePilotGPS 生成随机的飞手GPS坐标（基于中心点）
func (m *mockDataGenerator) generatePilotGPS() GPS {
	// 基于配置中心点生成飞手坐标，范围约为中心点周围10公里
	// 飞手位置生成后基本不变
	// 60%概率有飞手坐标，40%概率无飞手（只有无人机）
	var pilotGPS GPS
	if m.faker.IntRange(0, 100) < 60 {
		pilotGPS = m.generateGPSAroundCenter(10.0)
	} else {
		pilotGPS = GPS{Lat: 0.0, Lng: 0.0} // 无飞手坐标
	}
	return pilotGPS
}

// generateWiFiInfo 生成周围环境的WiFi信息
func (m *mockDataGenerator) generateWiFiInfo() *WiFiInfo {
	// 生成MAC地址
	mac := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		m.faker.IntRange(0, 255),
		m.faker.IntRange(0, 255),
		m.faker.IntRange(0, 255),
		m.faker.IntRange(0, 255),
		m.faker.IntRange(0, 255),
		m.faker.IntRange(0, 255),
	)

	// 生成SSID（20% Hidden，80% 普通SSID）
	var ssid string
	if m.faker.IntRange(0, 10) < 2 {
		ssid = "Hidden"
	} else {
		// 生成随机的WiFi名称
		ssidNames := []string{
			"hx1103", "NSGW_YW", "NSGW_TY", "hkjs", "YZB-office", "BJ_WLAN",
			"FAST_2.4G_CC71", "FAST_2.4G_CC8F", "3-807",
			m.faker.LetterN(6),
			fmt.Sprintf("%s-%d", m.faker.LetterN(4), m.faker.IntRange(100, 999)),
		}
		ssid = ssidNames[m.faker.IntRange(0, len(ssidNames)-1)]
	}

	// 生成频率：2.4GHz 和 5GHz
	var freq float64
	if m.faker.IntRange(0, 10) < 5 { // 50% 2.4GHz
		freq = float64(m.faker.IntRange(2412, 2484)) // 2.4GHz频段
	} else { // 50% 5GHz
		freq = float64(m.faker.IntRange(5170, 5825)) // 5GHz频段
	}

	// 生成RSSI（-80到-30）
	rssi := m.faker.IntRange(-80, -30)

	return &WiFiInfo{
		MAC:  mac,
		SSID: ssid,
		RSSI: rssi,
		Freq: freq,
	}
}

// generateDroneGPSAroundPilot 基于飞手位置生成无人机GPS坐标
// pilotGPS: 飞手的GPS坐标
// maxDistanceKm: 无人机距离飞手的最大距离（公里）
// minDistanceKm: 无人机距离飞手的最小距离（公里）
func (m *mockDataGenerator) generateDroneGPSAroundPilot(pilotGPS GPS, minDistanceKm, maxDistanceKm float64) GPS {
	// 如果没有飞手坐标，则基于中心点生成
	if pilotGPS.Lat == 0 && pilotGPS.Lng == 0 {
		return m.generateGPSAroundCenter(10.0)
	}

	// 生成随机角度 (0-360度)
	angle := m.faker.Float64Range(0, 360) * 3.14159265359 / 180 // 转换为弧度

	// 生成随机距离 (minDistanceKm到maxDistanceKm之间)
	distance := m.faker.Float64Range(minDistanceKm, maxDistanceKm)

	// 1度约等于111公里
	latOffset := (distance * math.Cos(angle)) / 111.0
	lngOffset := (distance * math.Sin(angle)) / (111.0 * math.Cos(pilotGPS.Lat*3.14159265359/180))

	lat := pilotGPS.Lat + latOffset
	lng := pilotGPS.Lng + lngOffset
	return GPS{Lat: lat, Lng: lng}
}

// GenerateAnalysisData 随机生成DID和RID数据，包括混合数据包（粘包）
func (m *mockDataGenerator) GenerateAnalysisData(deviceID int) fmt.Stringer {

	// 根据配置的概率生成空数据包
	cfg := config.GetConfig()
	emptyPacketProbability := cfg.Analysis.EmptyPacketProbability
	if emptyPacketProbability < 0 {
		emptyPacketProbability = 0
	}
	if emptyPacketProbability > 100 {
		emptyPacketProbability = 100
	}

	if m.faker.IntRange(0, 100) < emptyPacketProbability {
		return &EmptyPacketData{
			Freq: m.generateFreq(),
			RSSI: m.faker.IntRange(-90, -30),
		}
	}

	droneInfo := m.getCurrentDroneInfo()
	if droneInfo == nil {
		return &EmptyPacketData{
			Freq: m.generateFreq(),
			RSSI: m.faker.IntRange(-90, -30),
		}
	}

	// 30%概率生成混合数据包（含有数据+WiFi，模拟粘包）
	if m.faker.IntRange(0, 10) < 3 {
		if slices.Contains(m.droneRidList, droneInfo.Serial) {
			return m.generateMixedPacket(droneInfo, "RID", deviceID)
		} else if slices.Contains(m.droneDidList, droneInfo.Serial) {
			return m.generateMixedPacket(droneInfo, "DID", deviceID)
		}
	}

	if slices.Contains(m.droneDidList, droneInfo.Serial) {
		// 检查是否是O3+/O4型号，生成加密报文
		if m.isEncryptedModel(droneInfo.Model) {
			return m.generateEncryptedDID(deviceID, droneInfo)
		}
		return m.generateDID(deviceID, droneInfo)
	}

	if slices.Contains(m.droneRidList, droneInfo.Serial) {
		return m.generateRID(droneInfo)
	}

	// 这里要随机的返回这两个其中一个
	randVal := m.faker.IntRange(0, 2) // 0-2: DID普通、DID加密、RID
	if randVal == 0 {
		return m.generateDID(deviceID, droneInfo)
	} else if randVal == 1 && m.isEncryptedModel(droneInfo.Model) {
		return m.generateEncryptedDID(deviceID, droneInfo)
	}

	return m.generateRID(droneInfo)

}

// generateDID 生成单个Did数据
func (m *mockDataGenerator) generateDID(deviceID int, droneInfo *DroneInfo) *Did {

	// 基于配置中心点生成GPS坐标，范围约为中心点周围50公里
	baseGPS := m.generateGPSAroundCenter(50.0)

	// 家和飞手位置应该在无人机附近
	homeLat := baseGPS.Lat + m.faker.Float64Range(-0.01, 0.01)
	homeLng := baseGPS.Lng + m.faker.Float64Range(-0.01, 0.01)
	homeGPS := fmt.Sprintf("%.6f,%.6f", homeLng, homeLat)

	return &Did{
		Num:       m.faker.IntRange(1, 100),
		Device:    deviceID,
		DroneInfo: droneInfo.GenerateRandomDroneInfo(m.faker),
		UUID:      m.faker.UUID(),
		HomeGPS:   homeGPS,
		Height:    m.generateHeight(),
		Altitude:  m.faker.Float64Range(0.0, 120.0),
		EastV:     m.faker.Float64Range(-20.0, 20.0),
		NorthV:    m.faker.Float64Range(-20.0, 20.0),
		UpV:       m.faker.Float64Range(-10.0, 10.0),
		Distance:  m.faker.Float64Range(0.0, 10.0),
	}

}

func (m *mockDataGenerator) isDJIModel(model string) bool {
	return IsDJIModel(model)
}

// isEncryptedModel 判断是否是支持加密报文的型号（O3+/O4等）
func (m *mockDataGenerator) isEncryptedModel(model string) bool {
	encryptedModels := []string{
		"Air 3",
		"Mavic 3",
		"Mini 4 Pro",
		"Mini 3 Pro",
		"FPV",
		"Matrice 4",
		"Matrice 300 RTK",
	}

	for _, em := range encryptedModels {
		if strings.Contains(model, em) {
			return true
		}
	}
	return false
}

// O3PlusO4PacketType O3+/O4 数据包类型
type O3PlusO4PacketType int

const (
	// O3PlusO4PacketTypeKey 关键数据包 (aa/a3 开头)
	O3PlusO4PacketTypeKey O3PlusO4PacketType = iota
	// O3PlusO4PacketTypeDynamic 动态数据包 (87/80 开头)
	O3PlusO4PacketTypeDynamic
)

// o3PlusO4PacketCounter 用于交替生成 Key 和 Dynamic 数据包
var o3PlusO4PacketCounter int

// loadO3PlusO4Data 加载 O3+/O4 预定义数据文件
// 支持两种文件格式：
//  1. 纯数据格式（每行一条数据）：
//     #=序号, device=设备ID, Encypted Mavic_O4_ID=加密ID, freq=频率, rssi=信号强度, byte,字节数据...
//  2. 日志格式（带时间戳，会自动提取数据行）：
//     [2026-01-12 17:17:40.121]# RECV ASCII FROM 192.168.2.4 :9023>
//     #=0, device=3570, Encypted Mavic_O4_ID=30a958e6, freq=5796.5, rssi=-84, byte,...
func (m *mockDataGenerator) loadO3PlusO4Data() {
	cfg := config.GetConfig()
	dataFile := cfg.Analysis.O3PlusO4DataFile

	if dataFile == nil {
		log.Printf("[O3+/O4] 未配置预定义数据文件，将使用随机生成模式")
		return
	}

	file, err := os.Open(*dataFile)
	if err != nil {
		log.Printf("[O3+/O4] 打开预定义数据文件失败: %v，将使用随机生成模式", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// 增大缓冲区以处理较长的行
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行
		if line == "" {
			continue
		}

		// 跳过时间戳行（日志格式）
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]#") {
			continue
		}

		// 跳过注释行
		if strings.HasPrefix(line, "//") {
			continue
		}

		// 只处理包含 "Encypted" 的数据行（加密数据）
		// 格式: #=序号, device=设备ID, Encypted Mavic_O4_ID=加密ID, freq=频率, rssi=信号强度, byte,字节数据...
		if !strings.HasPrefix(line, "#=") || !strings.Contains(line, "Encypted") {
			continue
		}

		// 解析数据行
		data, err := m.parseO3PlusO4Line(line)
		if err != nil {
			log.Printf("[O3+/O4] 第 %d 行解析失败: %v", lineNo, err)
			continue
		}

		m.o3PlusO4DataList = append(m.o3PlusO4DataList, data)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[O3+/O4] 读取预定义数据文件错误: %v", err)
	}

	log.Printf("[O3+/O4] 成功加载 %d 条预定义数据", len(m.o3PlusO4DataList))
}

// parseO3PlusO4Line 解析 O3+/O4 数据行
func (m *mockDataGenerator) parseO3PlusO4Line(line string) (*O3PlusO4PredefinedData, error) {
	// 格式: #=序号, device=设备ID, Encypted Mavic_O4_ID=加密ID, freq=频率, rssi=信号强度, byte,字节数据...
	data := &O3PlusO4PredefinedData{}

	// 查找 "Encypted " 后的型号和ID
	// 例如: "Encypted Mavic_O4_ID=43556db6"
	encyptedIdx := strings.Index(line, "Encypted ")
	if encyptedIdx == -1 {
		return nil, fmt.Errorf("未找到 Encypted 字段")
	}

	// 提取型号和ID部分
	afterEncypted := line[encyptedIdx+9:] // 跳过 "Encypted "
	idIdx := strings.Index(afterEncypted, "_ID=")
	if idIdx == -1 {
		return nil, fmt.Errorf("未找到 _ID= 字段")
	}
	data.Model = afterEncypted[:idIdx]

	// 提取加密ID
	afterID := afterEncypted[idIdx+4:]
	commaIdx := strings.Index(afterID, ",")
	if commaIdx == -1 {
		return nil, fmt.Errorf("未找到加密ID结束符")
	}
	data.EncryptedID = afterID[:commaIdx]

	// 提取频率
	freqIdx := strings.Index(line, "freq=")
	if freqIdx != -1 {
		afterFreq := line[freqIdx+5:]
		commaIdx := strings.Index(afterFreq, ",")
		if commaIdx != -1 {
			fmt.Sscanf(afterFreq[:commaIdx], "%f", &data.Freq)
		}
	}

	// 提取RSSI
	rssiIdx := strings.Index(line, "rssi=")
	if rssiIdx != -1 {
		afterRssi := line[rssiIdx+5:]
		commaIdx := strings.Index(afterRssi, ",")
		if commaIdx != -1 {
			fmt.Sscanf(afterRssi[:commaIdx], "%d", &data.RSSI)
		}
	}

	// 提取字节数据
	byteIdx := strings.Index(line, "byte,")
	if byteIdx != -1 {
		data.ByteData = line[byteIdx+5:]
	}

	return data, nil
}

// getNextO3PlusO4Data 获取下一条 O3+/O4 预定义数据
// 如果有预定义数据则返回数据，否则返回 nil
func (m *mockDataGenerator) getNextO3PlusO4Data() *O3PlusO4PredefinedData {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.o3PlusO4DataList) == 0 {
		return nil
	}

	data := m.o3PlusO4DataList[m.o3PlusO4DataIndex]
	m.o3PlusO4DataIndex++

	// 循环使用
	if m.o3PlusO4DataIndex >= len(m.o3PlusO4DataList) {
		m.o3PlusO4DataIndex = 0
		log.Printf("[O3+/O4] 预定义数据已循环一轮，重新从头开始")
	}

	return data
}

// generateEncryptedDID 生成加密的DID报文
// O3+/O4 数据包规则：
// - 关键数据包 (Key Packet): 以 "aa" 或 "a3" 开头，需要先被缓存
// - 动态数据包 (Dynamic Packet): 以 "87" 或 "80" 开头，需要配合关键数据包才能解密
// 如果配置了预定义数据文件，则按顺序发送预定义数据；否则随机生成
func (m *mockDataGenerator) generateEncryptedDID(deviceID int, droneInfo *DroneInfo) *EncryptedDID {
	// 优先使用预定义数据
	predefinedData := m.getNextO3PlusO4Data()
	if predefinedData != nil {
		return &EncryptedDID{
			Num:         m.faker.IntRange(1, 100),
			Device:      deviceID,
			EncryptedID: predefinedData.EncryptedID,
			Model:       predefinedData.Model,
			Freq:        predefinedData.Freq,
			RSSI:        predefinedData.RSSI,
			ByteData:    predefinedData.ByteData,
		}
	}

	// 没有预定义数据，使用随机生成
	// 使用无人机的固定加密ID（如果没有则生成一个）
	encryptedID := droneInfo.EncryptedID
	if encryptedID == "" {
		encryptedID = fmt.Sprintf("%07x", m.faker.Uint32()%0x10000000)
	}

	// 生成随机长度的加密字节数据（通常180-200个字节）
	byteCount := m.faker.IntRange(180, 200)
	byteData := make([]string, byteCount)

	// 交替生成 Key 和 Dynamic 数据包
	// 规则：先发 Key 包，再发 Dynamic 包，这样接收端可以正确解密
	o3PlusO4PacketCounter++
	packetType := O3PlusO4PacketType(o3PlusO4PacketCounter % 2)

	// 根据数据包类型设置第一个字节
	var firstByte string
	switch packetType {
	case O3PlusO4PacketTypeKey:
		// 关键数据包：50% 概率 "aa"，50% 概率 "a3"
		if m.faker.IntN(2) == 0 {
			firstByte = "aa"
		} else {
			firstByte = "a3"
		}
	case O3PlusO4PacketTypeDynamic:
		// 动态数据包：50% 概率 "87"，50% 概率 "80"
		if m.faker.IntN(2) == 0 {
			firstByte = "87"
		} else {
			firstByte = "80"
		}
	}

	// 设置第一个字节
	byteData[0] = firstByte

	// 生成剩余的随机字节数据
	for i := 1; i < byteCount; i++ {
		byteData[i] = fmt.Sprintf("%02x", m.faker.IntRange(0, 255))
	}

	// 确定型号名称（从产品型号推断）
	modelName := "Mavic_O4"

	// Freq 和 RSSI 做小幅度随机波动，模拟真实信号变化
	freq := droneInfo.Freq + m.faker.Float64Range(-10.0, 10.0)
	rssi := droneInfo.RSSI + m.faker.IntRange(-10, 10)

	return &EncryptedDID{
		Num:         m.faker.IntRange(1, 100),
		Device:      deviceID,
		EncryptedID: encryptedID,
		Model:       modelName,
		Freq:        freq,
		RSSI:        rssi,
		ByteData:    strings.Join(byteData, ","),
	}
}

// generateRID 生成单个RID数据
func (m *mockDataGenerator) generateRID(droneInfo *DroneInfo) *RID {
	// 生成MAC地址
	mac := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		m.faker.IntRange(0, 255),
		m.faker.IntRange(0, 255),
		m.faker.IntRange(0, 255),
		m.faker.IntRange(0, 255),
		m.faker.IntRange(0, 255),
		m.faker.IntRange(0, 255),
	)

	// 生成序列号（比DID的序列号长一些）
	// serial := fmt.Sprintf("1581%s%s%03d%s",
	// 	m.faker.LetterN(2),       // 2个字母
	// 	m.faker.DigitN(2),        // 2个数字
	// 	m.faker.IntRange(0, 999), // 3位数字
	// 	m.faker.LetterN(7),       // 7个字母
	// )

	// 生成SSID（RID前缀+序列号）
	// ssid := fmt.Sprintf("RID-%s", serial)

	newDroneInfo := droneInfo.GenerateRandomDroneInfo(m.faker)

	if m.isDJIModel(newDroneInfo.Model) && !strings.HasPrefix(newDroneInfo.Serial, ridSerialPrefix) {
		newDroneInfo.Serial = ridSerialPrefix + newDroneInfo.Serial
	}

	return &RID{
		SSID:      newDroneInfo.Serial,
		DroneInfo: newDroneInfo,
		UAType:    UATypeEnum(m.faker.IntRange(0, 15)),
		Speed:     m.faker.Float64Range(0.0, 25.0),
		VSpeed:    m.faker.Float64Range(-10.0, 10.0),
		Direction: m.faker.IntRange(-181, 360),
		AltitudeP: m.faker.Float64Range(-200.0, 500.0),
		AltitudeG: m.faker.Float64Range(0.0, 500.0),
		HeightAGL: float64(m.generateHeight()),
		MAC:       mac,
	}

}

// generateMixedPacket 生成混合数据包（模拟粘包情况）
func (m *mockDataGenerator) generateMixedPacket(droneInfo *DroneInfo, dataType string, deviceID int) *MixedPacketData {
	// 生成1-4个WiFi信息
	wifiCount := m.faker.IntRange(1, 4)
	wifiInfos := make([]*WiFiInfo, wifiCount)
	for i := 0; i < wifiCount; i++ {
		wifiInfos[i] = m.generateWiFiInfo()
	}

	mixedData := &MixedPacketData{
		WiFiInfos: wifiInfos,
	}

	// 根据类型生成对应的数据
	if dataType == "RID" {
		// 生成1-2个RID数据（同一架无人机的多次报文）
		ridCount := m.faker.IntRange(1, 2)
		ridData := make([]*RID, ridCount)
		for i := 0; i < ridCount; i++ {
			ridData[i] = m.generateRID(droneInfo)
		}
		mixedData.RIDData = ridData
	} else if dataType == "DID" {
		// 检查是否是加密型号
		if m.isEncryptedModel(droneInfo.Model) {
			// 生成1-2个加密DID数据
			encCount := m.faker.IntRange(1, 2)
			encData := make([]*EncryptedDID, encCount)
			for i := 0; i < encCount; i++ {
				encData[i] = m.generateEncryptedDID(deviceID, droneInfo)
			}
			mixedData.EncryptedData = encData
		} else {
			// 生成1个普通DID数据（DID通常不会连续多次）
			didData := make([]*Did, 1)
			didData[0] = m.generateDID(deviceID, droneInfo)
			mixedData.DIDData = didData
		}
	}

	return mixedData
}

// GenerateDetectionData 生成单个DetectionData数据
func (m *mockDataGenerator) GenerateDetectionData(deviceID int, command *utils.DetectionCommand) fmt.Stringer {
	if command != nil {
		if command.UsesDirectionData() {
			return m.generateDirectionDetectionData(command, deviceID)
		}
	}

	droneInfo := m.getCurrentDroneInfo()
	if droneInfo == nil {
		return &SpectrumData{
			Device: deviceID,
			Model:  "NoDrone",
			Freq:   0,
			RSSI:   0,
		}
	}

	newDroneInfo := droneInfo.GenerateRandomDroneInfo(m.faker)

	return &SpectrumData{
		Device:      deviceID,
		Model:       droneInfo.DetectionModel, // 使用无人机固定的侦测型号
		Freq:        droneInfo.Freq,           // 使用无人机固定的频率
		RSSI:        float64(newDroneInfo.RSSI) + m.faker.Float64Range(-0.9, 0.9),
		ExtraFields: m.generateSpectrumExtraFields(droneInfo),
	}
}

func (m *mockDataGenerator) GenerateSpectrumFrame(command *utils.DetectionCommand) []byte {
	const fftSize = 64

	frame := make([]byte, 4+fftSize*2)
	startFreqKHz := m.nextSpectrumFrameStartKHz(command)

	binary.BigEndian.PutUint32(frame[:4], uint32(startFreqKHz))

	for i := 0; i < fftSize; i++ {
		power := m.realisticSpectrumPower(startFreqKHz, i, fftSize)

		raw := (power + 180) * 100
		if raw < 0 {
			raw = 0
		}
		binary.BigEndian.PutUint16(frame[4+i*2:6+i*2], uint16(raw))
	}

	return frame
}

func (m *mockDataGenerator) nextSpectrumFrameStartKHz(command *utils.DetectionCommand) int {
	const (
		defaultBandStartMHz = 3600
		defaultBandStopMHz  = 5800
		frameStepMHz        = 30
		headerOffsetKHz     = 2320
	)

	bandStart := defaultBandStartMHz
	bandStop := defaultBandStopMHz
	key := "default"
	if command != nil {
		if command.BandStart > 0 {
			bandStart = command.BandStart
		}
		if command.BandStop > bandStart {
			bandStop = command.BandStop
		}
		key = command.TaskKey()
	}

	maxStart := bandStop - frameStepMHz
	if maxStart < bandStart {
		maxStart = bandStart
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.spectrumScanOffsets == nil {
		m.spectrumScanOffsets = make(map[string]int)
	}

	offset := m.spectrumScanOffsets[key]
	start := bandStart + offset*frameStepMHz
	if start > maxStart {
		offset = 0
		start = bandStart
	}
	m.spectrumScanOffsets[key] = offset + 1

	return start*1000 + headerOffsetKHz
}

func (m *mockDataGenerator) realisticSpectrumPower(startFreqKHz int, pointIndex int, fftSize int) int {
	const (
		lowBandHighNoiseMaxKHz = 350000
		highBandHotStartKHz    = 5200000
		highBandHotStopKHz     = 5900000
	)

	freqKHz := startFreqKHz + int(math.Round(float64(pointIndex)*240))
	base := -94 + m.faker.IntRange(-2, 2)
	slopeBoost := 0
	if pointIndex < 6 {
		slopeBoost = 3 - pointIndex/2
	}

	switch {
	case freqKHz >= highBandHotStartKHz && freqKHz <= highBandHotStopKHz:
		base = -78 + m.faker.IntRange(-4, 4)
	case freqKHz <= lowBandHighNoiseMaxKHz:
		base = -95 + m.faker.IntRange(-3, 3)
	}

	peak := 0
	center := fftSize / 2
	if freqKHz >= highBandHotStartKHz && freqKHz <= highBandHotStopKHz {
		peak = max(0, 24-int(math.Abs(float64(pointIndex-center))*1.2))
	}

	power := base + slopeBoost + peak
	if power > -30 {
		return -30
	}
	if power < -105 {
		return -105
	}
	return power
}

func (m *mockDataGenerator) generateDirectionDetectionData(command *utils.DetectionCommand, deviceID int) fmt.Stringer {
	droneInfo := m.findDirectionDrone(float64(command.Freq))
	if droneInfo == nil {
		return &SpectrumData{
			Device: deviceID,
			Model:  "NoDrone",
			Freq:   float64(command.Freq),
			RSSI:   0,
		}
	}

	seq := m.nextDirectionSeq()
	gpio := seq % 8

	return &SpectrumData{
		Device: deviceID,
		Model:  droneInfo.DetectionModel,
		Freq:   float64(command.Freq),
		RSSI:   m.directionRSSI(droneInfo, gpio),
		ExtraFields: []SpectrumExtraField{
			{
				Key:   "id",
				Value: fmt.Sprintf("%d", 10000+spectrumSeed(droneInfo.Serial, droneInfo.Model)%90000),
			},
			{
				Key:   "seq",
				Value: fmt.Sprintf("%d", seq),
			},
			{
				Key:   "bw",
				Value: "20M",
			},
			{
				Key:   "gpio",
				Value: fmt.Sprintf("%d", gpio),
			},
		},
	}
}

func (m *mockDataGenerator) nextDirectionSeq() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.directionSeq++
	return m.directionSeq
}

func (m *mockDataGenerator) findDirectionDrone(freq float64) *DroneInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.droneList) == 0 {
		return nil
	}

	var best *DroneInfo
	bestDiff := math.MaxFloat64
	for _, drone := range m.droneList {
		if drone == nil {
			continue
		}
		diff := math.Abs(drone.Freq - freq)
		if drone.Freq >= 950 && drone.Freq <= 1010 {
			diff = math.Min(diff, math.Abs(drone.Freq+6200-freq))
		}
		if diff < bestDiff {
			copied := *drone
			best = &copied
			bestDiff = diff
		}
	}

	return best
}

func (m *mockDataGenerator) directionRSSI(droneInfo *DroneInfo, gpio int) float64 {
	bearing := float64(spectrumSeed(droneInfo.Serial, droneInfo.Model) % 360)
	antennaAngle := float64(gpio) * 45
	diff := math.Abs(bearing - antennaAngle)
	if diff > 180 {
		diff = 360 - diff
	}

	return float64(droneInfo.RSSI) + 8 - (diff/45)*4 + m.faker.Float64Range(-0.8, 0.8)
}

func (m *mockDataGenerator) generateSpectrumExtraFields(droneInfo *DroneInfo) []SpectrumExtraField {
	seed := spectrumSeed(droneInfo.Serial, droneInfo.Model)
	bandwidths := []string{"5M", "10M", "20M", "40M"}

	return []SpectrumExtraField{
		{
			Key:   "id",
			Value: fmt.Sprintf("%d", 10000+seed%90000),
		},
		{
			Key:   "seq",
			Value: fmt.Sprintf("%d", 1+seed%4),
		},
		{
			Key:   "bw",
			Value: bandwidths[m.faker.IntRange(0, len(bandwidths)-1)],
		},
		{
			Key:   "gpio",
			Value: fmt.Sprintf("%d", m.faker.IntRange(1, 8)),
		},
	}
}

func spectrumSeed(serial string, model string) uint32 {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(serial))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(model))
	return hasher.Sum32()
}

// GenerateFPVData
func (m *mockDataGenerator) GenerateFPVData() FPVData {
	return "SET+OK\n"
}

// GenerateFPVWarningData 生成FPV告警数据
func (m *mockDataGenerator) GenerateFPVWarningData() fmt.Stringer {
	freq := m.faker.IntRange(400, 6000)
	rssi := m.faker.Float64Range(0.40, 0.90)
	return FPVWarningData(fmt.Sprintf("Waring,Freq %4d,RSSI %4.2f\r\n", freq, rssi))
}

// GenerateDeviceID 生成设备ID
func (m *mockDataGenerator) GenerateDeviceID() int {
	return m.faker.IntRange(1000, 9999)
}

// GenerateDirectionHeartbeatData 生成方向心跳数据
func (m *mockDataGenerator) GenerateDirectionHeartbeatData(deviceID int) fmt.Stringer {
	num := m.faker.IntRange(1, 1000)

	// 计算位移
	distance := m.faker.Float64Range(5.0, 20.0)
	deviceDirectionChange := m.faker.Float64Range(-90.0, 90.0)

	newLat, newLng := utils.CalculateDestination(m.deviceGPS.Lat, m.deviceGPS.Lng, distance, deviceDirectionChange)

	m.deviceGPS = GPS{Lat: newLat, Lng: newLng}

	return &DirectionHeart{
		Num:       num,
		Device:    deviceID,
		HeartBeat: num + 1,
		Longitude: newLng,
		Latitude:  newLat,
	}
}
