package config

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// JointDirectionConfig replaces random detection targets with shared scan samples.
type JointDirectionConfig struct {
	Enabled       bool                   `json:"enabled"`
	HopIntervalMS int                    `json:"hop_interval_ms"`
	Targets       []JointDirectionTarget `json:"targets"`
}

type JointDirectionTarget struct {
	Model       string                 `json:"model"`
	Frequencies []float64              `json:"frequencies"`
	Faces       []JointDirectionSample `json:"faces"`
}

type JointDirectionSample struct {
	DeviceID  int     `json:"deviceID"`
	Reporting bool    `json:"reporting"`
	RSSI      float64 `json:"rssi"`
}

func normalizeJointDirection(c *Config) {
	order := make(map[int]int, len(c.Detections))
	for i, detector := range c.Detections {
		order[detector.DeviceID] = i + 1
	}
	for i := range c.JointDirection.Targets {
		target := &c.JointDirection.Targets[i]
		target.Model = strings.TrimSpace(target.Model)
		seen := make(map[int]bool, len(target.Faces))
		for _, face := range target.Faces {
			seen[face.DeviceID] = true
		}
		for _, detector := range c.Detections {
			if !seen[detector.DeviceID] {
				target.Faces = append(target.Faces, JointDirectionSample{DeviceID: detector.DeviceID, RSSI: -70})
				seen[detector.DeviceID] = true
			}
		}
		// Keep unknown and duplicate IDs so validation still rejects invalid scenes.
		slices.SortStableFunc(target.Faces, func(a, b JointDirectionSample) int {
			return order[a.DeviceID] - order[b.DeviceID]
		})
	}
}

// ValidateJointDirectionUpdate permits sample changes without restarting listeners.
func ValidateJointDirectionUpdate(current, next JointDirectionConfig) error {
	if current.Enabled != next.Enabled || current.HopIntervalMS != next.HopIntervalMS || len(current.Targets) != len(next.Targets) {
		return fmt.Errorf("运行中仅可修改联合测向各面的强度和上报开关，其他配置请先停止模拟器")
	}
	for i, target := range next.Targets {
		previous := current.Targets[i]
		if target.Model != previous.Model || !slices.Equal(target.Frequencies, previous.Frequencies) || len(target.Faces) != len(previous.Faces) {
			return fmt.Errorf("运行中不能修改联合测向目标或跳频配置，请先停止模拟器")
		}
		for j, face := range target.Faces {
			if face.DeviceID != previous.Faces[j].DeviceID {
				return fmt.Errorf("运行中不能修改联合测向模块硬件ID，请先停止模拟器")
			}
			if math.IsNaN(face.RSSI) || math.IsInf(face.RSSI, 0) || face.RSSI < -140 || face.RSSI > 0 {
				return fmt.Errorf("联合测向目标[%d]的RSSI必须在-140到0之间", i+1)
			}
		}
	}
	return nil
}

func validateJointDirection(c *Config) error {
	if !c.JointDirection.Enabled {
		return nil
	}
	if c.JointDirection.HopIntervalMS < 100 || c.JointDirection.HopIntervalMS > 60000 {
		return fmt.Errorf("联合测向跳频间隔必须在100到60000毫秒之间")
	}
	if len(c.JointDirection.Targets) == 0 {
		return fmt.Errorf("联合测向至少需要一个模拟目标")
	}
	devices := make(map[int]bool)
	for _, detector := range c.Detections {
		if devices[detector.DeviceID] {
			return fmt.Errorf("联合测向侦测硬件ID不能重复: %d", detector.DeviceID)
		}
		devices[detector.DeviceID] = true
	}
	for i, target := range c.JointDirection.Targets {
		if target.Model == "" || len(target.Model) > 64 || strings.ContainsAny(target.Model, ",=\r\n") {
			return fmt.Errorf("联合测向目标[%d]型号不能为空、超过64字节或包含逗号、等号、换行", i+1)
		}
		if len(target.Frequencies) == 0 {
			return fmt.Errorf("联合测向目标[%d]至少需要一个频点", i+1)
		}
		for _, freq := range target.Frequencies {
			if math.IsNaN(freq) || math.IsInf(freq, 0) || freq <= 0 {
				return fmt.Errorf("联合测向目标[%d]频点必须为有效正数", i+1)
			}
		}
		seen := make(map[int]bool)
		for _, face := range target.Faces {
			if !devices[face.DeviceID] || seen[face.DeviceID] {
				return fmt.Errorf("联合测向目标[%d]的面必须使用不同的已配置硬件ID", i+1)
			}
			seen[face.DeviceID] = true
			if math.IsNaN(face.RSSI) || math.IsInf(face.RSSI, 0) || face.RSSI < -140 || face.RSSI > 0 {
				return fmt.Errorf("联合测向目标[%d]的RSSI必须在-140到0之间", i+1)
			}
		}
	}
	return nil
}
