package modules

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"virtual-device-ui/internal/protocol"
)

var errPrecisionDFStopped = errors.New("receiver acquisition stopped")

type precisionDFCursor struct {
	taskID, sequence, sweep uint32
	frequency               uint64
	antenna, offset         uint32
	lastEmit, lastGNSS      time.Time
}

func (m *PrecisionDFModule) stream(s *precisionDFSession) {
	ticker := time.NewTicker(time.Duration(m.cfg.IntervalMS) * time.Millisecond)
	defer ticker.Stop()
	gnssTicker := time.NewTicker(time.Second)
	defer gnssTicker.Stop()
	var cursor precisionDFCursor
	for {
		acquire := false
		select {
		case <-m.stop:
			return
		case <-s.done:
			return
		case <-ticker.C:
			acquire = true
		case <-gnssTicker.C:
		case <-s.wake:
			acquire = true
		}
		if err := m.emit(s, &cursor, acquire); err != nil && !errors.Is(err, errPrecisionDFStopped) {
			select {
			case <-s.done:
			default:
				m.recordError(err)
			}
			return
		}
	}
}

func (m *PrecisionDFModule) emit(s *precisionDFSession, cursor *precisionDFCursor, acquire bool) error {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	m.mu.Lock()
	p, taskID, running := s.task, s.taskID, s.running
	active := m.session == s
	m.mu.Unlock()
	if !active {
		return errPrecisionDFStopped
	}
	now := time.Now().UTC()
	if cursor.taskID != taskID {
		cursor.taskID, cursor.frequency, cursor.antenna, cursor.offset = taskID, p.start, 1, 0
		cursor.lastEmit, cursor.sweep = time.Time{}, 1
	}
	send := func(f protocol.PrecisionDFFrame) error {
		m.mu.Lock()
		allowed := m.session == s && (f.DataType == 101 || f.DataType == 102 || (s.running && s.taskID == taskID))
		m.mu.Unlock()
		if !allowed {
			return errPrecisionDFStopped
		}
		cursor.sequence++
		f.Sequence, f.TaskID, f.Timestamp = cursor.sequence, taskID, now
		_ = s.data.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := writePrecisionDFBytes(s.data, protocol.EncodePrecisionDFFrame(f)); err != nil {
			return err
		}
		m.mu.Lock()
		m.status.SentFrames++
		if f.DataType == 1 {
			m.status.IQBytes += uint64(len(f.Payload))
		}
		m.status.LastActivityAt = now.Format(time.RFC3339)
		m.mu.Unlock()
		return nil
	}
	if (p.gnss || p.rawGNSS) && now.Sub(cursor.lastGNSS) >= time.Second {
		if p.gnss {
			if err := send(protocol.PrecisionDFFrame{DataType: 101, Payload: m.gnssPayload(now)}); err != nil {
				return err
			}
		}
		if p.rawGNSS {
			if err := send(protocol.PrecisionDFFrame{DataType: 102, Payload: m.nmeaPayload(now)}); err != nil {
				return err
			}
		}
		cursor.lastGNSS = now
	}
	if !acquire || !running || now.Sub(cursor.lastEmit) < p.interval(time.Duration(m.cfg.IntervalMS)*time.Millisecond) {
		return nil
	}
	cursor.lastEmit = now
	sampleRate := protocol.PrecisionDFSampleRate(p.bandwidth)
	f := protocol.PrecisionDFFrame{
		BandwidthHz: p.bandwidth, SampleRate: uint32(sampleRate),
		FFTSize: p.fftSize, BusinessSequence: cursor.sweep,
	}
	complete := false
	switch p.mode {
	case "SFDF":
		f.Business, f.FrequencyHz = 0xd2, p.frequency
		if p.continuous {
			f.Business++
		}
		f.DataType, f.Total = 2, uint64(p.fftSize)
		f.Payload = m.spectrum(p, 0, f.Total, f.Total)
		if err := send(f); err != nil {
			return err
		}
		f.DataType, f.Total, f.Payload = 103, 1, m.bearing(p, m.level(p, 0.5))
		if err := send(f); err != nil {
			return err
		}
		complete = true
	case "SCANDF":
		f.Business, f.StartHz, f.EndHz = 0xd4, float64(p.start), float64(p.end)
		if p.continuous {
			f.Business++
		}
		bin := sampleRate / float64(p.fftSize)
		f.Total = uint64(math.Ceil(float64(p.end-p.start)/bin)) + 1
		for offset := uint64(0); offset < f.Total; offset += 256 {
			count := min(uint64(256), f.Total-offset)
			f.Index, f.DataType = offset, 2
			f.Payload = m.spectrum(p, offset, count, f.Total)
			if err := send(f); err != nil {
				return err
			}
			f.DataType, f.Payload = 104, make([]byte, 0, count*12)
			for i := uint64(0); i < count; i++ {
				position := float64(offset+i) / float64(max(uint64(1), f.Total-1))
				f.Payload = append(f.Payload, m.bearing(p, m.level(p, position))...)
			}
			if err := send(f); err != nil {
				return err
			}
		}
		complete = true
	case "PSCAN2":
		f.Business, f.DataType, f.StartHz, f.EndHz = 0x90, 2, p.scanStart, p.scanEnd
		if p.continuous {
			f.Business++
		}
		bin := sampleRate / float64(p.fftSize)
		f.Total = uint64(math.Round((p.scanEnd-p.scanStart)/bin)) + 1
		for offset := uint64(0); offset < f.Total; offset += 256 {
			count := min(uint64(256), f.Total-offset)
			f.Index, f.Payload = offset, m.spectrum(p, offset, count, f.Total)
			if err := send(f); err != nil {
				return err
			}
		}
		complete = true
	case "FFM":
		f.Business, f.DataType, f.FrequencyHz = 0x10, 1, p.frequency
		if p.iqMode == "REPEAT" {
			f.Business = 0x13
		}
		f.Total, f.Index = uint64(p.iqPoints), uint64(cursor.offset)
		count := min(uint32(4096), p.iqPoints-cursor.offset)
		f.Payload = make([]byte, int(count)*4)
		for i := uint32(0); i < count; i++ {
			phase := 2 * math.Pi * float64((cursor.offset+i)%64) / 64
			binary.LittleEndian.PutUint16(f.Payload[i*4:], uint16(int16(8192*math.Cos(phase))))
			binary.LittleEndian.PutUint16(f.Payload[i*4+2:], uint16(int16(8192*math.Sin(phase))))
		}
		if err := send(f); err != nil {
			return err
		}
		cursor.offset += count
		if cursor.offset == p.iqPoints {
			cursor.offset, complete = 0, true
		}
	case "DFCAP":
		f.Business, f.DataType, f.FrequencyHz = 0xd0, 1, cursor.frequency
		if p.continuous {
			f.Business++
		}
		f.Antenna, f.Total, f.Index = cursor.antenna, uint64(p.iqPoints), uint64(cursor.offset)
		count := min(uint32(4096), p.iqPoints-cursor.offset)
		f.Payload = make([]byte, int(count)*4)
		for i := uint32(0); i < count; i++ {
			phase := 2*math.Pi*float64((cursor.offset+i)%64)/64 + float64(cursor.antenna-1)*0.25
			binary.LittleEndian.PutUint16(f.Payload[i*4:], uint16(int16(8192*math.Cos(phase))))
			binary.LittleEndian.PutUint16(f.Payload[i*4+2:], uint16(int16(8192*math.Sin(phase))))
		}
		if err := send(f); err != nil {
			return err
		}
		cursor.offset += count
		if cursor.offset == p.iqPoints {
			cursor.offset = 0
			cursor.antenna++
			if cursor.antenna > p.antennaCount {
				cursor.antenna = 1
				if p.step > 0 && p.end-cursor.frequency >= p.step {
					cursor.frequency += p.step
				} else {
					cursor.frequency, complete = p.start, true
				}
			}
		}
	}
	if complete {
		cursor.sweep++
		if !p.continuous {
			m.mu.Lock()
			if m.session == s && s.taskID == taskID {
				s.running = false
			}
			m.mu.Unlock()
		}
	}
	return nil
}

