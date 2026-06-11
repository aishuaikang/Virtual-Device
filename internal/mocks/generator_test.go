package mocks

import (
	"encoding/binary"
	"strings"
	"testing"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/utils"
)

func TestGenerateDetectionDataSimulatesDirectionInternally(t *testing.T) {
	originalCfg := config.GetConfig()
	defer config.SetGlobalConfig(originalCfg)

	cfg := config.DefaultConfig()
	cfg.PredefinedDrones = []config.PredefinedDrone{
		{
			Serial: "TEST-DIRECTION-001",
			Model:  "Autel EVO II",
			Freq:   2450,
			RSSI:   -65,
			Type:   "AUTO",
		},
	}
	config.SetGlobalConfig(cfg)

	generator := NewMockDataGenerator(1)
	command := &utils.DetectionCommand{
		Action: utils.ActionStart,
		Freq:   2450,
		Switch: 1,
	}

	first := generator.GenerateDetectionData(2000, command).String()
	second := generator.GenerateDetectionData(2000, command).String()

	for _, data := range []string{first, second} {
		if !strings.Contains(data, "device=2000") ||
			!strings.Contains(data, "freq=2450.0") ||
			!strings.Contains(data, "rssi=") ||
			!strings.Contains(data, "gpio=") {
			t.Fatalf("expected internally generated direction sample, got %q", data)
		}
	}

	if first == second {
		t.Fatalf("expected direction samples to vary across GPIO/RSSI values, got identical samples %q", first)
	}
}

func TestGenerateSpectrumFrameMatchesDetectorProtocol(t *testing.T) {
	generator := NewMockDataGenerator(0)
	command := &utils.DetectionCommand{
		Action:    utils.ActionStart,
		FFTSize:   64,
		BandStart: 3600,
		BandStop:  5800,
	}

	frame := generator.GenerateSpectrumFrame(command)
	if len(frame) != 4+64*2 {
		t.Fatalf("expected spectrum frame length 132, got %d", len(frame))
	}

	freqKHz := binary.BigEndian.Uint32(frame[:4])
	if freqKHz != 3600*1000+2320 {
		t.Fatalf("expected frequency header 3602320kHz, got %d", freqKHz)
	}

	for i := 0; i < 64; i++ {
		raw := binary.BigEndian.Uint16(frame[4+i*2 : 6+i*2])
		power := int(raw)/100 - 180
		if power < -105 || power > -30 {
			t.Fatalf("unexpected power at point %d: raw=%d power=%d", i, raw, power)
		}
	}
}

func TestGenerateSpectrumFrameAdvancesLikeRealDetector(t *testing.T) {
	generator := NewMockDataGenerator(0)
	command := &utils.DetectionCommand{
		Action:    utils.ActionStart,
		FFTSize:   64,
		BandStart: 70,
		BandStop:  6000,
	}

	first := generator.GenerateSpectrumFrame(command)
	second := generator.GenerateSpectrumFrame(command)

	firstFreq := binary.BigEndian.Uint32(first[:4])
	secondFreq := binary.BigEndian.Uint32(second[:4])

	if firstFreq != 70*1000+2320 {
		t.Fatalf("expected first spectrum frame to start at 72.32MHz, got %d", firstFreq)
	}
	if secondFreq-firstFreq != 30*1000 {
		t.Fatalf("expected consecutive spectrum frames to advance by 30MHz, got first=%d second=%d", firstFreq, secondFreq)
	}
}
