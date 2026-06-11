package utils

import "testing"

func TestDetectionCommandParsesBandStartCommand(t *testing.T) {
	raw := DetectionCommandString("st")

	command, err := raw.ParseCommand()
	if err != nil {
		t.Fatalf("ParseCommand returned error: %v", err)
	}

	if command.Action != ActionStart {
		t.Fatalf("expected start action, got %q", command.Action)
	}
}

func TestDetectionCommandParsesLowBandScanCommand(t *testing.T) {
	raw := DetectionCommandString("start -freq 3,-fpv_only 1,-rx2 2 -gain 70,-switch 1")

	command, err := raw.ParseCommand()
	if err != nil {
		t.Fatalf("ParseCommand returned error: %v", err)
	}

	if command.Freq != 3 || command.FPVOnly != 1 || command.Rx2 != 2 || command.Gain != 70 || command.Switch != 1 {
		t.Fatalf("unexpected command: %#v", command)
	}
	if !command.IsBandScanFrequency() {
		t.Fatalf("expected freq=3 to be treated as band scan")
	}
	if command.UsesDirectionData() {
		t.Fatalf("expected low-band scan command to use normal detection data")
	}
}

func TestDetectionCommandParsesDirectionLockCommand(t *testing.T) {
	raw := DetectionCommandString("start -freq 5730.000000 -switch 1")

	command, err := raw.ParseCommand()
	if err != nil {
		t.Fatalf("ParseCommand returned error: %v", err)
	}

	if command.Freq != 5730 || command.Switch != 1 {
		t.Fatalf("unexpected command: %#v", command)
	}
	if !command.UsesDirectionData() {
		t.Fatalf("expected target frequency lock command to use direction data")
	}
}

func TestDetectionCommandParsesFFTAnalysisCommand(t *testing.T) {
	raw := DetectionCommandString("start -fft 64 -band 3600,5800, -gain 40\n")

	command, err := raw.ParseCommand()
	if err != nil {
		t.Fatalf("ParseCommand returned error: %v", err)
	}

	if !command.IsSpectrumAnalysis() {
		t.Fatalf("expected fft command to use spectrum analysis, got %#v", command)
	}
	if command.FFTSize != 64 || command.BandStart != 3600 || command.BandStop != 5800 || command.Gain != 40 {
		t.Fatalf("unexpected fft command: %#v", command)
	}
}

func TestDetectionCommandParsesSpectrumStopCommand(t *testing.T) {
	raw := DetectionCommandString("start -fft 0,\n")

	command, err := raw.ParseCommand()
	if err != nil {
		t.Fatalf("ParseCommand returned error: %v", err)
	}

	if command.Action != ActionStop {
		t.Fatalf("expected fft 0 to stop spectrum analysis, got %#v", command)
	}
}
