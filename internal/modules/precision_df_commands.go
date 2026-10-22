package modules

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"virtual-device-ui/internal/protocol"
)

type precisionDFTask struct {
	mode                         string
	frequency, start, end        uint64
	bandwidth, fftSize           uint32
	continuous                   bool
	threshold                    float64
	antennaCount, iqPoints       uint32
	step                         uint64
	antennaMode                  string
	antennas                     map[uint32]bool
	rfMode                       string
	rfAttenuation, ifAttenuation uint32
	trigger, gnss, rawGNSS       bool
	triggerType                  string
	intervalNS                   uint64
	configured                   bool
	iqMode                       string
	scanStart, scanEnd           float64
}

func newPrecisionDFTask() precisionDFTask {
	return precisionDFTask{bandwidth: 80000000, rfMode: "NORMAL", iqMode: "SINGLE", antennas: make(map[uint32]bool)}
}

func precisionDFNumbers(arg string, count int) ([]float64, error) {
	parts := strings.Split(arg, ",")
	if len(parts) != count {
		return nil, fmt.Errorf("expected %d parameters", count)
	}
	values := make([]float64, count)
	for i, part := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("invalid numeric parameter")
		}
		values[i] = v
	}
	return values, nil
}

func precisionDFInteger(v, min, max float64) bool {
	return v >= min && v <= max && math.Trunc(v) == v
}

