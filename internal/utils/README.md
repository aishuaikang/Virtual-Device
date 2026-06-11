# Utils 工具包

这个包提供了一系列通用的工具函数，主要用于地理计算和数学运算。

## 包结构

- `geo.go` - 地理计算相关函数
- `math.go` - 数学工具函数
- `geo_test.go` - 地理计算函数的单元测试
- `math_test.go` - 数学工具函数的单元测试

## 地理计算函数 (geo.go)

### 距离和方位计算

#### `CalculateDistance(lat1, lng1, lat2, lng2 float64) float64`
使用 Haversine 公式计算两点间的球面距离（米）。

```go
distance := utils.CalculateDistance(31.230400, 121.473700, 39.904200, 116.407400)
// 返回上海到北京的距离，约1067000米
```

#### `CalculateBearing(lat1, lng1, lat2, lng2 float64) float64`
计算从点1到点2的方位角（度），返回值范围：0-360度。

```go
bearing := utils.CalculateBearing(31.230400, 121.473700, 31.240400, 121.473700)
// 返回0度，表示正北方向
```

#### `CalculateDestination(lat, lng, distance, bearing float64) (float64, float64)`
根据起点、距离和方向计算目标点的经纬度。

```go
newLat, newLng := utils.CalculateDestination(31.230400, 121.473700, 1000, 90)
// 从起点向东移动1000米后的新坐标
```

### 角度处理

#### `NormalizeAngle(angle float64) float64`
将角度标准化到0-360度范围内。

```go
normalized := utils.NormalizeAngle(-45) // 返回315
normalized := utils.NormalizeAngle(450) // 返回90
```

#### `AngleDifference(angle1, angle2 float64) float64`
计算两个角度之间的最小差值（0-180度）。

```go
diff := utils.AngleDifference(350, 10) // 返回20度，而不是340度
```

### 单位转换

#### `MetersToLatitudeDegrees(meters float64) float64`
将米转换为纬度度数（1度纬度 ≈ 111,111米）。

#### `MetersToLongitudeDegrees(meters, latitude float64) float64`
将米转换为经度度数，需要考虑当前纬度的影响。

## 数学工具函数 (math.go)

### 数值限制

#### `Clamp(value, min, max float64) float64`
将浮点数值限制在指定范围内。

```go
result := utils.Clamp(15.0, 0.0, 10.0) // 返回10.0
```

#### `ClampInt(value, min, max int) int`
将整数值限制在指定范围内。

### 插值函数

#### `Lerp(a, b, t float64) float64`
线性插值函数，t的值在0-1之间。

```go
result := utils.Lerp(0.0, 10.0, 0.5) // 返回5.0
```

#### `SmoothStep(t float64) float64`
平滑过渡函数（S曲线），提供更自然的过渡效果。

### 角度转换

#### `DegreesToRadians(degrees float64) float64`
角度转弧度。

#### `RadiansToDegrees(radians float64) float64`
弧度转角度。

### 基础数学运算

#### `Abs(value float64) float64`
返回浮点数的绝对值。

#### `Min(a, b float64) float64` / `Max(a, b float64) float64`
返回两个浮点数中的较小值/较大值。

#### `MinInt(a, b int) int` / `MaxInt(a, b int) int`
返回两个整数中的较小值/较大值。

#### `IsNearlyEqual(a, b, epsilon float64) bool`
判断两个浮点数是否在指定误差范围内相等。

#### `Round(value float64, decimals int) float64`
四舍五入到指定小数位数。

## 使用示例

### 无人机轨迹计算

```go
import "virtual-device/internal/utils"

// 计算新位置
currentLat, currentLng := 31.230400, 121.473700
distance := 5.0 // 5米
direction := 45.0 // 东北方向

newLat, newLng := utils.CalculateDestination(currentLat, currentLng, distance, direction)

// 计算移动距离
actualDistance := utils.CalculateDistance(currentLat, currentLng, newLat, newLng)

// 计算方向变化
bearing := utils.CalculateBearing(currentLat, currentLng, newLat, newLng)
directionChange := utils.AngleDifference(direction, bearing)
```

### 数值处理

```go
// 限制速度范围
speed := utils.Clamp(currentSpeed + speedChange, 1.0, 10.0)

// 平滑方向调整
newDirection := utils.Lerp(oldDirection, targetDirection, 0.3)

// 角度标准化
direction = utils.NormalizeAngle(direction + directionChange)
```

## 测试

运行所有测试：

```bash
go test ./internal/utils/ -v
```

运行基准测试：

```bash
go test ./internal/utils/ -bench=.
```

## 性能特性

- 所有地理计算函数都使用高精度的球面几何算法
- 数学函数都经过优化，适合频繁调用
- 包含完整的单元测试和基准测试
- 所有函数都是无状态的，线程安全

## 注意事项

1. 地理计算基于 WGS84 椭球体模型
2. 距离计算精度在全球范围内误差小于 0.5%
3. 角度计算统一使用度（°）作为单位
4. 所有函数都会处理边界情况和异常输入

## 依赖

- Go 标准库 `math` 包
- 无外部依赖