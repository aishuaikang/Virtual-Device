//go:build integration

package modules

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/protocol"
)

func startPrecisionDFTest(t *testing.T, bearing float64) *PrecisionDFModule {
	t.Helper()
	cfg := config.DefaultConfig().PrecisionDF
	cfg.Host, cfg.CommandPort, cfg.DataPort, cfg.IntervalMS, cfg.BearingDeg = "127.0.0.1", 0, 0, 20, bearing
	m, err := NewPrecisionDFModule(cfg, config.GPS{Lat: 31.25, Lng: 121.5})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { m.Start(); close(done) }()
	t.Cleanup(func() {
		m.Stop()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("receiver goroutines did not stop")
		}
	})
	return m
}

func waitPrecisionDFTest(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("receiver state did not converge")
}

func connectPrecisionDFTest(t *testing.T, m *PrecisionDFModule) (net.Conn, net.Conn) {
	t.Helper()
	command, err := net.DialTimeout("tcp", m.Snapshot().CommandAddress, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Close() })
	_ = command.SetDeadline(time.Now().Add(5 * time.Second))
	notification, err := protocol.ReadPrecisionDFCommand(command)
	if err != nil || !strings.HasPrefix(notification, "DATA_AT_PORT ") {
		t.Fatalf("invalid data port notification: %q, %v", notification, err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(notification, "DATA_AT_PORT ")))
	if err != nil {
		t.Fatal(err)
	}
	data, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = data.Close() })
	_ = data.SetDeadline(time.Now().Add(5 * time.Second))
	waitPrecisionDFTest(t, func() bool { return m.Snapshot().DataConnected })
	return command, data
}

func precisionDFCommandTest(t *testing.T, conn net.Conn, command string) string {
	t.Helper()
	frame, err := protocol.EncodePrecisionDFCommand(command)
	if err != nil {
		t.Fatal(err)
	}
	if err := writePrecisionDFBytes(conn, frame); err != nil {
		t.Fatal(err)
	}
	response, err := protocol.ReadPrecisionDFCommand(conn)
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Fields(command)[0]
	if !strings.HasPrefix(response, key+" OK\n") {
		t.Fatalf("%s rejected: %q", command, response)
	}
	return response
}

func readPrecisionDFTest(t *testing.T, conn net.Conn) protocol.PrecisionDFFrame {
	t.Helper()
	var h [288]byte
	if _, err := io.ReadFull(conn, h[:]); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(h[:8], []byte{0, 128, 0, 128, 0, 128, 0, 128}) {
		t.Fatalf("invalid data header: %x", h[:8])
	}
	u := binary.LittleEndian.Uint32
	n := u(h[12:16])
	if n > 1<<20 || u(h[16:20]) != 1 {
		t.Fatal("invalid frame size or channel")
	}
	if u(h[28:32]) == 1 && u(h[40:44]) != 16 {
		t.Fatal("IQ sample width is not 16 bits")
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatal(err)
	}
	var tail [32]byte
	if _, err := io.ReadFull(conn, tail[:]); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tail[24:], []byte{255, 127, 255, 127, 255, 127, 255, 127}) {
		t.Fatal("invalid data tail")
	}
	return protocol.PrecisionDFFrame{
		Sequence: u(h[8:12]), Business: u(h[20:24]), DataType: u(h[28:32]),
		BusinessSequence: u(h[44:48]),
		FrequencyHz:      uint64(u(h[32:36])) | uint64(u(h[36:40]))<<32,
		SampleRate:       u(h[68:72]), BandwidthHz: u(h[24:28]), FFTSize: u(h[140:144]), TaskID: u(h[108:112]), Antenna: u(h[132:136]),
		Total: binary.LittleEndian.Uint64(h[92:100]), Index: binary.LittleEndian.Uint64(h[100:108]),
		StartHz: math.Float64frombits(binary.LittleEndian.Uint64(h[184:192])),
		EndHz:   math.Float64frombits(binary.LittleEndian.Uint64(h[192:200])), Payload: payload,
		Timestamp: time.Unix(int64(u(h[80:84])), int64(u(h[84:88]))).UTC(),
	}
}

