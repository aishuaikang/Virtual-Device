package mocks

import (
	"fmt"
	"strings"
	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/utils"

	"github.com/brianvoe/gofakeit/v7"
)

type GPS struct {
	Lat float64
	Lng float64
}

type DroneInfo struct {
	Serial         string  // 无人机序列号（16个字符）
	Model          string  // 无人机产品型号  类型代号-类型， 代号可以不显示（代号 uint32整数，类型最大32个字符 ）
	DetectionModel string  // 侦测技术型号（用于侦测数据，如 Lightbridge、eWifi 等）
	EncryptedID    string  // 加密ID（O3+/O4型号的固定加密标识，8位十六进制）
	Freq           float64 // 报文的频率，注意这个频率不是图传的频率，遥控器上设置的图传频率(浮点)
	RSSI           int     // 接收电平（int32整型数字）
	DroneGPS       GPS     // 无人机坐标  （浮点）
	PilotGPS       GPS     // 飞手坐标(浮点)

	// 运动状态信息（用于保持运动连续性）
	Direction float64 // 当前运动方向（角度，0-360度）
	Speed     float64 // 当前速度（米/秒）
	LastGPS   *GPS    // 上一次的GPS位置

	// 飞手运动状态信息
	PilotDirection  float64 // 飞手运动方向（角度，0-360度）
	PilotSpeed      float64 // 飞手移动速度（米/秒，通常比无人机慢）
	LastPilotGPS    *GPS    // 飞手上一次的GPS位置
	InitialPilotGPS *GPS    // 飞手初始位置（用于限制移动范围）
}

