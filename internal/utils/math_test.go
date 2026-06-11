package utils

import (
	"math"
	"testing"
)

// TestClamp 测试浮点数限制函数
func TestClamp(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		min      float64
		max      float64
		expected float64
	}{
		{"正常范围内", 5.0, 0.0, 10.0, 5.0},
		{"小于最小值", -1.0, 0.0, 10.0, 0.0},
		{"大于最大值", 15.0, 0.0, 10.0, 10.0},
		{"等于最小值", 0.0, 0.0, 10.0, 0.0},
		{"等于最大值", 10.0, 0.0, 10.0, 10.0},
		{"负数范围", -5.0, -10.0, -1.0, -5.0},
		{"负数超出范围", -15.0, -10.0, -1.0, -10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Clamp(tt.value, tt.min, tt.max)
			if result != tt.expected {
				t.Errorf("Clamp(%v, %v, %v) = %v, want %v", tt.value, tt.min, tt.max, result, tt.expected)
			}
		})
	}
}

// TestClampInt 测试整数限制函数
func TestClampInt(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		min      int
		max      int
		expected int
	}{
		{"正常范围内", 5, 0, 10, 5},
		{"小于最小值", -1, 0, 10, 0},
		{"大于最大值", 15, 0, 10, 10},
		{"等于最小值", 0, 0, 10, 0},
		{"等于最大值", 10, 0, 10, 10},
		{"负数范围", -5, -10, -1, -5},
		{"负数超出范围", -15, -10, -1, -10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClampInt(tt.value, tt.min, tt.max)
			if result != tt.expected {
				t.Errorf("ClampInt(%v, %v, %v) = %v, want %v", tt.value, tt.min, tt.max, result, tt.expected)
			}
		})
	}
}

// TestLerp 测试线性插值函数
func TestLerp(t *testing.T) {
	tests := []struct {
		name     string
		a        float64
		b        float64
		t        float64
		expected float64
		delta    float64
	}{
		{"起点", 0.0, 10.0, 0.0, 0.0, 0.001},
		{"终点", 0.0, 10.0, 1.0, 10.0, 0.001},
		{"中点", 0.0, 10.0, 0.5, 5.0, 0.001},
		{"四分之一", 0.0, 10.0, 0.25, 2.5, 0.001},
		{"四分之三", 0.0, 10.0, 0.75, 7.5, 0.001},
		{"负数范围", -5.0, 5.0, 0.5, 0.0, 0.001},
		{"反向范围", 10.0, 0.0, 0.5, 5.0, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Lerp(tt.a, tt.b, tt.t)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("Lerp(%v, %v, %v) = %v, want %v (±%v)", tt.a, tt.b, tt.t, result, tt.expected, tt.delta)
			}
		})
	}
}

// TestSmoothStep 测试平滑过渡函数
func TestSmoothStep(t *testing.T) {
	tests := []struct {
		name     string
		t        float64
		expected float64
		delta    float64
	}{
		{"起点", 0.0, 0.0, 0.001},
		{"终点", 1.0, 1.0, 0.001},
		{"中点", 0.5, 0.5, 0.001},
		{"小于0", -0.5, 0.0, 0.001},
		{"大于1", 1.5, 1.0, 0.001},
		{"四分之一", 0.25, 0.15625, 0.001}, // 0.25^2 * (3 - 2*0.25) = 0.15625
		{"四分之三", 0.75, 0.84375, 0.001}, // 0.75^2 * (3 - 2*0.75) = 0.84375
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SmoothStep(tt.t)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("SmoothStep(%v) = %v, want %v (±%v)", tt.t, result, tt.expected, tt.delta)
			}
		})
	}
}

// TestDegreesToRadians 测试角度转弧度函数
func TestDegreesToRadians(t *testing.T) {
	tests := []struct {
		name     string
		degrees  float64
		expected float64
		delta    float64
	}{
		{"0度", 0, 0, 0.001},
		{"90度", 90, math.Pi / 2, 0.001},
		{"180度", 180, math.Pi, 0.001},
		{"270度", 270, 3 * math.Pi / 2, 0.001},
		{"360度", 360, 2 * math.Pi, 0.001},
		{"45度", 45, math.Pi / 4, 0.001},
		{"负90度", -90, -math.Pi / 2, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DegreesToRadians(tt.degrees)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("DegreesToRadians(%v) = %v, want %v (±%v)", tt.degrees, result, tt.expected, tt.delta)
			}
		})
	}
}

// TestRadiansToDegrees 测试弧度转角度函数
func TestRadiansToDegrees(t *testing.T) {
	tests := []struct {
		name     string
		radians  float64
		expected float64
		delta    float64
	}{
		{"0弧度", 0, 0, 0.001},
		{"π/2弧度", math.Pi / 2, 90, 0.001},
		{"π弧度", math.Pi, 180, 0.001},
		{"3π/2弧度", 3 * math.Pi / 2, 270, 0.001},
		{"2π弧度", 2 * math.Pi, 360, 0.001},
		{"π/4弧度", math.Pi / 4, 45, 0.001},
		{"负π/2弧度", -math.Pi / 2, -90, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RadiansToDegrees(tt.radians)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("RadiansToDegrees(%v) = %v, want %v (±%v)", tt.radians, result, tt.expected, tt.delta)
			}
		})
	}
}