func TestPrecisionDFPSCAN2(t *testing.T) {
	for _, tc := range []struct {
		name       string
		continuous int
		business   uint32
		sweeps     uint32
	}{
		{"single", 0, 0x90, 1},
		{"continuous", 1, 0x91, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := startPrecisionDFTest(t, 45)
			command, data := connectPrecisionDFTest(t, m)
			for _, cmd := range []string{
				"CLR", "MODE PSCAN2", "RFMO NORMAL", "RFAT 0", "IFAT 0", "DETEC MAX",
				fmt.Sprintf("PSCAN2:CFG 315000001,395000001,80000000,%d,KAISER,1024,1", tc.continuous),
				"GET:GNSS OFF", "GET:RAWGNSS OFF", "START 1,SPEC",
			} {
				precisionDFCommandTest(t, command, cmd)
			}
			for sweep := uint32(1); sweep <= tc.sweeps; sweep++ {
				for offset := uint64(0); offset < 802; {
					f := readPrecisionDFTest(t, data)
					if f.DataType != 2 || f.Business != tc.business || f.TaskID != 1 || f.BusinessSequence != sweep ||
						f.Index != offset || f.Total != 802 || f.StartHz != 315000000 || f.EndHz != 395100000 ||
						f.FFTSize != 1024 || f.SampleRate != 102400000 || len(f.Payload) > 256*2 {
						t.Fatalf("invalid PSCAN2 segment: %+v", f)
					}
					for i := 0; i < len(f.Payload); i += 2 {
						level := float64(int16(binary.LittleEndian.Uint16(f.Payload[i:]))) / 64
						if level < m.cfg.LevelDBUV-45.01 || level > m.cfg.LevelDBUV+0.01 {
							t.Fatalf("invalid 1/64 dBuV level: %f", level)
						}
					}
					offset += uint64(len(f.Payload) / 2)
				}
			}
			if tc.continuous == 0 {
				waitPrecisionDFTest(t, func() bool { return !m.Snapshot().Running })
			}
			precisionDFCommandTest(t, command, "ABOR 1,SPEC")
			if m.Snapshot().Running {
				t.Fatal("PSCAN2 did not stop")
			}
		})
	}
}

func TestPrecisionDFPSCAN2FractionalSampleRate(t *testing.T) {
	for _, bandwidth := range []uint32{1000, 600} {
		t.Run(fmt.Sprint(bandwidth), func(t *testing.T) {
			m := startPrecisionDFTest(t, 45)
			command, data := connectPrecisionDFTest(t, m)
			for _, cmd := range []string{
				"CLR", "MODE PSCAN2",
				fmt.Sprintf("PSCAN2:CFG 100000001,101000001,%d,0,KAISER,8,1", bandwidth),
				"START 1,SPEC",
			} {
				precisionDFCommandTest(t, command, cmd)
			}
			const step = 1562.5 / 8
			const total = 5122
			for offset := uint64(0); offset < total; {
				f := readPrecisionDFTest(t, data)
				if f.Business != 0x90 || f.DataType != 2 || f.BandwidthHz != bandwidth || f.FFTSize != 8 ||
					f.StartHz != 100000000 || f.EndHz != 101000195.3125 || f.Total != total ||
					f.Index != offset || len(f.Payload) != int(min(uint64(256), total-offset))*2 {
					t.Fatalf("invalid fractional-rate scan: business=%x type=%d bandwidth=%d FFT=%d start=%f end=%f total=%d index=%d bytes=%d",
						f.Business, f.DataType, f.BandwidthHz, f.FFTSize, f.StartHz, f.EndHz, f.Total, f.Index, len(f.Payload))
				}
				if math.Abs(float64(f.SampleRate)-1562.5) > 1 {
					t.Fatalf("invalid integer sample rate field: %d", f.SampleRate)
				}
				if math.Abs((f.EndHz-f.StartHz)/step-float64(f.Total-1)) > 0.001 {
					t.Fatal("scan frequency grid is incompatible with the receiver sample rate")
				}
				offset += uint64(len(f.Payload) / 2)
			}
			waitPrecisionDFTest(t, func() bool { return !m.Snapshot().Running })
		})
	}
}