func (d *DroneInfo) GenerateRandomDroneInfo(faker *gofakeit.Faker) *DroneInfo {
	cfg := config.GetConfig()

	// 初始化运动状态（如果是第一次生成）
	if d.LastGPS == nil {
		d.LastGPS = &GPS{Lat: d.DroneGPS.Lat, Lng: d.DroneGPS.Lng}
		d.Direction = faker.Float64Range(0, 360) // 随机初始方向
		d.Speed = faker.Float64Range(2, 8)       // 初始速度 2-8 m/s
	}
	// 初始化飞手运动状态
	if d.LastPilotGPS == nil && (d.PilotGPS.Lat != 0 || d.PilotGPS.Lng != 0) {
		d.LastPilotGPS = &GPS{Lat: d.PilotGPS.Lat, Lng: d.PilotGPS.Lng}
		d.InitialPilotGPS = &GPS{Lat: d.PilotGPS.Lat, Lng: d.PilotGPS.Lng} // 记录初始位置
		d.PilotDirection = faker.Float64Range(0, 360)
		d.PilotSpeed = faker.Float64Range(0.3, 1.0) // 飞手移动很慢 0.3-1 m/s
	}

	// 计算时间间隔（假设每次调用间隔1秒）
	timeInterval := 1.0 // 秒

	// ========== 更新无人机位置 ==========
	// 方向变化：允许小幅度转向，避免突然折返
	maxDirectionChange := cfg.MaxDirectionChange // 最大转向角度30度
	directionChange := faker.Float64Range(-maxDirectionChange, maxDirectionChange)
	d.Direction += directionChange

	// 保持方向在0-360度范围内
	d.Direction = utils.NormalizeAngle(d.Direction)

	// 速度变化：允许小幅度速度调整
	speedChange := faker.Float64Range(-1, 1) // 速度变化范围 ±1 m/s
	d.Speed += speedChange

	// 限制速度范围在 1-10 m/s
	d.Speed = utils.Clamp(d.Speed, 1, 10)

	// 计算位移
	distance := d.Speed * timeInterval

	// 根据当前位置、距离和方向计算新位置
	newLat, newLng := utils.CalculateDestination(d.DroneGPS.Lat, d.DroneGPS.Lng, distance, d.Direction)

	// 检查是否需要调整方向以避免偏离中心点太远
	centerPoint := cfg.CenterPoint
	maxDistance := cfg.MaxDistanceFromCenterPoint

	// 确定围绕的中心点：有飞手则围绕飞手，否则围绕配置的中心点
	var targetCenter GPS
	if d.PilotGPS.Lat != 0 && d.PilotGPS.Lng != 0 {
		// 有飞手，围绕飞手飞行
		targetCenter = d.PilotGPS
	} else {
		// 无飞手，围绕配置的中心点飞行
		targetCenter = GPS{Lat: centerPoint.Lat, Lng: centerPoint.Lng}
	}

	distanceFromCenter := utils.CalculateDistance(newLat, newLng, targetCenter.Lat, targetCenter.Lng)

	if distanceFromCenter > maxDistance {
		// 计算返回中心点的方向
		bearingToCenter := utils.CalculateBearing(newLat, newLng, targetCenter.Lat, targetCenter.Lng)
		// 调整方向，使其偏向中心点
		directionDiff := bearingToCenter - d.Direction

		// 处理角度差值的周期性
		if directionDiff > 180 {
			directionDiff -= 360
		} else if directionDiff < -180 {
			directionDiff += 360
		}

		// 逐渐调整方向（不要立即转向）
		d.Direction += directionDiff * 0.3 // 30%的调整力度

		// 重新计算位置
		newLat, newLng = utils.CalculateDestination(d.DroneGPS.Lat, d.DroneGPS.Lng, distance, d.Direction)
	}

	// ========== 更新飞手位置 ==========
	// 计算飞手到无人机的距离
	distanceToDrone := utils.CalculateDistance(d.PilotGPS.Lat, d.PilotGPS.Lng, newLat, newLng)

	// 定义飞手跟随行为的参数
	const (
		minFollowDistance = 10.0  // 最小跟随距离（米）
		maxFollowDistance = 100.0 // 最大跟随距离（米）
		optimalDistance   = 50.0  // 理想距离（米）
	)

	// 飞手移动逻辑
	if distanceToDrone > maxFollowDistance {
		// 无人机太远，飞手快速跟随
		bearingToDrone := utils.CalculateBearing(d.PilotGPS.Lat, d.PilotGPS.Lng, newLat, newLng)
		d.PilotDirection = bearingToDrone
		d.PilotSpeed = utils.Clamp(d.PilotSpeed+0.5, 1.0, 3.0) // 加速跟随
	} else if distanceToDrone < minFollowDistance {
		// 无人机太近，飞手缓慢移动或停止
		d.PilotSpeed = utils.Clamp(d.PilotSpeed-0.3, 0, 1.0)
		// 保持当前方向或随机小幅调整
		d.PilotDirection += faker.Float64Range(-10, 10)
	} else {
		// 在合理范围内，飞手保持适度跟随
		if distanceToDrone > optimalDistance {
			// 稍微接近无人机
			bearingToDrone := utils.CalculateBearing(d.PilotGPS.Lat, d.PilotGPS.Lng, newLat, newLng)
			directionDiff := bearingToDrone - d.PilotDirection

			// 处理角度差值
			if directionDiff > 180 {
				directionDiff -= 360
			} else if directionDiff < -180 {
				directionDiff += 360
			}

			d.PilotDirection += directionDiff * 0.2 // 温和转向
			d.PilotSpeed = utils.Clamp(d.PilotSpeed+0.1, 0.5, 2.0)
		} else {
			// 距离合适，保持当前速度或减速
			d.PilotSpeed = utils.Clamp(d.PilotSpeed-0.1, 0.3, 1.5)
			// 随机小幅调整方向
			d.PilotDirection += faker.Float64Range(-5, 5)
		}
	}

	// 规范化飞手方向
	d.PilotDirection = utils.NormalizeAngle(d.PilotDirection)

	// 计算飞手新位置（只有当飞手存在时才计算）
	var pilotGPS GPS
	if d.PilotGPS.Lat != 0 || d.PilotGPS.Lng != 0 {
		// 有飞手坐标，飞手基本不动，只做微小的随机移动
		pilotDistance := d.PilotSpeed * timeInterval * 0.05 // 飞手移动速度非常慢
		newPilotLat, newPilotLng := utils.CalculateDestination(
			d.PilotGPS.Lat,
			d.PilotGPS.Lng,
			pilotDistance,
			d.PilotDirection,
		)

		// 检查是否超出初始位置10米范围
		if d.InitialPilotGPS != nil {
			distanceFromInitial := utils.CalculateDistance(newPilotLat, newPilotLng, d.InitialPilotGPS.Lat, d.InitialPilotGPS.Lng)
			if distanceFromInitial > 0.01 { // 10米 = 0.01公里
				// 超出范围，调整方向返回初始位置
				bearingToInitial := utils.CalculateBearing(newPilotLat, newPilotLng, d.InitialPilotGPS.Lat, d.InitialPilotGPS.Lng)
				d.PilotDirection = bearingToInitial + faker.Float64Range(-30, 30) // 大致朝初始位置方向
			} else {
				// 在范围内，随机调整方向
				d.PilotDirection += faker.Float64Range(-10, 10)
			}
			d.PilotDirection = utils.NormalizeAngle(d.PilotDirection)
		}

		pilotGPS = GPS{
			Lat: newPilotLat,
			Lng: newPilotLng,
		}
	} else {
		// 无飞手坐标，保持为0
		pilotGPS = GPS{Lat: 0, Lng: 0}
	}

	// 更新位置信息
	droneGPS := GPS{
		Lat: newLat,
		Lng: newLng,
	}

	// 更新历史位置
	d.LastGPS = &GPS{Lat: d.DroneGPS.Lat, Lng: d.DroneGPS.Lng}
	if d.PilotGPS.Lat != 0 || d.PilotGPS.Lng != 0 {
		d.LastPilotGPS = &GPS{Lat: d.PilotGPS.Lat, Lng: d.PilotGPS.Lng}
	}
	d.DroneGPS = droneGPS
	d.PilotGPS = pilotGPS

	return &DroneInfo{
		Serial:         d.Serial,
		Model:          d.Model,
		Freq:           faker.Float64Range(d.Freq-10.0, d.Freq+10.0),
		RSSI:           faker.IntRange(d.RSSI-10, d.RSSI+10),
		DroneGPS:       droneGPS,
		PilotGPS:       pilotGPS,
		Direction:      d.Direction,
		Speed:          d.Speed,
		LastGPS:        d.LastGPS,
		PilotDirection: d.PilotDirection,
		PilotSpeed:     d.PilotSpeed,
		LastPilotGPS:   d.LastPilotGPS,
	}
}

