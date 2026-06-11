package utils

import (
	"math"
	"testing"
)

// TestCalculateDistance 测试距离计算函数
func TestCalculateDistance(t *testing.T) {
	tests := []struct {
		name     string
		lat1     float64
		lng1     float64
		lat2     float64
		lng2     float64
		expected float64
		delta    float64
	}{
		{
			name:     "同一点",
			lat1:     31.230400,
			lng1:     121.473700,
			lat2:     31.230400,
			lng2:     121.473700,
			expected: 0,
			delta:    0.001,
		},
		{
			name:     "上海到北京大约距离",
			lat1:     31.230400, // 上海
			lng1:     121.473700,
			lat2:     39.904200, // 北京
			lng2:     116.407400,
			expected: 1067000, // 大约1067公里
			delta:    10000,   // 允许10公里误差
		},
		{
			name:     "短距离测试",
			lat1:     31.230400,
			lng1:     121.473700,
			lat2:     31.230500,
			lng2:     121.473800,
			expected: 13.9, // 大约13.9米
			delta:    1.0,  // 允许1米误差
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateDistance(tt.lat1, tt.lng1, tt.lat2, tt.lng2)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("CalculateDistance() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}

// TestCalculateBearing 测试方位角计算函数
func TestCalculateBearing(t *testing.T) {
	tests := []struct {
		name     string
		lat1     float64
		lng1     float64
		lat2     float64
		lng2     float64
		expected float64
		delta    float64
	}{
		{
			name:     "正北方向",
			lat1:     31.230400,
			lng1:     121.473700,
			lat2:     31.240400, // 向北移动
			lng2:     121.473700,
			expected: 0, // 0度表示正北
			delta:    1,
		},
		{
			name:     "正东方向",
			lat1:     31.230400,
			lng1:     121.473700,
			lat2:     31.230400,
			lng2:     121.483700, // 向东移动
			expected: 90,         // 90度表示正东
			delta:    1,
		},
		{
			name:     "正南方向",
			lat1:     31.230400,
			lng1:     121.473700,
			lat2:     31.220400, // 向南移动
			lng2:     121.473700,
			expected: 180, // 180度表示正南
			delta:    1,
		},
		{
			name:     "正西方向",
			lat1:     31.230400,
			lng1:     121.473700,
			lat2:     31.230400,
			lng2:     121.463700, // 向西移动
			expected: 270,        // 270度表示正西
			delta:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBearing(tt.lat1, tt.lng1, tt.lat2, tt.lng2)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("CalculateBearing() = %v, want %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}

// TestNormalizeAngle 测试角度标准化函数
func TestNormalizeAngle(t *testing.T) {
	tests := []struct {
		name     string
		angle    float64
		expected float64
	}{
		{"正常角度", 45, 45},
		{"零度", 0, 0},
		{"360度", 360, 0},
		{"负角度", -45, 315},
		{"大于360度", 450, 90},
		{"大负数", -360, 0},
		{"很大的正数", 720, 0},
		{"很大的负数", -720, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeAngle(tt.angle)
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("NormalizeAngle(%v) = %v, want %v", tt.angle, result, tt.expected)
			}
		})
	}
}

// TestAngleDifference 测试角度差值计算函数
func TestAngleDifference(t *testing.T) {
	tests := []struct {
		name     string
		angle1   float64
		angle2   float64
		expected float64
	}{
		{"相同角度", 45, 45, 0},
		{"小差值", 45, 50, 5},
		{"跨越0度", 350, 10, 20},
		{"跨越180度", 10, 350, 20},
		{"最大差值", 0, 180, 180},
		{"90度差值", 0, 90, 90},
		{"270度差值", 0, 270, 90}, // 应该返回较小的90度而不是270度
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AngleDifference(tt.angle1, tt.angle2)
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("AngleDifference(%v, %v) = %v, want %v", tt.angle1, tt.angle2, result, tt.expected)
			}
		})
	}
}

// TestMetersToLatitudeDegrees 测试米转纬度函数
func TestMetersToLatitudeDegrees(t *testing.T) {
	tests := []struct {
		name     string
		meters   float64
		expected float64
		delta    float64
	}{
		{"1米", 1, 0.000009, 0.000001},
		{"100米", 100, 0.0009, 0.0001},
		{"1000米", 1000, 0.009, 0.001},
		{"零米", 0, 0, 0.000001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MetersToLatitudeDegrees(tt.meters)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("MetersToLatitudeDegrees(%v) = %v, want %v (±%v)", tt.meters, result, tt.expected, tt.delta)
			}
		})
	}
}

// TestMetersToLongitudeDegrees 测试米转经度函数
func TestMetersToLongitudeDegrees(t *testing.T) {
	tests := []struct {
		name     string
		meters   float64
		latitude float64
		expected float64
		delta    float64
	}{
		{"赤道附近1米", 1, 0, 0.000009, 0.000001},
		{"北京纬度100米", 100, 39.904200, 0.00116, 0.0001},
		{"上海纬度1000米", 1000, 31.230400, 0.0104, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MetersToLongitudeDegrees(tt.meters, tt.latitude)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("MetersToLongitudeDegrees(%v, %v) = %v, want %v (±%v)",
					tt.meters, tt.latitude, result, tt.expected, tt.delta)
			}
		})
	}
}

// TestCalculateDestination 测试目标点计算函数
func TestCalculateDestination(t *testing.T) {
	tests := []struct {
		name        string
		lat         float64
		lng         float64
		distance    float64
		bearing     float64
		expectedLat float64
		expectedLng float64
		delta       float64
	}{
		{
			name:        "向北移动1000米",
			lat:         31.230400,
			lng:         121.473700,
			distance:    1000,
			bearing:     0,          // 正北
			expectedLat: 31.239400,  // 大约增加0.009度
			expectedLng: 121.473700, // 经度不变
			delta:       0.001,
		},
		{
			name:        "向东移动1000米",
			lat:         31.230400,
			lng:         121.473700,
			distance:    1000,
			bearing:     90,         // 正东
			expectedLat: 31.230400,  // 纬度不变
			expectedLng: 121.485200, // 经度增加
			delta:       0.001,
		},
		{
			name:        "零距离",
			lat:         31.230400,
			lng:         121.473700,
			distance:    0,
			bearing:     45,
			expectedLat: 31.230400,
			expectedLng: 121.473700,
			delta:       0.000001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultLat, resultLng := CalculateDestination(tt.lat, tt.lng, tt.distance, tt.bearing)
			if math.Abs(resultLat-tt.expectedLat) > tt.delta || math.Abs(resultLng-tt.expectedLng) > tt.delta {
				t.Errorf("CalculateDestination() = (%v, %v), want (%v, %v) (±%v)",
					resultLat, resultLng, tt.expectedLat, tt.expectedLng, tt.delta)
			}
		})
	}
}

// BenchmarkCalculateDistance 基准测试距离计算性能
func BenchmarkCalculateDistance(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateDistance(31.230400, 121.473700, 39.904200, 116.407400)
	}
}

// BenchmarkCalculateBearing 基准测试方位角计算性能
func BenchmarkCalculateBearing(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateBearing(31.230400, 121.473700, 39.904200, 116.407400)
	}
}

// BenchmarkCalculateDestination 基准测试目标点计算性能
func BenchmarkCalculateDestination(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateDestination(31.230400, 121.473700, 1000, 45)
	}
}
