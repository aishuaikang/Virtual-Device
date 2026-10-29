package mocks

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/utils"
)

// JointDirectionState publishes one immutable sample snapshot to every face.
type JointDirectionState struct {
	settings atomic.Pointer[config.JointDirectionConfig]
}

func NewJointDirectionState(settings config.JointDirectionConfig) *JointDirectionState {
	state := &JointDirectionState{}
	state.Update(settings)
	return state
}

func (s *JointDirectionState) Update(settings config.JointDirectionConfig) {
	settings.Targets = slices.Clone(settings.Targets)
	for i := range settings.Targets {
		settings.Targets[i].Frequencies = slices.Clone(settings.Targets[i].Frequencies)
		settings.Targets[i].Faces = slices.Clone(settings.Targets[i].Faces)
	}
	s.settings.Store(&settings)
}

// Snapshot is read-only; updates must be published through Update.
func (s *JointDirectionState) Snapshot() *config.JointDirectionConfig {
	return s.settings.Load()
}

type jointDirectionGenerator struct {
	MockDataGenerator
	settings  *JointDirectionState
	faceIndex int
	startedAt time.Time
	now       func() time.Time
	sequence  atomic.Uint64
}

// All faces share an epoch, so their frequency hops depend on time, not send order.
func NewJointDirectionGenerator(base MockDataGenerator, settings *JointDirectionState, faceIndex int, startedAt time.Time) MockDataGenerator {
	return &jointDirectionGenerator{MockDataGenerator: base, settings: settings, faceIndex: faceIndex, startedAt: startedAt, now: time.Now}
}

type jointDirectionPacket string

func (p jointDirectionPacket) String() string { return string(p) }

func (g *jointDirectionGenerator) TotalDroneCount() int { return len(g.settings.Snapshot().Targets) }

func (g *jointDirectionGenerator) GenerateDetectionData(deviceID int, command *utils.DetectionCommand) fmt.Stringer {
	if command != nil && command.UsesDirectionData() {
		return g.MockDataGenerator.GenerateDetectionData(deviceID, command)
	}
	settings := g.settings.Snapshot()
	elapsed := max(g.now().Sub(g.startedAt), 0)
	hop := int64(elapsed / (time.Duration(settings.HopIntervalMS) * time.Millisecond))
	var packet strings.Builder
	for _, target := range settings.Targets {
		for _, face := range target.Faces {
			if face.DeviceID != deviceID || !face.Reporting {
				continue
			}
			// Small receiver offsets keep frequencies close but deliberately unequal.
			offset := float64(g.faceIndex%7-3) * 0.2
			freq := max(0.1, target.Frequencies[hop%int64(len(target.Frequencies))]+offset)
			rssi := min(0, max(-140, face.RSSI+0.4*math.Sin(elapsed.Seconds()*2+float64(g.faceIndex))))
			sample := SpectrumData{
				Device: deviceID, Model: target.Model, Freq: freq, RSSI: rssi,
				ExtraFields: []SpectrumExtraField{{Key: "seq", Value: strconv.FormatUint(g.sequence.Add(1), 10)}},
			}
			packet.WriteString(sample.String())
		}
	}
	if packet.Len() == 0 {
		return nil // No fabricated target or RSSI for a missing face; heartbeats continue.
	}
	return jointDirectionPacket(packet.String())
}
