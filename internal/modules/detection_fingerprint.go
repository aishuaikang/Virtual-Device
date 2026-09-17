package modules

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type fingerprintCommandKind uint8

const (
	fingerprintCommandUnknown fingerprintCommandKind = iota
	fingerprintCommandCapability
	fingerprintCommandAuthorize
	fingerprintCommandTrain
	fingerprintCommandSave
	fingerprintCommandList
	fingerprintCommandDelete
)

const simulatedFingerprintAuthorizationCode = "f381d12cca7987fe"

type fingerprintCommand struct {
	kind              fingerprintCommandKind
	authorizationCode string
	frequency         int
	name              string
	index             int
}

type detectionString string

func (d detectionString) String() string { return string(d) }

var (
	fingerprintRawNamePattern       = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
	fingerprintAuthorizationPattern = regexp.MustCompile(`^[0-9A-Fa-f]{16}$`)
)

func parseFingerprintCommand(command string) (fingerprintCommand, bool) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return fingerprintCommand{}, false
	}
	switch fields[0] {
	case "-setfun":
		if len(fields) != 2 || !fingerprintAuthorizationPattern.MatchString(fields[1]) {
			return fingerprintCommand{}, false
		}
		return fingerprintCommand{kind: fingerprintCommandAuthorize, authorizationCode: fields[1]}, true
	case "-train":
		if len(fields) == 1 {
			return fingerprintCommand{kind: fingerprintCommandCapability}, true
		}
		if len(fields) != 2 {
			return fingerprintCommand{}, false
		}
		frequency, err := strconv.Atoi(fields[1])
		if err != nil || frequency < 70 || frequency > 8000 {
			return fingerprintCommand{}, false
		}
		return fingerprintCommand{kind: fingerprintCommandTrain, frequency: frequency}, true
	case "-save":
		if len(fields) != 2 || !fingerprintRawNamePattern.MatchString(fields[1]) {
			return fingerprintCommand{}, false
		}
		return fingerprintCommand{kind: fingerprintCommandSave, name: fields[1]}, true
	case "-list_type":
		if len(fields) == 1 {
			return fingerprintCommand{kind: fingerprintCommandList}, true
		}
	case "-del_type":
		if len(fields) != 2 {
			return fingerprintCommand{}, false
		}
		index, err := strconv.Atoi(fields[1])
		if err != nil || index <= 0 {
			return fingerprintCommand{}, false
		}
		return fingerprintCommand{kind: fingerprintCommandDelete, index: index}, true
	}
	return fingerprintCommand{}, false
}

func redactDetectionCommandForLog(data []byte) string {
	command := strings.TrimSpace(string(data))
	if strings.HasPrefix(command, "-setfun") {
		return "-setfun [REDACTED]"
	}
	return command
}

func (f *detectionUDPServer) handleFingerprintCommand(
	conn *net.UDPConn,
	addr *net.UDPAddr,
	session *clientSession,
	commandText string,
) bool {
	command, ok := parseFingerprintCommand(commandText)
	if !ok {
		return false
	}
	send := func(message string) {
		if _, err := conn.WriteToUDP([]byte(message), addr); err != nil {
			return
		}
		f.touchSession(session)
	}

	if command.kind == fingerprintCommandAuthorize {
		if strings.EqualFold(command.authorizationCode, simulatedFingerprintAuthorizationCode) {
			f.fingerprintMu.Lock()
			f.fingerprintAuthorized = true
			f.fingerprintMu.Unlock()
		}
		send(buildFingerprintAuthorizationAck(f.deviceId, strings.TrimSpace(commandText)))
		return true
	}

	if !f.isFingerprintAuthorized() {
		send("The device does not support AI")
		return true
	}

	switch command.kind {
	case fingerprintCommandCapability:
		send("The device support AI")
	case fingerprintCommandTrain:
		session.mu.Lock()
		if session.cancel != nil {
			session.cancel()
		}
		trainingCtx, cancel := context.WithCancel(f.ctx)
		session.ctx = trainingCtx
		session.cancel = cancel
		session.isRunning = true
		session.lastCommand = nil
		session.fingerprintTraining = true
		session.trainingFrequency = command.frequency
		session.mu.Unlock()
		go f.streamFingerprintTraining(trainingCtx, conn, addr, session, command.frequency)
	case fingerprintCommandSave:
		session.mu.Lock()
		if !session.fingerprintTraining {
			session.mu.Unlock()
			send("fingerprint training is not active")
			return true
		}
		if session.cancel != nil {
			session.cancel()
		}
		session.ctx = nil
		session.cancel = nil
		session.isRunning = false
		session.fingerprintTraining = false
		session.savedFingerprintFrequency = session.trainingFrequency
		session.savedFingerprintModel = command.name
		session.mu.Unlock()

		f.fingerprintMu.Lock()
		found := false
		for _, name := range f.fingerprints {
			if name == command.name {
				found = true
				break
			}
		}
		if !found {
			f.fingerprints = append(f.fingerprints, command.name)
		}
		f.fingerprintMu.Unlock()
		send(command.name + " is saved.,")
		send(buildDetectorCommandAck(f.deviceId, "-save "+command.name))
	case fingerprintCommandList:
		if response := f.fingerprintListResponse(); response != "" {
			send(response)
		}
	case fingerprintCommandDelete:
		var deleted string
		f.fingerprintMu.Lock()
		if command.index <= len(f.fingerprints) {
			deleted = f.fingerprints[command.index-1]
			f.fingerprints = append(f.fingerprints[:command.index-1], f.fingerprints[command.index:]...)
		}
		f.fingerprintMu.Unlock()
		if deleted != "" {
			session.mu.Lock()
			if session.savedFingerprintModel == deleted {
				session.savedFingerprintModel = ""
			}
			session.mu.Unlock()
		}
		if response := f.fingerprintListResponse(); response != "" {
			send(response)
		}
	}
	return true
}

