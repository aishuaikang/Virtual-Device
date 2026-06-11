package utils

import "math"

// Clamp 将值限制在指定范围内
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClampInt 将整数值限制在指定范围内
func ClampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Lerp 线性插值函数
// t的值在0-1之间，0返回a，1返回b
func Lerp(a, b, t float64) float64 {
	return a + t*(b-a)
}

// SmoothStep 平滑过渡函数（S曲线）
// t的值在0-1之间，返回平滑的过渡值
func SmoothStep(t float64) float64 {
	t = Clamp(t, 0, 1)
	return t * t * (3 - 2*t)
}

// DegreesToRadians 角度转弧度
func DegreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

// RadiansToDegrees 弧度转角度
func RadiansToDegrees(radians float64) float64 {
	return radians * 180 / math.Pi
}

// Abs 返回浮点数的绝对值
func Abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

// AbsInt 返回整数的绝对值
func AbsInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// Min 返回两个浮点数中的较小值
func Min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Max 返回两个浮点数中的较大值
func Max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// MinInt 返回两个整数中的较小值
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MaxInt 返回两个整数中的较大值
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// IsNearlyEqual 判断两个浮点数是否近似相等
func IsNearlyEqual(a, b, epsilon float64) bool {
	return Abs(a-b) < epsilon
}

// Round 四舍五入到指定小数位数
func Round(value float64, decimals int) float64 {
	multiplier := math.Pow(10, float64(decimals))
	return math.Round(value*multiplier) / multiplier
}