func TestPrecisionDFFixedFrequencyIQ(t *testing.T) {
	for _, tc := range []struct {
		name     string
		business uint32
		packets  uint32
	}{
		{"SINGLE", 0x10, 1},
		{"REPEAT", 0x13, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := startPrecisionDFTest(t, 45)
			command, data := connectPrecisionDFTest(t, m)
			for _, cmd := range []string{
				"CLR", "MODE FFM", "RFMO NORMAL", "RFAT 0", "IFAT 0", "RFFR 5800000000",
				"WB:BAND 50000", "WB:DT IQ", "WB:IQ:MODE " + tc.name, "WB:IQ 4104",
				"TRIG:SW OFF", "GET:GNSS OFF", "GET:RAWGNSS OFF", "START 1,IQ",
			} {
				precisionDFCommandTest(t, command, cmd)
			}
			var previousTimestamp time.Time
			for packet := uint32(1); packet <= tc.packets; packet++ {
				for index, points := range []int{4096, 8} {
					f := readPrecisionDFTest(t, data)
					if f.TaskID != 1 || f.DataType != 1 || f.Business != tc.business || f.BusinessSequence != packet ||
						f.FrequencyHz != 5800000000 || f.Antenna != 0 || f.SampleRate != 100000 || f.BandwidthHz != 50000 || f.Total != 4104 ||
						f.Index != uint64(index*4096) || len(f.Payload) != points*4 {
						t.Fatalf("invalid %s IQ frame: %+v size=%d", tc.name, f, len(f.Payload))
					}
					if packet > 1 && index == 0 && f.Timestamp.Sub(previousTimestamp) < 10*time.Millisecond {
						t.Fatal("REPEAT packets have no simulated acquisition gap")
					}
					previousTimestamp = f.Timestamp
					if binary.LittleEndian.Uint16(f.Payload[:2]) != 8192 || binary.LittleEndian.Uint16(f.Payload[2:4]) != 0 {
						t.Fatal("IQ payload is not little-endian interleaved int16")
					}
				}
			}
			if tc.name == "SINGLE" {
				waitPrecisionDFTest(t, func() bool { return !m.Snapshot().Running })
			}
			precisionDFCommandTest(t, command, "ABOR 1,IQ")
			if m.Snapshot().Running {
				t.Fatal("fixed-frequency IQ did not stop")
			}
		})
	}
}

func TestPrecisionDFFixedFrequencyIQMaximumPointsAndStop(t *testing.T) {
	m := startPrecisionDFTest(t, 45)
	command, data := connectPrecisionDFTest(t, m)
	for _, cmd := range []string{
		"MODE FFM", "RFFR 315000000", "WB:DT IQ", "WB:IQ:MODE SINGLE", "WB:IQ 512458752", "START 1,IQ",
	} {
		precisionDFCommandTest(t, command, cmd)
	}
	f := readPrecisionDFTest(t, data)
	if f.Total != 512458752 || len(f.Payload) != 4096*4 {
		t.Fatalf("large IQ request was not emitted in bounded frames: total=%d bytes=%d", f.Total, len(f.Payload))
	}
	precisionDFCommandTest(t, command, "ABOR 1,IQ")
	_ = data.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	if _, err := io.Copy(io.Discard, data); err == nil {
		t.Fatal("stopped receiver unexpectedly closed the data link")
	} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("draining stopped receiver: %v", err)
	}
	_ = data.SetReadDeadline(time.Now().Add(5 * time.Second))
	for _, cmd := range []string{"CLR", "MODE PSCAN2", "DETEC MAX", "PSCAN2:CFG 315000000,316000000,80000000,0,KAISER,1024,1", "START 1,SPEC"} {
		precisionDFCommandTest(t, command, cmd)
	}
	f = readPrecisionDFTest(t, data)
	if f.TaskID != 2 || f.Business != 0x90 || f.DataType != 2 {
		t.Fatalf("old IQ task leaked into new spectrum task: %+v", f)
	}
}