// DetectionDataDid 无人机DID数据
type Did struct {
	Num    int
	Device int // 设备编号（uint32整型数字）
	*DroneInfo
	UUID     string  // 飞手执照代码，有可能为空
	HomeGPS  string  // 返航点坐标(浮点)
	Height   int     // 对地高度 m(浮点)
	Altitude float64 // 海拔 m(浮点)
	EastV    float64 // 向东速度 m/s(浮点)
	NorthV   float64 // 向北速度 m/s(浮点)
	UpV      float64 // 向上速度 m/s(浮点)
	Distance float64 // 距离(浮点)
}

func (d *Did) String() string {
	// 根据注释中的格式构造字符串
	// result := fmt.Sprintf("num=%d, device=%d, serial=%s, model=%s, uuid=%s, drone_GPS=%s, home_GPS=%s, pilot_GPS=%s, Height=%d, Altitude=%.1f,EastV=%.1f, NothV=%.1f,UpV=%.1f, freq=%.1f, rssi=%d, distance=%.1fkm,\r\n",

	// 处理飞手GPS：如果没有飞手坐标（0,0），则输出空字符串
	pilotGPSStr := ""
	if d.PilotGPS.Lat != 0 || d.PilotGPS.Lng != 0 {
		pilotGPSStr = fmt.Sprintf("%.6f,%.6f", d.PilotGPS.Lng, d.PilotGPS.Lat)
	}

	result := fmt.Sprintf("num=%d, device=%d, serial=%s, model=%s, uuid=%s, drone_GPS=%s, home_GPS=%s, pilot_GPS=%s, Height=%d, Altitude=%.1f,EastV=%.1f, NothV=%.1f,UpV=%.1f, freq=%.1f, rssi=%d, distance=,\r\n",
		// result := fmt.Sprintf("num=%d, device=%d, serial=%s, model=%s, uuid=%s, drone_GPS=%s, home_GPS=%s, pilot_GPS=%s, Height=%d, Altitude=%.1f,EastV=%.1f, NothV=%.1f,UpV=%.1f, freq=%.1f, rssi=%d,\r\n",
		d.Num,
		d.Device,
		d.Serial,
		d.Model,
		d.UUID,
		fmt.Sprintf("%.6f,%.6f", d.DroneGPS.Lng, d.DroneGPS.Lat),
		d.HomeGPS,
		pilotGPSStr,
		d.Height,
		d.Altitude,
		d.EastV,
		d.NorthV,
		d.UpV,
		d.Freq,
		d.RSSI,
	)

	return result
}

