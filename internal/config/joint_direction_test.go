package config

import (
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

func jointDirectionTestConfig() *Config {
	cfg := DefaultConfig()
	cfg.JointDirection = JointDirectionConfig{
		Enabled: true, HopIntervalMS: 1500,
		Targets: []JointDirectionTarget{{
			Model: "O4", Frequencies: []float64{5800, 5803, 5798, 5806},
			Faces: []JointDirectionSample{
				{DeviceID: 2000, Reporting: true, RSSI: -60},
				{DeviceID: 2001, Reporting: true, RSSI: -40},
				{DeviceID: 2002, Reporting: false, RSSI: -70},
			},
		}},
	}
	return cfg
}

func TestJointDirectionConfigRoundTrip(t *testing.T) {
	original := GetConfig()
	t.Cleanup(func() { SetGlobalConfig(original) })
	cfg := jointDirectionTestConfig()
	path := filepath.Join(t.TempDir(), "scenario.json")
	if err := SaveConfig(cfg, path); err != nil {
		t.Fatal(err)
	}
	restored, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.JointDirection, cfg.JointDirection) {
		t.Fatalf("shared targets changed after saving: %+v", restored.JointDirection)
	}
	for _, raw := range []string{`{}`, `{"joint_direction":null}`} {
		if err := json.Unmarshal([]byte(raw), &restored); err != nil {
			t.Fatal(err)
		}
		if restored.JointDirection.Enabled || restored.JointDirection.HopIntervalMS != 1500 {
			t.Fatalf("old scenes must keep random detection: %+v", restored.JointDirection)
		}
	}
}

func TestJointDirectionConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Config)
	}{
		{"missing targets", func(c *Config) { c.JointDirection.Targets = nil }},
		{"zero hop interval", func(c *Config) { c.JointDirection.HopIntervalMS = 0 }},
		{"long hop interval", func(c *Config) { c.JointDirection.HopIntervalMS = 60001 }},
		{"empty model", func(c *Config) { c.JointDirection.Targets[0].Model = "  " }},
		{"injected fields", func(c *Config) { c.JointDirection.Targets[0].Model = "O4, freq=2000" }},
		{"missing frequencies", func(c *Config) { c.JointDirection.Targets[0].Frequencies = nil }},
		{"negative frequency", func(c *Config) { c.JointDirection.Targets[0].Frequencies[0] = -1 }},
		{"nan frequency", func(c *Config) { c.JointDirection.Targets[0].Frequencies[0] = math.NaN() }},
		{"infinite rssi", func(c *Config) { c.JointDirection.Targets[0].Faces[0].RSSI = math.Inf(1) }},
		{"out of range rssi", func(c *Config) { c.JointDirection.Targets[0].Faces[0].RSSI = -141 }},
		{"unknown hardware id", func(c *Config) { c.JointDirection.Targets[0].Faces[0].DeviceID = 9999 }},
		{"duplicate face", func(c *Config) { c.JointDirection.Targets[0].Faces[1].DeviceID = 2000 }},
		{"duplicate detector id", func(c *Config) { c.Detections[1].DeviceID = 2000 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := jointDirectionTestConfig()
			tc.mutate(cfg)
			if err := ValidateConfig(cfg); err == nil {
				t.Fatal("invalid joint direction configuration accepted")
			}
		})
	}

	cfg := jointDirectionTestConfig()
	for i := range cfg.JointDirection.Targets[0].Faces {
		cfg.JointDirection.Targets[0].Faces[i].Reporting = false
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("all faces may stop reporting while keeping heartbeats: %v", err)
	}
	cfg.JointDirection.Enabled = false
	cfg.JointDirection.Targets = nil
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("disabled scenario must not affect normal mode: %v", err)
	}
}

