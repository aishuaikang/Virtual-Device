package engine

import (
	"errors"
	"testing"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/mocks"
)

type jammingRuntimeProbe struct {
	settings config.JammingConfig
	calls    int
	err      error
}

func (*jammingRuntimeProbe) Start()              {}
func (*jammingRuntimeProbe) Stop()               {}
func (*jammingRuntimeProbe) ConnectedCount() int { return 0 }
func (p *jammingRuntimeProbe) UpdateSimulation(settings config.JammingConfig) error {
	if p.err != nil {
		return p.err
	}
	p.settings = settings
	p.calls++
	return nil
}

func TestEngineJammingRuntimeUpdates(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name         string
		invalidJoint bool
		jammingError bool
	}{
		{name: "valid updates"},
		{name: "joint rejection leaves jamming unchanged", invalidJoint: true},
		{name: "jamming rejection leaves joint unchanged", jammingError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			joint := config.JointDirectionConfig{
				Enabled: true, HopIntervalMS: 200,
				Targets: []config.JointDirectionTarget{{
					Model: "O4", Frequencies: []float64{5800},
					Faces: []config.JointDirectionSample{{DeviceID: 2000, RSSI: -60, Reporting: true}},
				}},
			}
			state := mocks.NewJointDirectionState(joint)
			probe := &jammingRuntimeProbe{}
			if tt.jammingError {
				probe.err = errors.New("invalid simulation")
			}
			e := &Engine{running: true, jamming: probe, jointDirection: state}
			next := config.Config{JointDirection: joint, Jamming: config.JammingConfig{FaultMode: config.JammingFaultSilent}}
			next.JointDirection.Targets[0].Faces[0].RSSI = -50
			if tt.invalidJoint {
				next.JointDirection.HopIntervalMS++
			}
			err := e.UpdateRuntimeConfig(next)
			wantRSSI, wantCalls := -50.0, 1
			if tt.invalidJoint || tt.jammingError {
				wantRSSI, wantCalls = -60, 0
				if err == nil {
					t.Fatal("invalid update was accepted")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if probe.calls != wantCalls || state.Snapshot().Targets[0].Faces[0].RSSI != wantRSSI {
				t.Fatal("runtime update was partially applied or lost")
			}
			if wantCalls > 0 && probe.settings.FaultMode != config.JammingFaultSilent {
				t.Fatal("jamming settings did not reach the module")
			}
		})
	}
	t.Run("disabled module requires restart", func(t *testing.T) {
		e := &Engine{running: true}
		if err := e.UpdateRuntimeConfig(config.Config{Jamming: config.JammingConfig{Enabled: true}}); err == nil {
			t.Fatal("enabled a module without starting it")
		}
	})
}
