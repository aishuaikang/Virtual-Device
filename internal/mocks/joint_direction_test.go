package mocks

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/utils"
)

func sharedTargets() config.JointDirectionConfig {
	return config.JointDirectionConfig{
		Enabled: true, HopIntervalMS: 1500,
		Targets: []config.JointDirectionTarget{{
			Model: "O4", Frequencies: []float64{5800, 5803, 5798, 5806},
			Faces: []config.JointDirectionSample{
				{DeviceID: 2000, Reporting: true, RSSI: -60},
				{DeviceID: 2001, Reporting: true, RSSI: -40},
				{DeviceID: 2002, Reporting: true, RSSI: -70},
			},
		}},
	}
}

func sampleFields(t *testing.T, packet fmt.Stringer) map[string]string {
	t.Helper()
	if packet == nil {
		t.Fatal("missing sample")
	}
	fields := make(map[string]string)
	for _, part := range strings.Split(strings.TrimSpace(packet.String()), ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			fields[kv[0]] = kv[1]
		}
	}
	if len(fields) != 5 {
		t.Fatalf("samples must use only device/model/freq/rssi/seq, got %v", fields)
	}
	return fields
}

func TestJointDirectionSharesHopsWithNearFrequencies(t *testing.T) {
	settings := sharedTargets()
	epoch := time.Unix(1700000000, 0)
	now := epoch
	var generators []*jointDirectionGenerator
	for face := range settings.Targets[0].Faces {
		g := NewJointDirectionGenerator(nil, NewJointDirectionState(settings), face, epoch).(*jointDirectionGenerator)
		g.now = func() time.Time { return now }
		generators = append(generators, g)
	}
	for hop := 0; hop < 8; hop++ {
		now = epoch.Add(time.Duration(hop) * 1500 * time.Millisecond)
		for face, g := range generators {
			fields := sampleFields(t, g.GenerateDetectionData(2000+face, nil))
			freq, _ := strconv.ParseFloat(fields["freq"], 64)
			rssi, _ := strconv.ParseFloat(fields["rssi"], 64)
			wantFreq := settings.Targets[0].Frequencies[hop%4] + float64(face-3)*0.2
			if fields["model"] != "O4" || fields["device"] != strconv.Itoa(2000+face) || math.Abs(freq-wantFreq) > 0.01 {
				t.Fatalf("hop %d face %d: %v, want frequency %.1f", hop, face, fields, wantFreq)
			}
			if math.Abs(rssi-settings.Targets[0].Faces[face].RSSI) > 0.41 {
				t.Fatalf("face %d RSSI unexpectedly changed: %v", face, rssi)
			}
			if fields["seq"] != strconv.Itoa(hop+1) {
				t.Fatalf("sequence must advance per module: %v", fields)
			}
		}
	}
}

func TestJointDirectionMissingFacesAndMultipleTargets(t *testing.T) {
	settings := sharedTargets()
	settings.Targets[0].Faces[0].Reporting = false
	settings.Targets[0].Faces[2].Reporting = false
	g := NewJointDirectionGenerator(nil, NewJointDirectionState(settings), 1, time.Now())
	for _, deviceID := range []int{2000, 2002, 9999} {
		if g.GenerateDetectionData(deviceID, nil) != nil {
			t.Fatalf("missing face %d must not fabricate a target", deviceID)
		}
	}
	if g.GenerateDetectionData(2001, nil) == nil {
		t.Fatal("single reporting face must still produce data")
	}
	settings.Targets = append(settings.Targets, config.JointDirectionTarget{
		Model: "O4", Frequencies: []float64{5860, 5863},
		Faces: []config.JointDirectionSample{{DeviceID: 2001, Reporting: true, RSSI: -50}},
	})
	g = NewJointDirectionGenerator(nil, NewJointDirectionState(settings), 1, time.Now())
	lines := strings.Split(strings.TrimSpace(g.GenerateDetectionData(2001, nil).String()), "\r\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "freq=5799.6") || !strings.Contains(lines[1], "freq=5859.6") {
		t.Fatalf("every scan round must include both same-model targets: %v", lines)
	}
	for i := range settings.Targets {
		for j := range settings.Targets[i].Faces {
			settings.Targets[i].Faces[j].Reporting = false
		}
	}
	g = NewJointDirectionGenerator(nil, NewJointDirectionState(settings), 1, time.Now())
	if g.GenerateDetectionData(2001, nil) != nil {
		t.Fatal("all stopped targets must not produce NoDrone or zero RSSI")
	}
}