func TestJointDirectionNormalizesFacesForLiveUpdates(t *testing.T) {
	for _, tc := range []struct {
		name  string
		faces []JointDirectionSample
	}{
		{"missing face", []JointDirectionSample{
			{DeviceID: 2000, Reporting: true, RSSI: -60},
			{DeviceID: 2001, Reporting: true, RSSI: -40},
		}},
		{"reordered faces", []JointDirectionSample{
			{DeviceID: 2002, Reporting: false, RSSI: -70},
			{DeviceID: 2001, Reporting: true, RSSI: -40},
			{DeviceID: 2000, Reporting: true, RSSI: -60},
		}},
		{"all faces missing", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current := jointDirectionTestConfig()
			current.JointDirection.Targets[0].Faces = tc.faces
			if err := ValidateConfig(current); err != nil {
				t.Fatal(err)
			}
			faces := append([]JointDirectionSample(nil), current.JointDirection.Targets[0].Faces...)
			if len(faces) != len(current.Detections) {
				t.Fatalf("normalized faces = %v, want every configured detector", faces)
			}
			for i, detector := range current.Detections {
				want := JointDirectionSample{DeviceID: detector.DeviceID, RSSI: -70}
				for _, original := range tc.faces {
					if original.DeviceID == detector.DeviceID {
						want = original
					}
				}
				if faces[i] != want {
					t.Fatalf("face[%d] = %+v, want %+v", i, faces[i], want)
				}
			}
			// Match the editor: send every detector in module order, then enable a face.
			next := current.JointDirection
			next.Targets = append([]JointDirectionTarget(nil), next.Targets...)
			next.Targets[0].Faces = append([]JointDirectionSample(nil), faces...)
			next.Targets[0].Faces[2].Reporting = true
			next.Targets[0].Faces[2].RSSI = -25
			if err := ValidateJointDirectionUpdate(current.JointDirection, next); err != nil {
				t.Fatalf("live RSSI/reporting update rejected: %v", err)
			}
			NormalizeConfig(current)
			if !reflect.DeepEqual(current.JointDirection.Targets[0].Faces, faces) {
				t.Fatal("normalizing twice changed the samples")
			}
		})
	}
}

func TestJointDirectionLiveUpdateValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*JointDirectionConfig)
		wantErr bool
	}{
		{"rssi and reporting", func(c *JointDirectionConfig) {
			c.Targets[0].Faces[0].RSSI = -25
			c.Targets[0].Faces[2].Reporting = true
		}, false},
		{"minimum rssi", func(c *JointDirectionConfig) { c.Targets[0].Faces[0].RSSI = -140 }, false},
		{"maximum rssi", func(c *JointDirectionConfig) { c.Targets[0].Faces[0].RSSI = 0 }, false},
		{"invalid rssi", func(c *JointDirectionConfig) { c.Targets[0].Faces[0].RSSI = -141 }, true},
		{"nonfinite rssi", func(c *JointDirectionConfig) { c.Targets[0].Faces[0].RSSI = math.NaN() }, true},
		{"mode switch", func(c *JointDirectionConfig) { c.Enabled = false }, true},
		{"hop interval", func(c *JointDirectionConfig) { c.HopIntervalMS++ }, true},
		{"target count", func(c *JointDirectionConfig) { c.Targets = nil }, true},
		{"model", func(c *JointDirectionConfig) { c.Targets[0].Model = "O3" }, true},
		{"frequency", func(c *JointDirectionConfig) { c.Targets[0].Frequencies[0]++ }, true},
		{"face count", func(c *JointDirectionConfig) { c.Targets[0].Faces = nil }, true},
		{"hardware id", func(c *JointDirectionConfig) { c.Targets[0].Faces[0].DeviceID = 9999 }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current := jointDirectionTestConfig().JointDirection
			next := jointDirectionTestConfig().JointDirection
			tc.mutate(&next)
			if err := ValidateJointDirectionUpdate(current, next); (err != nil) != tc.wantErr {
				t.Fatalf("update error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