// configure only acknowledges the commands supported by the integrated receiver client.
func (p *precisionDFTask) configure(key, arg string) error {
	invalid := fmt.Errorf("invalid %s parameters", key)
	switch key {
	case "MODE":
		if arg != "SFDF" && arg != "SCANDF" && arg != "DFCAP" && arg != "PSCAN2" && arg != "FFM" {
			return invalid
		}
		*p = newPrecisionDFTask()
		p.mode = arg
	case "RFMO":
		if arg != "NORMAL" && arg != "LOWNOISE" && arg != "LOWDIST" {
			return invalid
		}
		p.rfMode = arg
	case "RFAT", "IFAT":
		n, err := strconv.ParseUint(arg, 10, 32)
		if err != nil || n > 30 {
			return invalid
		}
		if key == "RFAT" {
			p.rfAttenuation = uint32(n)
		} else {
			p.ifAttenuation = uint32(n)
		}
	case "SFDF:CFG", "SCANDF:CFG":
		count, bandIndex := 5, 1
		if key == "SCANDF:CFG" {
			count, bandIndex = 7, 2
		}
		v, err := precisionDFNumbers(arg, count)
		if err != nil || p.mode+":CFG" != key {
			return invalid
		}
		if !precisionDFInteger(v[0], 9000, 40000000000) || !precisionDFInteger(v[bandIndex], 600, 320000000) {
			return invalid
		}
		band := uint32(v[bandIndex])
		if protocol.PrecisionDFSampleRate(band) == 0 || !precisionDFInteger(v[bandIndex+1], 8, 16384) {
			return invalid
		}
		fft := uint32(v[bandIndex+1])
		if fft&(fft-1) != 0 || !precisionDFInteger(v[bandIndex+2], 0, 1) || !precisionDFInteger(v[count-1], 0, 1) {
			return invalid
		}
		if key == "SCANDF:CFG" {
			if !precisionDFInteger(v[1], v[0], 40000000000) || v[5] < -100 || v[5] > 200 {
				return invalid
			}
			bin := protocol.PrecisionDFSampleRate(band) / float64(fft)
			if math.Ceil((v[1]-v[0])/bin)+1 > 1000000 {
				return fmt.Errorf("scan exceeds simulator limit of 1000000 points")
			}
			p.start, p.end, p.threshold = uint64(v[0]), uint64(v[1]), v[5]
		}
		p.frequency, p.bandwidth, p.fftSize = uint64(v[0]), band, fft
		p.continuous, p.configured = v[bandIndex+2] == 1, true
	case "DETEC":
		if arg != "MAX" {
			return invalid
		}
	case "PSCAN2:CFG":
		parts := strings.Split(arg, ",")
		if p.mode != "PSCAN2" || len(parts) != 7 || strings.TrimSpace(parts[4]) != "KAISER" {
			return invalid
		}
		v, err := precisionDFNumbers(strings.Join(append(parts[:4], parts[5:]...), ","), 6)
		if err != nil || !precisionDFInteger(v[0], 9000, 40000000000) || !precisionDFInteger(v[1], v[0]+1, 40000000000) ||
			!precisionDFInteger(v[2], 600, 320000000) || !precisionDFInteger(v[3], 0, 1) ||
			!precisionDFInteger(v[4], 8, 16384) || !precisionDFInteger(v[5], 1, math.MaxUint32) {
			return invalid
		}
		band, fft := uint32(v[2]), uint32(v[4])
		rate := protocol.PrecisionDFSampleRate(band)
		if rate == 0 || fft&(fft-1) != 0 {
			return invalid
		}
		bin := rate / float64(fft)
		start, end := math.Floor(v[0]/bin)*bin, math.Ceil(v[1]/bin)*bin
		if math.Round((end-start)/bin)+1 > 1000000 {
			return fmt.Errorf("scan exceeds simulator limit of 1000000 points")
		}
		p.start, p.end, p.scanStart, p.scanEnd = uint64(v[0]), uint64(v[1]), start, end
		p.bandwidth, p.fftSize, p.continuous, p.configured = band, fft, v[3] == 1, true
	case "RFFR":
		n, err := strconv.ParseUint(arg, 10, 64)
		if err != nil || p.mode != "FFM" || n < 9000 || n > 40000000000 {
			return invalid
		}
		p.frequency = n
	case "WB:BAND":
		n, err := strconv.ParseUint(arg, 10, 32)
		if err != nil || protocol.PrecisionDFSampleRate(uint32(n)) == 0 {
			return invalid
		}
		p.bandwidth = uint32(n)
	case "WB:DT":
		if arg != "IQ" {
			return invalid
		}
	case "WB:IQ:MODE":
		if p.mode != "FFM" || (arg != "SINGLE" && arg != "REPEAT") {
			return invalid
		}
		p.iqMode, p.continuous = arg, arg == "REPEAT"
	case "WB:IQ":
		n, err := strconv.ParseUint(arg, 10, 32)
		if err != nil || p.mode != "FFM" || n < 8 || n > 512458752 {
			return invalid
		}
		p.iqPoints, p.configured = uint32(n), true
	case "DF:CFG":
		v, err := precisionDFNumbers(arg, 8)
		if err != nil || p.mode != "DFCAP" {
			return invalid
		}
		for i, n := range v {
			max := float64(math.MaxUint32)
			if i < 2 {
				max = 40000000000
			}
			if !precisionDFInteger(n, 0, max) {
				return invalid
			}
		}
		if v[0] < 9000 || v[1] < v[0] || (v[1] > v[0] && v[2] == 0) || v[3] > 1 || v[4] < 1 || v[4] > 255 || v[6] < 1 || v[7] < 50 || v[7] > 10000000 {
			return invalid
		}
		p.start, p.end, p.step = uint64(v[0]), uint64(v[1]), uint64(v[2])
		p.continuous, p.antennaCount, p.iqPoints = v[3] == 1, uint32(v[4]), uint32(v[6])
		p.antennas, p.antennaMode, p.configured = make(map[uint32]bool), "", true
	case "DF:ALIST":
		v, err := precisionDFNumbers(arg, 10)
		if err != nil || p.mode != "DFCAP" || !p.configured {
			return invalid
		}
		for _, n := range v {
			if !precisionDFInteger(n, 0, math.MaxUint32) {
				return invalid
			}
		}
		if v[0] < 1 || v[0] > float64(p.antennaCount) || v[1] < 1 || v[1] > 31 || p.antennaMode == "DFCAP_B" {
			return invalid
		}
		p.antennas[uint32(v[0])] = true
		p.antennaMode = "DFCAP_A"
	case "DF:ALISTS":
		n, err := strconv.ParseUint(arg, 10, 32)
		index := uint32(n) & 0xffff
		if err != nil || p.mode != "DFCAP" || !p.configured || index >= p.antennaCount || p.antennaMode == "DFCAP_A" {
			return invalid
		}
		p.antennas[index+1] = true
		p.antennaMode = "DFCAP_B"
	case "TRIG:SW", "GET:GNSS", "GET:RAWGNSS":
		if arg != "ON" && arg != "OFF" {
			return invalid
		}
		switch key {
		case "TRIG:SW":
			p.trigger = arg == "ON"
		case "GET:GNSS":
			p.gnss = arg == "ON"
		case "GET:RAWGNSS":
			p.rawGNSS = arg == "ON"
		}
	case "TRIG:TYPE":
		if arg != "LEV" && arg != "GPIO" && arg != "TIME" {
			return invalid
		}
		p.triggerType = arg
	case "TRIG:CON":
		if arg != "RISE" && arg != "FALL" && arg != "HIGH" && arg != "LOW" {
			return invalid
		}
	case "TRIG:POS", "TRIG:EDGE:HDT", "TRIG:INTERVAL":
		n, err := strconv.ParseUint(arg, 10, 32)
		if err != nil || n > 1000000000 || (key == "TRIG:POS" && n > 2000) {
			return invalid
		}
		if key == "TRIG:INTERVAL" {
			p.intervalNS = n
		}
	case "TRIG:LEVEL":
		v, err := precisionDFNumbers(arg, 1)
		if err != nil || v[0] < -200 || v[0] > 200 {
			return invalid
		}
	default:
		return fmt.Errorf("unsupported receiver command: %s", key)
	}
	return nil
}

func (p precisionDFTask) business() string {
	if p.mode == "DFCAP" || p.mode == "FFM" {
		return "1,IQ"
	}
	if p.mode == "PSCAN2" {
		return "1,SPEC"
	}
	return "1,DF"
}

func (p precisionDFTask) interval(minimum time.Duration) time.Duration {
	if p.trigger && p.triggerType == "TIME" && time.Duration(p.intervalNS) > minimum {
		return time.Duration(p.intervalNS)
	}
	return minimum
}