// UATypeEnum 无人机类型枚举
type UATypeEnum int

// 0：未知或未定义
// 1：固定翼飞机
// 2：直升飞机（或多旋翼飞机）
// 3：自转旋翼机
// 4：垂直起降固定翼飞机
// 5：扑翼机
// 6：滑翔机
// 7：风筝
// 8：自由气球
// 9：系留气球
// 10：飞艇
// 11：自由落体/降落伞（无动力）
// 12：火箭
// 13：系留动力飞机
// 14：地面障碍物
// 15：其他
const (
	UATypeUnknown UATypeEnum = iota
	UATypeFixedWing
	UATypeRotaryWing
	UATypeTiltRotor
	UATypeFlappingWing
	UATypeGlider
	UATypeKite
	UATypeFreeBalloon
	UATypeTetheredBalloon
	UATypeAirship
	UATypeFreeFall
	UATypeRocket
	UATypeTetheredPowered
	UATypeGroundObstacle
	UATypeOther
)

// WiFiInfo WiFi信息（周围环境的WiFi热点）
type WiFiInfo struct {
	MAC  string  // MAC地址
	SSID string  // WiFi SSID（可能是Hidden）
	RSSI int     // 信号强度
	Freq float64 // 频率
}

// String 返回WiFi信息的字符串表示
func (w *WiFiInfo) String() string {
	return fmt.Sprintf("wifi mac=%s, ssid=%s, rssi=%d, freq=%.0f\r\n",
		w.MAC,
		w.SSID,
		w.RSSI,
		w.Freq,
	)
}

// MixedPacketData 混合数据包（可能包含多个数据+WiFi信息，模拟真实粘包情况）
type MixedPacketData struct {
	WiFiInfos     []*WiFiInfo     // WiFi信息列表
	RIDData       []*RID          // RID数据列表
	DIDData       []*Did          // DID数据列表
	EncryptedData []*EncryptedDID // 加密DID数据列表
}

// String 返回混合数据包的字符串表示
func (m *MixedPacketData) String() string {
	var result string
	// 随机顺序输出WiFi、RID、DID和加密数据，模拟真实粘包
	for _, wifi := range m.WiFiInfos {
		result += wifi.String()
	}
	for _, rid := range m.RIDData {
		result += rid.String()
	}
	for _, did := range m.DIDData {
		result += did.String()
	}
	for _, enc := range m.EncryptedData {
		result += enc.String()
	}
	return result
}

// DetectionDataRID 无人机RID数据
type RID struct {
	SSID string // 无人机wifi广播的ssid（长度为24，最大为100）
	*DroneInfo
	UAType    UATypeEnum
	Speed     float64 // 垂直速度（浮点）
	VSpeed    float64 // 水平速度（浮点）
	Direction int     // 方向（浮点）
	AltitudeP float64 // 气压高度 （浮点）
	AltitudeG float64 // 几何高度（浮点）
	HeightAGL float64 // 对地高度 （浮点）
	MAC       string  // MAC地址
}

