package utils

import "math"

// calculateDistance 计算两点间距离（米）
// 使用 Haversine 公式计算球面距离
func CalculateDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000 // 地球半径（米）

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLatRad := (lat2 - lat1) * math.Pi / 180
	deltaLngRad := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(deltaLatRad/2)*math.Sin(deltaLatRad/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLngRad/2)*math.Sin(deltaLngRad/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// CalculateBearing 计算从点1到点2的方位角（度）
// 返回值范围：0-360度
func CalculateBearing(lat1, lng1, lat2, lng2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLngRad := (lng2 - lng1) * math.Pi / 180

	y := math.Sin(deltaLngRad) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) - math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(deltaLngRad)

	bearing := math.Atan2(y, x) * 180 / math.Pi
	return math.Mod(bearing+360, 360)
}

// NormalizeAngle 将角度标准化到0-360度范围内
func NormalizeAngle(angle float64) float64 {
	angle = math.Mod(angle, 360)
	if angle < 0 {
		angle += 360
	}
	return angle
}

// AngleDifference 计算两个角度之间的最小差值
// 返回值范围：0-180度
func AngleDifference(angle1, angle2 float64) float64 {
	diff := math.Abs(angle1 - angle2)
	if diff > 180 {
		diff = 360 - diff
	}
	return diff
}

// MetersToLatitudeDegrees 将米转换为纬度度数
// 1度纬度 ≈ 111,111米
func MetersToLatitudeDegrees(meters float64) float64 {
	return meters / 111111.0
}

// MetersToLongitudeDegrees 将米转换为经度度数
// 需要考虑当前纬度的影响
func MetersToLongitudeDegrees(meters, latitude float64) float64 {
	return meters / (111111.0 * math.Cos(latitude*math.Pi/180))
}

// CalculateDestination 根据起点、距离和方向计算目标点
// distance: 距离（米）
// bearing: 方位角（度）
// 返回目标点的经纬度
func CalculateDestination(lat, lng, distance, bearing float64) (float64, float64) {
	const R = 6371000 // 地球半径（米）

	latRad := lat * math.Pi / 180
	bearingRad := bearing * math.Pi / 180

	newLatRad := math.Asin(math.Sin(latRad)*math.Cos(distance/R) +
		math.Cos(latRad)*math.Sin(distance/R)*math.Cos(bearingRad))

	newLngRad := lng*math.Pi/180 + math.Atan2(
		math.Sin(bearingRad)*math.Sin(distance/R)*math.Cos(latRad),
		math.Cos(distance/R)-math.Sin(latRad)*math.Sin(newLatRad))

	newLat := newLatRad * 180 / math.Pi
	newLng := newLngRad * 180 / math.Pi

	return newLat, newLng
}
