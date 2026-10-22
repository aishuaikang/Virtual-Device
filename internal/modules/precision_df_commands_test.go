package modules

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
	"time"

	"virtual-device-ui/internal/config"
)

func TestPrecisionDFConfigurationRejectsUnsupportedParameters(t *testing.T) {
	for _, tc := range []struct {
		name, mode, key, value string
	}{
		{"unsupported mode", "", "MODE", "TDOA"},
		{"invalid FFT", "SFDF", "SFDF:CFG", "315000000,80000000,1000,1,0"},
		{"invalid band", "SFDF", "SFDF:CFG", "315000000,12345,1024,1,0"},
		{"invalid frequency", "SFDF", "SFDF:CFG", "NaN,80000000,1024,1,0"},
		{"wrong mode", "DFCAP", "SFDF:CFG", "315000000,80000000,1024,1,0"},
		{"reversed scan", "SCANDF", "SCANDF:CFG", "400000000,315000000,80000000,1024,1,50,0"},
		{"unbounded scan", "SCANDF", "SCANDF:CFG", "9000,40000000000,600,16384,1,50,0"},
		{"IQ sweep without step", "DFCAP", "DF:CFG", "315000000,316000000,0,0,2,100,1000,115200"},
		{"empty IQ", "DFCAP", "DF:CFG", "315000000,315000000,0,0,2,100,0,115200"},
		{"excessive antennas", "DFCAP", "DF:CFG", "315000000,315000000,0,0,256,100,1000,115200"},
		{"out of range attenuation", "SFDF", "RFAT", "31"},
		{"reversed spectrum", "PSCAN2", "PSCAN2:CFG", "400000000,315000000,80000000,1,KAISER,1024,1"},
		{"excessive spectrum points", "PSCAN2", "PSCAN2:CFG", "9000,40000000000,600,1,KAISER,16384,1"},
		{"invalid spectrum FFT", "PSCAN2", "PSCAN2:CFG", "315000000,395000000,80000000,1,KAISER,1000,1"},
		{"invalid spectrum detector count", "PSCAN2", "PSCAN2:CFG", "315000000,395000000,80000000,1,KAISER,1024,0"},
		{"invalid IQ frequency", "FFM", "RFFR", "40000000001"},
		{"too few IQ points", "FFM", "WB:IQ", "7"},
		{"too many IQ points", "FFM", "WB:IQ", "512458753"},
		{"unsupported continuous IQ", "FFM", "WB:IQ:MODE", "CONT"},
		{"unknown command", "SFDF", "UNSUPPORTED", "ON"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newPrecisionDFTask()
			p.mode = tc.mode
			if err := p.configure(tc.key, tc.value); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestPrecisionDFPSCAN2AndFFMConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, mode, business string
		commands             []string
		continuous           bool
	}{
		{"spectrum", "PSCAN2", "1,SPEC", []string{"DETEC MAX", "PSCAN2:CFG 315000001,395000001,80000000,1,KAISER,1024,1"}, true},
		{"single IQ", "FFM", "1,IQ", []string{"RFFR 5800000000", "WB:BAND 50000", "WB:DT IQ", "WB:IQ:MODE SINGLE", "WB:IQ 512458752"}, false},
		{"repeat IQ", "FFM", "1,IQ", []string{"RFFR 315000000", "WB:DT IQ", "WB:IQ:MODE REPEAT", "WB:IQ 8"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newPrecisionDFTask()
			for _, command := range append([]string{"MODE " + tc.mode}, tc.commands...) {
				key, arg, _ := strings.Cut(command, " ")
				if err := p.configure(key, arg); err != nil {
					t.Fatalf("%s: %v", command, err)
				}
			}
			if p.mode != tc.mode || p.business() != tc.business || p.continuous != tc.continuous || !p.configured {
				t.Fatalf("incorrect task configuration: %+v", p)
			}
		})
	}
}

func TestPrecisionDFTimeTriggerInterval(t *testing.T) {
	p := newPrecisionDFTask()
	for _, command := range []string{"TRIG:TYPE TIME", "TRIG:INTERVAL 900000000", "TRIG:SW ON"} {
		key, arg, _ := strings.Cut(command, " ")
		if err := p.configure(key, arg); err != nil {
			t.Fatal(err)
		}
	}
	if got := p.interval(20 * time.Millisecond); got != 900*time.Millisecond {
		t.Fatalf("interval = %s", got)
	}
	if err := p.configure("TRIG:SW", "OFF"); err != nil {
		t.Fatal(err)
	}
	if got := p.interval(20 * time.Millisecond); got != 20*time.Millisecond {
		t.Fatalf("disabled trigger interval = %s", got)
	}
}

func TestPrecisionDFGNSSLayout(t *testing.T) {
	m := &PrecisionDFModule{position: config.GPS{Lat: -31.25, Lng: -121.5}}
	now := time.Unix(1700000000, 0).UTC()
	b := m.gnssPayload(now)
	if len(b) != 40 || binary.LittleEndian.Uint32(b[:4]) != 1 ||
		math.Float64frombits(binary.LittleEndian.Uint64(b[4:12])) != -12130 ||
		math.Float64frombits(binary.LittleEndian.Uint64(b[12:20])) != -3115 ||
		math.Float32frombits(binary.LittleEndian.Uint32(b[20:24])) != 50 ||
		binary.LittleEndian.Uint32(b[32:36]) != 12 || binary.LittleEndian.Uint32(b[36:40]) != 1700000000 {
		t.Fatalf("invalid GNSS payload: %x", b)
	}
	nmea := string(m.nmeaPayload(now))
	if !strings.Contains(nmea, ",3115.0000,S,12130.0000,W,") || !strings.HasSuffix(nmea, "\r\n") {
		t.Fatalf("invalid NMEA: %q", nmea)
	}
}
