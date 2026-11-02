package config

import (
	"encoding/json"
	"fmt"
)

const (
	JammingFaultNormal        = "normal"
	JammingFaultDropACK       = "drop_ack"
	JammingFaultDropStatus    = "drop_status"
	JammingFaultSilent        = "silent"
	JammingFaultStuckOn       = "stuck_on"
	JammingFaultPartialStatus = "partial_status"
	JammingFaultDisconnected  = "disconnected"
)

type JammingConfig struct {
	Enabled         bool     `json:"enabled"`
	DeviceID        int      `json:"deviceID"`
	Hosts           []string `json:"hosts"`
	Port            int      `json:"port"`
	ResponseDelayMS int      `json:"response_delay_ms"`
	FaultMode       string   `json:"fault_mode"`
	FaultChannel    int      `json:"fault_channel"`
}

func (c *JammingConfig) UnmarshalJSON(data []byte) error {
	type plain JammingConfig
	value := plain{Enabled: true, FaultMode: JammingFaultNormal, FaultChannel: 1}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*c = JammingConfig(value)
	c.normalize()
	return nil
}

func (c *JammingConfig) normalize() {
	c.Hosts = normalizeHosts(c.Hosts)
	if c.FaultMode == "" {
		c.FaultMode = JammingFaultNormal
	}
	if c.FaultChannel == 0 {
		c.FaultChannel = 1
	}
}

func ValidateJammingSimulation(c JammingConfig) error {
	if c.ResponseDelayMS < 0 || c.ResponseDelayMS > 10000 {
		return fmt.Errorf("应答延迟必须在0到10000毫秒之间")
	}
	if c.FaultChannel < 1 || c.FaultChannel > 10 {
		return fmt.Errorf("故障通道必须在1到10之间")
	}
	switch c.FaultMode {
	case JammingFaultNormal, JammingFaultDropACK, JammingFaultDropStatus,
		JammingFaultSilent, JammingFaultStuckOn, JammingFaultPartialStatus, JammingFaultDisconnected:
		return nil
	default:
		return fmt.Errorf("未知的全向故障模式: %s", c.FaultMode)
	}
}

func validateJammingConfig(c JammingConfig) error {
	if !c.Enabled {
		return nil
	}
	base := BaseConfig{Enabled: c.Enabled, DeviceID: c.DeviceID, Hosts: c.Hosts, Port: c.Port}
	if err := validateBaseConfig(&base, "Jamming"); err != nil {
		return err
	}
	if c.DeviceID > 65535 {
		return fmt.Errorf("设备ID不能超过65535")
	}
	return ValidateJammingSimulation(c)
}