func TestPrecisionDFSingleAndScan(t *testing.T) {
	for _, tc := range []struct {
		name, cfg string
		dataType  uint32
		points    uint64
		bearing   float64
	}{
		{"SFDF", "SFDF:CFG 5800000000,80000000,1024,0,0", 103, 1, 45},
		{"SCANDF", "SCANDF:CFG 315000000,395000000,80000000,1024,0,50,0", 104, 801, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := startPrecisionDFTest(t, tc.bearing)
			command, data := connectPrecisionDFTest(t, m)
			// Both a split frame and two concatenated frames must be accepted.
			clear, _ := protocol.EncodePrecisionDFCommand("CLR")
			mode, _ := protocol.EncodePrecisionDFCommand("MODE " + tc.name)
			if _, err := command.Write(clear[:3]); err != nil {
				t.Fatal(err)
			}
			if err := writePrecisionDFBytes(command, append(clear[3:], mode...)); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"CLR", "MODE"} {
				if reply, err := protocol.ReadPrecisionDFCommand(command); err != nil || reply != key+" OK\n" {
					t.Fatalf("coalesced %s reply: %q %v", key, reply, err)
				}
			}
			for _, cmd := range []string{tc.cfg, "GET:GNSS ON", "GET:RAWGNSS ON", "START 1,DF"} {
				precisionDFCommandTest(t, command, cmd)
			}
			var count, spectrumPoints uint64
			var sequence uint32
			gnss, nmea := false, false
			for count < tc.points {
				f := readPrecisionDFTest(t, data)
				if f.Sequence <= sequence || f.TaskID != 1 {
					t.Fatalf("invalid sequence/task: %+v", f)
				}
				sequence = f.Sequence
				switch f.DataType {
				case 101:
					gnss = len(f.Payload) == 40
				case 102:
					nmea = strings.HasPrefix(string(f.Payload), "$GNRMC,")
				case 2:
					spectrumPoints += uint64(len(f.Payload) / 2)
				case tc.dataType:
					if tc.dataType == 104 && (f.Index != count || f.Total != tc.points || f.StartHz != 315000000 || f.EndHz != 395000000) {
						t.Fatalf("invalid scan segment: %+v", f)
					}
					if tc.dataType == 103 && f.FrequencyHz != 5800000000 {
						t.Fatalf("frequency lost high bits: %d", f.FrequencyHz)
					}
					for i := 0; i < len(f.Payload); i += 12 {
						angle := math.Float32frombits(binary.LittleEndian.Uint32(f.Payload[i:]))
						if float64(angle) != tc.bearing {
							t.Fatalf("angle = %f", angle)
						}
					}
					count += uint64(len(f.Payload) / 12)
				}
			}
			if !gnss || !nmea || spectrumPoints == 0 {
				t.Fatalf("missing receiver streams: GNSS=%v NMEA=%v spectrum=%d", gnss, nmea, spectrumPoints)
			}
			waitPrecisionDFTest(t, func() bool { return !m.Snapshot().Running })
			precisionDFCommandTest(t, command, "ABOR 1,DF")
		})
	}
}