func (f *detectionUDPServer) isFingerprintAuthorized() bool {
	f.fingerprintMu.RLock()
	defer f.fingerprintMu.RUnlock()
	return f.fingerprintAuthorized
}

func buildFingerprintAuthorizationAck(deviceID int, command string) string {
	return fmt.Sprintf("device=%d, dna(0), ver=detect-20260802, b4, %s", deviceID, strings.TrimSpace(command))
}

func (f *detectionUDPServer) streamFingerprintTraining(
	ctx context.Context,
	conn *net.UDPConn,
	addr *net.UDPAddr,
	session *clientSession,
	frequency int,
) {
	sampleTicker := time.NewTicker(65 * time.Millisecond)
	defer sampleTicker.Stop()
	heartbeatInterval := time.Duration(f.detectionCfg.HeartbeatInterval) * time.Second
	if heartbeatInterval <= 0 {
		heartbeatInterval = 10 * time.Second
	}
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()
	sampleCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-sampleTicker.C:
			sampleCount++
			observed, confidence := fingerprintTrainingSample(frequency, sampleCount)
			message := fmt.Sprintf("freq=%.1f, Confidence=%.3f,", observed, confidence)
			if _, err := conn.WriteToUDP([]byte(message), addr); err != nil {
				continue
			}
			f.touchSession(session)
		case <-heartbeatTicker.C:
			data := f.mock.GenerateDirectionHeartbeatData(f.deviceId)
			if _, err := conn.WriteToUDP([]byte(data.String()), addr); err != nil {
				continue
			}
			f.touchSession(session)
		}
	}
}

func fingerprintTrainingSample(frequency, sampleCount int) (float64, float64) {
	switch sampleCount {
	case 1:
		return staleTrainingFrequency(frequency, 0), 0
	case 2:
		return staleTrainingFrequency(frequency, 30), 0
	case 3, 4, 5, 6, 7, 8, 9, 10, 11, 12:
		return float64(frequency), 0
	}

	observed := float64(frequency) + rand.Float64()*1.6 - 0.8
	if sampleCount%12 == 1 || sampleCount%12 == 2 {
		return observed, 0
	}
	return observed, 0.9 + rand.Float64()*0.1
}

func staleTrainingFrequency(frequency, offset int) float64 {
	stale := 5215 + offset
	if frequency < stale-2 || frequency > stale+2 {
		return float64(stale)
	}
	return 750 + float64(offset)
}

func (f *detectionUDPServer) fingerprintListResponse() string {
	f.fingerprintMu.RLock()
	defer f.fingerprintMu.RUnlock()
	if len(f.fingerprints) == 0 {
		return ""
	}
	var response strings.Builder
	response.WriteString("the pattern is list below:")
	for index, name := range f.fingerprints {
		fmt.Fprintf(&response, "\n%d %s", index+1, name)
	}
	return response.String()
}
