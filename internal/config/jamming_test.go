package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJammingConfigRoundTrip(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		data string
	}{
		{"legacy", `{"deviceID":2345,"hosts":[" 127.0.0.1 "],"port":10003}`},
		{"disabled", `{"enabled":false,"deviceID":2345,"port":10003}`},
		{"faults", `{"enabled":true,"deviceID":2345,"hosts":["127.0.0.1"],"port":10003,"response_delay_ms":1000,"fault_mode":"stuck_on","fault_channel":10}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			if err := json.Unmarshal([]byte(tt.data), &cfg.Jamming); err != nil {
				t.Fatal(err)
			}
			if err := ValidateConfig(cfg); err != nil {
				t.Fatal(err)
			}
			if tt.name == "legacy" && (!cfg.Jamming.Enabled || cfg.Jamming.FaultMode != JammingFaultNormal || cfg.Jamming.FaultChannel != 1) {
				t.Fatalf("legacy defaults: %+v", cfg.Jamming)
			}
			if tt.name == "disabled" && cfg.Jamming.Enabled {
				t.Fatal("explicit disabled flag was lost")
			}
			data, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			var restored Config
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(restored.Jamming, cfg.Jamming) {
				t.Fatalf("round trip changed config: %+v, want %+v", restored.Jamming, cfg.Jamming)
			}
		})
	}
}

func TestJammingConfigValidation(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{JammingFaultNormal, JammingFaultDropACK, JammingFaultDropStatus, JammingFaultSilent, JammingFaultStuckOn, JammingFaultPartialStatus, JammingFaultDisconnected} {
		t.Run(mode, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Jamming.FaultMode = mode
			if err := ValidateConfig(cfg); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, tt := range []struct {
		name   string
		change func(*JammingConfig)
	}{
		{"negative delay", func(c *JammingConfig) { c.ResponseDelayMS = -1 }},
		{"excessive delay", func(c *JammingConfig) { c.ResponseDelayMS = 10001 }},
		{"unknown fault", func(c *JammingConfig) { c.FaultMode = "other" }},
		{"negative channel", func(c *JammingConfig) { c.FaultChannel = -1 }},
		{"excessive channel", func(c *JammingConfig) { c.FaultChannel = 11 }},
		{"excessive device ID", func(c *JammingConfig) { c.DeviceID = 65536 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.change(&cfg.Jamming)
			if err := ValidateConfig(cfg); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