// TestAbs 测试浮点数绝对值函数
func TestAbs(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected float64
	}{
		{"正数", 5.5, 5.5},
		{"负数", -5.5, 5.5},
		{"零", 0.0, 0.0},
		{"负零", 0.0, 0.0},
		{"很小的正数", 0.001, 0.001},
		{"很小的负数", -0.001, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Abs(tt.value)
			if result != tt.expected {
				t.Errorf("Abs(%v) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

// TestAbsInt 测试整数绝对值函数
func TestAbsInt(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		expected int
	}{
		{"正数", 5, 5},
		{"负数", -5, 5},
		{"零", 0, 0},
		{"最大正整数", math.MaxInt32, math.MaxInt32},
		{"最小负整数+1", math.MinInt32 + 1, math.MaxInt32}, // 避免溢出
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AbsInt(tt.value)
			if result != tt.expected {
				t.Errorf("AbsInt(%v) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

// TestMin 测试浮点数最小值函数
func TestMin(t *testing.T) {
	tests := []struct {
		name     string
		a        float64
		b        float64
		expected float64
	}{
		{"第一个较小", 3.0, 5.0, 3.0},
		{"第二个较小", 5.0, 3.0, 3.0},
		{"相等", 4.0, 4.0, 4.0},
		{"负数", -3.0, -5.0, -5.0},
		{"正负数", -3.0, 5.0, -3.0},
		{"零和正数", 0.0, 1.0, 0.0},
		{"零和负数", 0.0, -1.0, -1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Min(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Min(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// TestMax 测试浮点数最大值函数
func TestMax(t *testing.T) {
	tests := []struct {
		name     string
		a        float64
		b        float64
		expected float64
	}{
		{"第一个较大", 5.0, 3.0, 5.0},
		{"第二个较大", 3.0, 5.0, 5.0},
		{"相等", 4.0, 4.0, 4.0},
		{"负数", -3.0, -5.0, -3.0},
		{"正负数", -3.0, 5.0, 5.0},
		{"零和正数", 0.0, 1.0, 1.0},
		{"零和负数", 0.0, -1.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Max(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Max(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// TestMinInt 测试整数最小值函数
func TestMinInt(t *testing.T) {
	tests := []struct {
		name     string
		a        int
		b        int
		expected int
	}{
		{"第一个较小", 3, 5, 3},
		{"第二个较小", 5, 3, 3},
		{"相等", 4, 4, 4},
		{"负数", -3, -5, -5},
		{"正负数", -3, 5, -3},
		{"零和正数", 0, 1, 0},
		{"零和负数", 0, -1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MinInt(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("MinInt(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// TestMaxInt 测试整数最大值函数
func TestMaxInt(t *testing.T) {
	tests := []struct {
		name     string
		a        int
		b        int
		expected int
	}{
		{"第一个较大", 5, 3, 5},
		{"第二个较大", 3, 5, 5},
		{"相等", 4, 4, 4},
		{"负数", -3, -5, -3},
		{"正负数", -3, 5, 5},
		{"零和正数", 0, 1, 1},
		{"零和负数", 0, -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaxInt(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("MaxInt(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// TestIsNearlyEqual 测试浮点数近似相等函数
func TestIsNearlyEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        float64
		b        float64
		epsilon  float64
		expected bool
	}{
		{"完全相等", 1.0, 1.0, 0.001, true},
		{"在误差范围内", 1.0, 1.0005, 0.001, true},
		{"超出误差范围", 1.0, 1.002, 0.001, false},
		{"负数在误差范围内", -1.0, -1.0005, 0.001, true},
		{"负数超出误差范围", -1.0, -1.002, 0.001, false},
		{"零和很小的数", 0.0, 0.0001, 0.001, true},
		{"零和较大的数", 0.0, 0.002, 0.001, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNearlyEqual(tt.a, tt.b, tt.epsilon)
			if result != tt.expected {
				t.Errorf("IsNearlyEqual(%v, %v, %v) = %v, want %v", tt.a, tt.b, tt.epsilon, result, tt.expected)
			}
		})
	}
}

// TestRound 测试四舍五入函数
func TestRound(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		decimals int
		expected float64
		delta    float64
	}{
		{"整数", 1.0, 2, 1.0, 0.001},
		{"一位小数", 1.23, 1, 1.2, 0.001},
		{"两位小数", 1.234, 2, 1.23, 0.001},
		{"四舍五入向上", 1.235, 2, 1.24, 0.001},
		{"四舍五入向下", 1.234, 2, 1.23, 0.001},
		{"负数", -1.235, 2, -1.24, 0.001},
		{"零位小数", 1.6, 0, 2.0, 0.001},
		{"负零位小数", 1.4, 0, 1.0, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Round(tt.value, tt.decimals)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("Round(%v, %v) = %v, want %v (±%v)", tt.value, tt.decimals, result, tt.expected, tt.delta)
			}
		})
	}
}

// BenchmarkClamp 基准测试限制函数性能
func BenchmarkClamp(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Clamp(5.5, 0, 10)
	}
}

// BenchmarkLerp 基准测试线性插值性能
func BenchmarkLerp(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Lerp(0, 10, 0.5)
	}
}

// BenchmarkSmoothStep 基准测试平滑过渡性能
func BenchmarkSmoothStep(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SmoothStep(0.5)
	}
}

// BenchmarkDegreesToRadians 基准测试角度转弧度性能
func BenchmarkDegreesToRadians(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DegreesToRadians(90)
	}
}

// BenchmarkRadiansToDegrees 基准测试弧度转角度性能
func BenchmarkRadiansToDegrees(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RadiansToDegrees(math.Pi / 2)
	}
}
