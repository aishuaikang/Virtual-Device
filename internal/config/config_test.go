package config

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestAnalysisDeviceIDsConfigRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name     string
		deviceID string
		wantIDs  []int
	}{
		{"legacy numeric ID", `2222`, []int{2222}},
		{"single ID", `"2222"`, []int{2222}},
		{"multiple IDs", `"2222,3333"`, []int{2222, 3333}},
		{"surrounding spaces", `" 2222 , 3333 "`, []int{2222, 3333}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			raw := `{"deviceID":` + tc.deviceID + `,"drone_count":3,"hosts":["127.0.0.1"],"port":10002}`
			if err := json.Unmarshal([]byte(raw), &cfg.Analysis); err != nil {
				t.Fatalf("unmarshal analysis config: %v", err)
			}
			if err := ValidateConfig(cfg); err != nil {
				t.Fatalf("validate analysis config: %v", err)
			}
			if got, err := cfg.Analysis.DeviceIDs(); err != nil || !reflect.DeepEqual(got, tc.wantIDs) {
				t.Fatalf("device IDs = %v, %v; want %v", got, err, tc.wantIDs)
			}
			encoded, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			var restored Config
			if err := json.Unmarshal(encoded, &restored); err != nil {
				t.Fatalf("restore config: %v", err)
			}
			if !reflect.DeepEqual(restored.Analysis, cfg.Analysis) {
				t.Fatalf("analysis config changed after round trip: %+v, want %+v", restored.Analysis, cfg.Analysis)
			}
		})
	}
}

func TestAnalysisDeviceIDsConfigRejectsInvalidIDs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		deviceID string
	}{
		{"empty", ""},
		{"zero", "2222,0"},
		{"negative", "2222,-1"},
		{"non-numeric", "2222,abc"},
		{"fractional", "2222,3.3"},
		{"missing ID", "2222,,3333"},
		{"trailing comma", "2222,"},
		{"duplicate", "2222,2222"},
		{"duplicate with leading zeros", "2222,02222"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			raw, err := json.Marshal(tc.deviceID)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(`{"deviceID":`+string(raw)+`,"hosts":["127.0.0.1"],"port":10002}`), &cfg.Analysis); err != nil {
				t.Fatalf("unmarshal draft ID: %v", err)
			}
			if err := ValidateConfig(cfg); err == nil || !strings.Contains(err.Error(), "设备ID") {
				t.Fatalf("expected invalid device ID validation error, got %v", err)
			}
		})
	}
}

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

func TestPrecisionDFConfigDefaultsAndRoundTrip(t *testing.T) {
	var legacy Config
	if err := json.Unmarshal([]byte(`{"precision_df":null}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.PrecisionDF.Enabled || legacy.PrecisionDF.CommandPort != 10087 || legacy.PrecisionDF.DataPort != 10088 || legacy.PrecisionDF.IntervalMS != 200 {
		t.Fatalf("invalid legacy defaults: %+v", legacy.PrecisionDF)
	}
	cfg := DefaultConfig()
	cfg.PrecisionDF.Enabled = true
	cfg.PrecisionDF.BearingDeg = 0
	cfg.PrecisionDF.LevelDBUV = 0
	cfg.PrecisionDF.Quality = 0
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var restored Config
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.PrecisionDF != cfg.PrecisionDF {
		t.Fatalf("config changed on scene round trip: %+v", restored.PrecisionDF)
	}
}

func TestValidatePrecisionDFConfig(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*PrecisionDFConfig)
	}{
		{"same port", func(c *PrecisionDFConfig) { c.DataPort = c.CommandPort }},
		{"invalid port", func(c *PrecisionDFConfig) { c.CommandPort = 65536 }},
		{"too fast", func(c *PrecisionDFConfig) { c.IntervalMS = 19 }},
		{"invalid angle", func(c *PrecisionDFConfig) { c.BearingDeg = 360 }},
		{"nonfinite angle", func(c *PrecisionDFConfig) { c.BearingDeg = math.NaN() }},
		{"nonfinite level", func(c *PrecisionDFConfig) { c.LevelDBUV = math.Inf(1) }},
		{"invalid quality", func(c *PrecisionDFConfig) { c.Quality = 101 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.PrecisionDF.Enabled = true
			tc.mutate(&cfg.PrecisionDF)
			if err := ValidateConfig(cfg); err == nil {
				t.Fatal("invalid precision DF config accepted")
			}
		})
	}
	cfg := DefaultConfig()
	cfg.PrecisionDF.Enabled, cfg.PrecisionDF.BearingDeg = true, -1
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("invalid angle marker should be accepted: %v", err)
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

func TestLegacyDroneCountMigration(t *testing.T) {
	cases := []struct {
		name       string
		detection  string
		detections string
		want       []int
	}{
		{"single device inherits total", `{}`, `[]`, []int{3}},
		{"single device keeps zero", `{"drone_count":0}`, `[]`, []int{0}},
		{"single device keeps count", `{"drone_count":5}`, `[]`, []int{5}},
		{"device list takes precedence", `{"drone_count":99}`, `[{"drone_count":0},{}]`, []int{0, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"drone_count":2,"predefined_drones":[{}],"analysis":{},"detection":` + tc.detection + `,"detections":` + tc.detections + `}`
			var cfg Config
			if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
				t.Fatal(err)
			}
			if cfg.Analysis.DroneCount != 3 {
				t.Errorf("analysis count = %d, want 3", cfg.Analysis.DroneCount)
			}
			if len(cfg.Detections) != len(tc.want) {
				t.Fatalf("detection count = %d, want %d", len(cfg.Detections), len(tc.want))
			}
			for i, want := range tc.want {
				if cfg.Detections[i].DroneCount != want {
					t.Errorf("detections[%d] drone count = %d, want %d", i, cfg.Detections[i].DroneCount, want)
				}
			}
			if cfg.Detection.DroneCount != tc.want[0] {
				t.Errorf("legacy detection drone count = %d, want %d", cfg.Detection.DroneCount, tc.want[0])
			}
		})
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