func (m *PrecisionDFModule) level(p precisionDFTask, position float64) float64 {
	peak := math.Exp(-math.Pow((position-0.5)/0.06, 2))
	peak = max(peak, 0.8*math.Exp(-math.Pow((position-0.2)/0.03, 2)))
	peak = max(peak, 0.9*math.Exp(-math.Pow((position-0.8)/0.04, 2)))
	return m.cfg.LevelDBUV - float64(p.rfAttenuation+p.ifAttenuation) - 45 + 45*peak
}

func (m *PrecisionDFModule) spectrum(p precisionDFTask, offset, count, total uint64) []byte {
	payload := make([]byte, count*2)
	for i := uint64(0); i < count; i++ {
		position := float64(offset+i) / float64(max(uint64(1), total-1))
		binary.LittleEndian.PutUint16(payload[i*2:], uint16(int16(math.Round(m.level(p, position)*64))))
	}
	return payload
}

func (m *PrecisionDFModule) bearing(p precisionDFTask, level float64) []byte {
	angle := m.cfg.BearingDeg
	if p.mode == "SCANDF" && level < p.threshold {
		angle = -1
	}
	payload := make([]byte, 12)
	binary.LittleEndian.PutUint32(payload[:4], math.Float32bits(float32(angle)))
	binary.LittleEndian.PutUint32(payload[4:8], math.Float32bits(float32(level)))
	binary.LittleEndian.PutUint32(payload[8:12], uint32(m.cfg.Quality))
	return payload
}

func precisionDFDegreesMinutes(value float64) float64 {
	abs := math.Abs(value)
	degrees := math.Floor(abs)
	return math.Copysign(degrees*100+(abs-degrees)*60, value)
}

func (m *PrecisionDFModule) gnssPayload(now time.Time) []byte {
	payload := make([]byte, 40)
	binary.LittleEndian.PutUint32(payload[:4], 1)
	binary.LittleEndian.PutUint64(payload[4:12], math.Float64bits(precisionDFDegreesMinutes(m.position.Lng)))
	binary.LittleEndian.PutUint64(payload[12:20], math.Float64bits(precisionDFDegreesMinutes(m.position.Lat)))
	binary.LittleEndian.PutUint32(payload[20:24], math.Float32bits(50))
	binary.LittleEndian.PutUint32(payload[32:36], 12)
	binary.LittleEndian.PutUint32(payload[36:40], uint32(now.Unix()))
	return payload
}

func (m *PrecisionDFModule) nmeaPayload(now time.Time) []byte {
	latHemisphere, lngHemisphere := "N", "E"
	if m.position.Lat < 0 {
		latHemisphere = "S"
	}
	if m.position.Lng < 0 {
		lngHemisphere = "W"
	}
	sentence := fmt.Sprintf("GNRMC,%s.00,A,%09.4f,%s,%010.4f,%s,0.0,0.0,%s,,,A",
		now.Format("150405"), math.Abs(precisionDFDegreesMinutes(m.position.Lat)), latHemisphere,
		math.Abs(precisionDFDegreesMinutes(m.position.Lng)), lngHemisphere, now.Format("020106"))
	var checksum byte
	for _, b := range []byte(sentence) {
		checksum ^= b
	}
	return []byte(fmt.Sprintf("$%s*%02X\r\n", sentence, checksum))
}