type jointDirectionFallback struct {
	MockDataGenerator
}

func (*jointDirectionFallback) GenerateDetectionData(int, *utils.DetectionCommand) fmt.Stringer {
	return DirectionData("legacy lock")
}

func (*jointDirectionFallback) GenerateSpectrumFrame(*utils.DetectionCommand) []byte {
	return []byte{1, 2, 3}
}

func TestJointDirectionKeepsLegacyLockAndSpectrum(t *testing.T) {
	g := NewJointDirectionGenerator(&jointDirectionFallback{}, NewJointDirectionState(sharedTargets()), 0, time.Now())
	locked := &utils.DetectionCommand{Action: utils.ActionStart, Freq: 5800}
	if g.GenerateDetectionData(2000, locked).String() != "legacy lock" {
		t.Fatal("traditional lock behavior changed")
	}
	for _, freq := range []int{0, 3, 4} {
		cmd := &utils.DetectionCommand{Action: utils.ActionStart, Freq: freq}
		if !strings.Contains(g.GenerateDetectionData(2000, cmd).String(), "model=O4") {
			t.Fatalf("scan type %d must use shared targets", freq)
		}
	}
	if string(g.GenerateSpectrumFrame(locked)) != "\x01\x02\x03" {
		t.Fatal("spectrum generation changed")
	}
}

func TestJointDirectionConcurrentSequences(t *testing.T) {
	g := NewJointDirectionGenerator(nil, NewJointDirectionState(sharedTargets()), 0, time.Now())
	var wg sync.WaitGroup
	results := make(chan string, 100)
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- g.GenerateDetectionData(2000, nil).String()
		}()
	}
	wg.Wait()
	close(results)
	seen := make(map[string]bool)
	for packet := range results {
		fields := sampleFields(t, DirectionData(packet))
		if seen[fields["seq"]] {
			t.Fatalf("duplicate concurrent sequence %s", fields["seq"])
		}
		seen[fields["seq"]] = true
	}
}

func TestJointDirectionLiveSamplesPreserveHopsAndSequences(t *testing.T) {
	settings := sharedTargets()
	state := NewJointDirectionState(settings)
	epoch := time.Unix(1700000000, 0)
	g := NewJointDirectionGenerator(nil, state, 0, epoch).(*jointDirectionGenerator)
	g.now = func() time.Time { return epoch.Add(1600 * time.Millisecond) }
	before := sampleFields(t, g.GenerateDetectionData(2000, nil))
	settings.Targets[0].Faces[0].RSSI = -25
	state.Update(settings)
	// Caller-owned slices must not mutate a snapshot already in use by scan goroutines.
	settings.Targets[0].Faces[0].RSSI = -90
	settings.Targets[0].Frequencies[1] = 2400
	after := sampleFields(t, g.GenerateDetectionData(2000, nil))
	rssi, _ := strconv.ParseFloat(after["rssi"], 64)
	if math.Abs(rssi-(-25)) > 0.41 || after["freq"] != before["freq"] || after["seq"] != "2" {
		t.Fatalf("live update reset scan state or missed new RSSI: before=%v after=%v", before, after)
	}
	settings.Targets[0].Faces[0].Reporting = false
	state.Update(settings)
	if g.GenerateDetectionData(2000, nil) != nil {
		t.Fatal("live missing-face update still reported a target")
	}
}

func TestJointDirectionConcurrentLiveUpdates(t *testing.T) {
	state := NewJointDirectionState(sharedTargets())
	g := NewJointDirectionGenerator(nil, state, 0, time.Now())
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				g.GenerateDetectionData(2000, nil)
			}
		}()
	}
	for i := range 1000 {
		settings := sharedTargets()
		settings.Targets[0].Faces[0].RSSI = -40 - float64(i%30)
		state.Update(settings)
	}
	wg.Wait()
	if g.GenerateDetectionData(2000, nil) == nil {
		t.Fatal("concurrent updates lost the reporting face")
	}
}
