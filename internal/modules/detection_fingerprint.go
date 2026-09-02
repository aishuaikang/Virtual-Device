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
	fingerprintCommandTrain
	fingerprintCommandSave
	fingerprintCommandList
	fingerprintCommandDelete
)

type fingerprintCommand struct {
	kind      fingerprintCommandKind
	frequency int
	name      string
	index     int
}

type detectionString string

func (d detectionString) String() string { return string(d) }

var fingerprintRawNamePattern = regexp.MustCompile(`^type_[A-Za-z0-9_-]{1,32}$`)

func parseFingerprintCommand(command string) (fingerprintCommand, bool) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return fingerprintCommand{}, false
	}
	switch fields[0] {
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
	case fingerprintCommandList:
		send(f.fingerprintListResponse())
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
		send(f.fingerprintListResponse())
	}
	return true
}

func (f *detectionUDPServer) streamFingerprintTraining(
	ctx context.Context,
	conn *net.UDPConn,
	addr *net.UDPAddr,
	session *clientSession,
	frequency int,
) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	sampleCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sampleCount++
			observed := float64(frequency) + rand.Float64()*1.6 - 0.8
			confidence := 0.82 + float64(sampleCount)*0.008 + rand.Float64()*0.01
			if confidence > 0.998 {
				confidence = 0.998
			}
			message := fmt.Sprintf("freq=%.1f, Confidence=%.3f,", observed, confidence)
			if _, err := conn.WriteToUDP([]byte(message), addr); err != nil {
				continue
			}
			f.touchSession(session)
		}
	}
}

func (f *detectionUDPServer) fingerprintListResponse() string {
	f.fingerprintMu.RLock()
	defer f.fingerprintMu.RUnlock()
	var response strings.Builder
	response.WriteString("the pattern is list below:")
	for index, name := range f.fingerprints {
		fmt.Fprintf(&response, "\n%d %s", index+1, name)
	}
	return response.String()
}