// String 返回RID数据的字符串表示 RID ssid=RID-1581F6Z9C244V003SB1F, serial=1581F6Z9C244V003SB1F, model=DJI Mini 4 pro, UA_type=2, drone_GPS=121.513382,31.335012, pilot_GPS=0.000000,0.000000, speed=0.0, Vspeed=0, direc=-181, AltitudeP=-128.5, AltitudeG=27.0, Height_AGL=0, MAC=60:60:1f:34:be:99, rssi=-93, freq=2437
func (d *RID) String() string {
	// 处理飞手GPS：如果没有飞手坐标（0,0），则输出空字符串
	pilotGPSStr := ""
	if d.PilotGPS.Lat != 0 || d.PilotGPS.Lng != 0 {
		pilotGPSStr = fmt.Sprintf("%.6f,%.6f", d.PilotGPS.Lng, d.PilotGPS.Lat)
	}

	return fmt.Sprintf("RID ssid=%s, serial=%s, model=%s, UA_type=%d, drone_GPS=%s, pilot_GPS=%s, speed=%.1f, Vspeed=%.1f, direc=%d, AltitudeP=%.1f, AltitudeG=%.1f, Height_AGL=%.1f, MAC=%s, rssi=%d, freq=%.1f\r\n",
		d.SSID,
		d.Serial,
		d.Model,
		d.UAType,
		fmt.Sprintf("%.6f,%.6f", d.DroneGPS.Lng, d.DroneGPS.Lat),
		pilotGPSStr,
		d.Speed,
		d.VSpeed,
		d.Direction,
		d.AltitudeP,
		d.AltitudeG,
		d.HeightAGL,
		d.MAC,
		d.RSSI,
		d.Freq,
	)
}

// EncryptedDID 加密的DID报文（O3+/O4等加密无人机）
type EncryptedDID struct {
	Num         int     // 序号
	Device      int     // 设备编号
	EncryptedID string  // 加密ID（如 447e5681）
	Model       string  // 型号（如 Mavic_O4）
	Freq        float64 // 频率
	RSSI        int     // 信号强度
	ByteData    string  // 加密字节数据（十六进制字符串）
}

// String 返回加密DID数据的字符串表示
// 格式：#=12, device=2523, Encypted Mavic_O4_ID=447e5681, freq=5776.5, rssi=-83, byte,55,c6,9b,a7,...
func (e *EncryptedDID) String() string {
	return fmt.Sprintf("#=%d, device=%d, Encypted %s_ID=%s, freq=%.1f, rssi=%d, byte,%s\r\n",
		e.Num,
		e.Device,
		e.Model,
		e.EncryptedID,
		e.Freq,
		e.RSSI,
		e.ByteData,
	)
}

// SpectrumData 频谱数据
type SpectrumExtraField struct {
	Key   string
	Value string
}

type SpectrumData struct {
	Device      int                  // 设备
	Model       string               // 无人机型号
	Freq        float64              // 无人机频率
	RSSI        float64              // 接收信号强度
	ExtraFields []SpectrumExtraField // 随机扩展字段
}

// String 返回频谱数据的字符串表示
// device=4453, model=Autel_type2, freq=5753.5, rssi=-46.4, id=19742, seq=2, bw=20M, gpio=3,
func (s *SpectrumData) String() string {
	parts := []string{
		fmt.Sprintf("device=%d", s.Device),
		fmt.Sprintf("model=%s", s.Model),
		fmt.Sprintf("freq=%.1f", s.Freq),
		fmt.Sprintf("rssi=%.1f", s.RSSI),
	}

	for _, field := range s.ExtraFields {
		if field.Key == "" || field.Value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", field.Key, field.Value))
	}

	return strings.Join(parts, ", ") + ",\r\n"
}

type DirectionData string

func (d DirectionData) String() string {
	return string(d)
}

// EmptyPacketData 空数据包（在指定频率上检测到信号但没有有效载荷）
type EmptyPacketData struct {
	Freq float64 // 频率
	RSSI int     // 接收信号强度
}

// String 返回空数据包的字符串表示 Empty packet, freq=2414.5, rssi=-47
func (e *EmptyPacketData) String() string {
	return fmt.Sprintf("Empty packet, freq=%.1f, rssi=%d\r\n",
		e.Freq,
		e.RSSI,
	)
}

type FPVData string

func (f FPVData) String() string {
	return string(f)
}

// FPVWarningData FPV告警数据类型
type FPVWarningData string

func (f FPVWarningData) String() string {
	return string(f)
}

// DirectionHeart #=1677, device=2942, Heart_Beat_#=1678, longi=0.000000, lati=0.000000,
type DirectionHeart struct {
	Num       int
	Device    int
	HeartBeat int
	Longitude float64
	Latitude  float64
}

func (d *DirectionHeart) String() string {
	return fmt.Sprintf("#=%d, device=%d, Heart_Beat_#=%d, longi=%.6f, lati=%.6f\r\n",
		d.Num,
		d.Device,
		d.HeartBeat,
		d.Longitude,
		d.Latitude,
	)
}
