package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateConfigNormalizesOptionalPaths(t *testing.T) {
	cfg := DefaultConfig()
	blank := "   "
	cfg.Analysis.O3PlusO4DataFile = &blank
	cfg.Analysis.Hosts = []string{" 127.0.0.1 ", "", " 192.168.1.2 "}
	cfg.Detections[0].Host = " 127.0.0.1 "

	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig returned error: %v", err)
	}

	if cfg.Analysis.O3PlusO4DataFile != nil {
		t.Fatalf("expected empty analysis data file to normalize to nil")
	}
	if cfg.Detection.Host != "127.0.0.1" {
		t.Fatalf("expected detection host to be trimmed, got %q", cfg.Detection.Host)
	}
	if len(cfg.Analysis.Hosts) != 2 || cfg.Analysis.Hosts[0] != "127.0.0.1" || cfg.Analysis.Hosts[1] != "192.168.1.2" {
		t.Fatalf("expected analysis hosts to be trimmed and empty items dropped, got %#v", cfg.Analysis.Hosts)
	}
}

func TestValidateConfigRejectsInvalidLongitude(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CenterPoint.Lng = 181

	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected invalid longitude to fail validation")
	}
}

func TestDefaultConfigDetectionHostUsesAllInterfaces(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Detection.Host != "0.0.0.0" {
		t.Fatalf("expected default detection host to listen on all interfaces, got %q", cfg.Detection.Host)
	}
}

func TestDefaultConfigIncludesThreeDetectionModules(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.Detections) != 3 {
		t.Fatalf("expected 3 default detection modules, got %d", len(cfg.Detections))
	}
	if cfg.Detections[0].Port != 9024 || cfg.Detections[1].Port != 9026 || cfg.Detections[2].Port != 9028 {
		t.Fatalf("unexpected default detection ports: %#v", cfg.Detections)
	}
}

func TestDefaultConfigEnablesDirectedStrikeSimulator(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.DirectedStrike.Enabled {
		t.Fatal("expected directed strike simulator to be enabled")
	}
	if cfg.DirectedStrike.Host != "0.0.0.0" || cfg.DirectedStrike.Port != 19000 {
		t.Fatalf("unexpected directed strike listen address: %#v", cfg.DirectedStrike)
	}
}

func TestValidateConfigRejectsInvalidDirectedStrikeDelay(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DirectedStrike.ResponseDelayMS = 10001

	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "回执延迟") {
		t.Fatalf("expected directed strike delay validation error, got %v", err)
	}
}

func TestLegacyDetectionConfigPopulatesDetections(t *testing.T) {
	raw := `{
		"min_push_speed": 100,
		"max_push_speed": 200,
		"random_drone_refresh_interval": 0,
		"max_direction_change": 30,
		"max_distance_from_center_point": 100,
		"center_point": {"lat": 28.2, "lng": 117.1},
		"analysis": {"enabled": false},
		"detection": {"enabled": true, "deviceID": 2000, "drone_count": 1, "host": "127.0.0.1", "port": 9024, "heartbeat_interval": 5},
		"fpv": {"enabled": false},
		"jamming": {"enabled": false}
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal legacy config: %v", err)
	}
	if err := ValidateConfig(&cfg); err != nil {
		t.Fatalf("validate legacy config: %v", err)
	}

	if len(cfg.Detections) != 1 {
		t.Fatalf("expected legacy detection to become one detection module, got %d", len(cfg.Detections))
	}
	if cfg.Detection.DeviceID != 2000 || cfg.Detections[0].DeviceID != 2000 {
		t.Fatalf("expected detection compatibility fields to be synchronized, got detection=%#v detections=%#v", cfg.Detection, cfg.Detections)
	}
	if cfg.DirectedStrike.Enabled {
		t.Fatal("expected a legacy config to preserve the new module as disabled")
	}
	if cfg.DirectedStrike.Host != "0.0.0.0" || cfg.DirectedStrike.Port != 19000 {
		t.Fatalf("expected legacy config to receive usable directed strike defaults, got %#v", cfg.DirectedStrike)
	}
}

func TestValidateConfigRejectsDuplicateDetectionListenAddress(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Detections[1].Host = cfg.Detections[0].Host
	cfg.Detections[1].Port = cfg.Detections[0].Port

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected duplicate detection listen address to fail validation")
	}
	if !strings.Contains(err.Error(), "监听地址重复") {
		t.Fatalf("expected duplicate listen address error, got %v", err)
	}
}
