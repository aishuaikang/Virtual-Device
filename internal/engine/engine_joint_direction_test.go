package engine

import (
	"encoding/json"
	"math"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"virtual-device-ui/internal/config"
)

func jointClient(t *testing.T, port int) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	server := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}
	// Start() launches listeners asynchronously; retry the same idempotent scan command.
	deadline := time.Now().Add(3 * time.Second)
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		if _, err := conn.WriteToUDP([]byte("st -freq 3"), server); err != nil {
			t.Fatal(err)
		}
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		if n, _, err := conn.ReadFromUDP(buf); err == nil && n > 0 {
			return conn
		}
	}
	t.Fatal("detector listener did not accept scan command")
	return nil
}

func jointRead(t *testing.T, conn *net.UDPConn) []byte {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	return buf[:n]
}

func jointWaitTargets(t *testing.T, conn *net.UDPConn, id int) {
	t.Helper()
	seen := make(map[int]bool)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(seen) < 2 {
		packet := string(jointRead(t, conn))
		if !strings.Contains(packet, "model=O4") {
			continue
		}
		if strings.Count(packet, "device=") != 1 || !strings.Contains(packet, "device="+strconv.Itoa(id)+",") {
			t.Fatalf("each datagram must carry one target from the correct hardware ID: %q", packet)
		}
		for _, field := range strings.Split(packet, ",") {
			field = strings.TrimSpace(field)
			if !strings.HasPrefix(field, "freq=") {
				continue
			}
			freq, err := strconv.ParseFloat(strings.TrimPrefix(field, "freq="), 64)
			if err != nil {
				t.Fatal(err)
			}
			if freq > 5790 && freq < 5820 {
				seen[5800] = true
			} else if freq > 5850 && freq < 5880 {
				seen[5860] = true
			} else {
				t.Fatalf("unexpected random target leaked into joint mode: %q", packet)
			}
		}
	}
	if len(seen) != 2 {
		t.Fatalf("both same-model targets must arrive as separate packets: %v", seen)
	}
}

func TestJointDirectionUDPScenario(t *testing.T) {
	original := config.GetConfig()
	t.Cleanup(func() { config.SetGlobalConfig(original) })
	cfg := config.DefaultConfig()
	cfg.Analysis.Enabled, cfg.FPV.Enabled, cfg.Jamming.Enabled = false, false, false
	cfg.DirectedStrike.Enabled, cfg.PrecisionDF.Enabled = false, false
	cfg.Relay16.Enabled = false
	cfg.MinPushSpeed, cfg.MaxPushSpeed = 20, 20
	cfg.RandomDroneRefreshInterval = 0
	ports := make(map[int]bool)
	for i := range cfg.Detections {
		port := reserveUDPPort(t)
		for ports[port] {
			port = reserveUDPPort(t)
		}
		ports[port] = true
		cfg.Detections[i].Host, cfg.Detections[i].Port = "127.0.0.1", port
		cfg.Detections[i].HeartbeatInterval = 1
	}
	cfg.JointDirection = config.JointDirectionConfig{
		Enabled: true, HopIntervalMS: 200,
		Targets: []config.JointDirectionTarget{
			{Model: "O4", Frequencies: []float64{5800, 5803, 5798}, Faces: []config.JointDirectionSample{
				{DeviceID: 2000, Reporting: true, RSSI: -60},
				{DeviceID: 2001, Reporting: true, RSSI: -40},
				{DeviceID: 2002, Reporting: false, RSSI: -70},
			}},
			{Model: "O4", Frequencies: []float64{5860, 5863}, Faces: []config.JointDirectionSample{
				{DeviceID: 2001, Reporting: true, RSSI: -60},
				{DeviceID: 2000, Reporting: true, RSSI: -50},
			}},
		},
	}
	if err := config.ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	e := New()
	if err := e.Start(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Stop)
	clients := make([]*net.UDPConn, len(cfg.Detections))
	for i, detector := range cfg.Detections {
		clients[i] = jointClient(t, detector.Port)
	}
	jointWaitTargets(t, clients[0], 2000)
	jointWaitTargets(t, clients[1], 2001)

	for {
		packet := string(jointRead(t, clients[2]))
		if strings.Contains(packet, "model=") {
			t.Fatalf("missing face fabricated a target: %q", packet)
		}
		if strings.Contains(packet, "Heart_Beat_#=") {
			break
		}
	}

	server := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: cfg.Detections[1].Port}
	if _, err := clients[1].WriteToUDP([]byte("start -fft 64 -band 3600 5800"), server); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	foundSpectrum := false
	for time.Now().Before(deadline) {
		if packet := jointRead(t, clients[1]); len(packet) == 4+64*2 {
			foundSpectrum = true
			break
		}
	}
	if !foundSpectrum {
		t.Fatal("joint mode blocked spectrum analysis")
	}
	for range 3 {
		if packet := string(jointRead(t, clients[1])); strings.Contains(packet, "model=") {
			t.Fatalf("spectrum-occupied face still reported a target: %q", packet)
		}
	}
	jointWaitTargets(t, clients[0], 2000)
	if _, err := clients[1].WriteToUDP([]byte("st -freq 3"), server); err != nil {
		t.Fatal(err)
	}
	jointWaitTargets(t, clients[1], 2001)
	var updated config.JointDirectionConfig
	encoded, err := json.Marshal(cfg.JointDirection)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &updated); err != nil {
		t.Fatal(err)
	}
	updated.Targets[0].Faces[0].RSSI = -22
	updated.Targets[0].Faces[1].RSSI = -80
	updated.Targets[0].Faces[2].RSSI = -32
	updated.Targets[0].Faces[2].Reporting = true
	updated.Targets[1].Faces[2].RSSI = -28
	updated.Targets[1].Faces[2].Reporting = true
	if err := e.UpdateJointDirection(updated); err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		face int
		freq float64
		rssi float64
	}{{0, 5800, -22}, {1, 5800, -80}, {2, 5800, -32}, {2, 5860, -28}} {
		deadline := time.Now().Add(2 * time.Second)
		matched := false
		for time.Now().Before(deadline) {
			fields := make(map[string]string)
			for _, part := range strings.Split(string(jointRead(t, clients[sample.face])), ",") {
				if kv := strings.SplitN(strings.TrimSpace(part), "=", 2); len(kv) == 2 {
					fields[kv[0]] = kv[1]
				}
			}
			freq, _ := strconv.ParseFloat(fields["freq"], 64)
			rssi, _ := strconv.ParseFloat(fields["rssi"], 64)
			if freq > sample.freq-10 && freq < sample.freq+20 && math.Abs(rssi-sample.rssi) <= 0.41 {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("face %d did not use live RSSI %.1f at %.1f MHz on its existing UDP connection", sample.face, sample.rssi, sample.freq)
		}
	}
	updated.Targets[0].Faces[0].RSSI = -141
	if err := e.UpdateJointDirection(updated); err == nil {
		t.Fatal("invalid live sample accepted")
	}
	if got := e.jointDirection.Snapshot().Targets[0].Faces[0].RSSI; got != -22 {
		t.Fatalf("rejected update replaced live samples: %v", got)
	}
	if !e.IsRunning() {
		t.Fatal("sample updates stopped the engine")
	}
}