func TestPrecisionDFIQBothAntennaModes(t *testing.T) {
	for _, mode := range []string{"A", "B"} {
		t.Run(mode, func(t *testing.T) {
			m := startPrecisionDFTest(t, 45)
			command, data := connectPrecisionDFTest(t, m)
			for run := uint32(1); run <= 2; run++ {
				for _, cmd := range []string{"CLR", "MODE DFCAP", "WB:BAND 80000000", "WB:DT IQ", "DF:CFG 315000000,316000000,1000000,0,2,100,4100,115200"} {
					precisionDFCommandTest(t, command, cmd)
				}
				for antenna := 0; antenna < 2; antenna++ {
					cmd := fmt.Sprintf("DF:ALISTS %d", (antenna+1)<<16|antenna)
					if mode == "A" {
						cmd = fmt.Sprintf("DF:ALIST %d,1,16777216,0,0,0,0,0,0,0", antenna+1)
					}
					precisionDFCommandTest(t, command, cmd)
				}
				reply := precisionDFCommandTest(t, command, "START 1,IQ")
				if !strings.Contains(reply, fmt.Sprintf("TASK %d\n", run)) {
					t.Fatalf("invalid task notification: %q", reply)
				}
				var bytesReceived uint64
				for _, frequency := range []uint64{315000000, 316000000} {
					for antenna := uint32(1); antenna <= 2; antenna++ {
						for _, size := range []int{4096 * 4, 4 * 4} {
							f := readPrecisionDFTest(t, data)
							if f.TaskID != run || f.DataType != 1 || f.Business != 0xd0 || f.Antenna != antenna || f.FrequencyHz != frequency || len(f.Payload) != size || f.SampleRate != 102400000 {
								t.Fatalf("invalid IQ frame: %+v size=%d", f, len(f.Payload))
							}
							bytesReceived += uint64(len(f.Payload))
						}
					}
				}
				if bytesReceived != 4100*4*2*2 {
					t.Fatalf("IQ data missing: %d", bytesReceived)
				}
				waitPrecisionDFTest(t, func() bool { return !m.Snapshot().Running })
				precisionDFCommandTest(t, command, "ABOR 1,IQ")
			}
		})
	}
}

func TestPrecisionDFAbortDisconnectAndReconnect(t *testing.T) {
	m := startPrecisionDFTest(t, 45)
	command, data := connectPrecisionDFTest(t, m)
	for _, cmd := range []string{"MODE SFDF", "SFDF:CFG 315000000,80000000,1024,1,0", "START 1,DF"} {
		precisionDFCommandTest(t, command, cmd)
	}
	readPrecisionDFTest(t, data)
	readPrecisionDFTest(t, data)
	precisionDFCommandTest(t, command, "ABOR 1,DF")
	if m.Snapshot().Running {
		t.Fatal("ABOR was acknowledged before stopping")
	}
	_ = data.Close()
	var b [1]byte
	if _, err := command.Read(b[:]); err == nil {
		t.Fatal("data disconnect did not close the command link")
	}
	waitPrecisionDFTest(t, func() bool { return !m.Snapshot().CommandConnected })
	command, data = connectPrecisionDFTest(t, m)
	for _, cmd := range []string{"MODE SFDF", "SFDF:CFG 315000000,80000000,1024,0,0", "START 1,DF"} {
		precisionDFCommandTest(t, command, cmd)
	}
	f := readPrecisionDFTest(t, data)
	if f.TaskID != 2 {
		t.Fatalf("reconnect reused previous task: %d", f.TaskID)
	}
	commandAddress, dataAddress := m.Snapshot().CommandAddress, m.Snapshot().DataAddress
	m.Stop()
	for _, address := range []string{commandAddress, dataAddress} {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatalf("stop left listener open: %v", err)
		}
		_ = listener.Close()
	}
}

func TestPrecisionDFRejectsSecondController(t *testing.T) {
	m := startPrecisionDFTest(t, 45)
	command, _ := connectPrecisionDFTest(t, m)
	other, err := net.DialTimeout("tcp", m.Snapshot().CommandAddress, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	_ = other.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	if _, err := other.Read(b[:]); err != io.EOF {
		t.Fatalf("second controller was not rejected: %v", err)
	}
	precisionDFCommandTest(t, command, "MODE SFDF")
}
