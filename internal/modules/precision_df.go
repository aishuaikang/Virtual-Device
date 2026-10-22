package modules

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/protocol"
)

// PrecisionDFSnapshot describes a single receiver and its paired TCP connections.
type PrecisionDFSnapshot struct {
	Listening        bool     `json:"listening"`
	CommandAddress   string   `json:"commandAddress"`
	DataAddress      string   `json:"dataAddress"`
	CommandConnected bool     `json:"commandConnected"`
	DataConnected    bool     `json:"dataConnected"`
	ClientAddresses  []string `json:"clientAddresses"`
	Running          bool     `json:"running"`
	Mode             string   `json:"mode"`
	TaskID           uint32   `json:"taskID"`
	ReceivedCommands uint64   `json:"receivedCommands"`
	SentFrames       uint64   `json:"sentFrames"`
	IQBytes          uint64   `json:"iqBytes"`
	LastCommand      string   `json:"lastCommand,omitempty"`
	LastActivityAt   string   `json:"lastActivityAt,omitempty"`
	LastError        string   `json:"lastError,omitempty"`
}

type precisionDFSession struct {
	command, data net.Conn
	done          chan struct{}
	wake          chan struct{}
	closeOnce     sync.Once
	streamMu      sync.Mutex
	task          precisionDFTask // guarded by the module mutex
	taskID        uint32
	running       bool
}

// PrecisionDFModule accepts one platform connection, like one physical receiver.
type PrecisionDFModule struct {
	commandListener, dataListener net.Listener
	cfg                           config.PrecisionDFConfig
	position                      config.GPS
	stop                          chan struct{}
	stopOnce                      sync.Once
	wg                            sync.WaitGroup
	mu                            sync.Mutex
	session                       *precisionDFSession
	status                        PrecisionDFSnapshot
}

// NewPrecisionDFModule binds both ports before the engine reports successful startup.
func NewPrecisionDFModule(cfg config.PrecisionDFConfig, position config.GPS) (*PrecisionDFModule, error) {
	if cfg.IntervalMS < 20 || cfg.IntervalMS > 60000 {
		return nil, fmt.Errorf("receiver interval must be 20..60000 ms")
	}
	command, err := net.Listen("tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.CommandPort)))
	if err != nil {
		return nil, fmt.Errorf("listen for receiver commands: %w", err)
	}
	data, err := net.Listen("tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.DataPort)))
	if err != nil {
		_ = command.Close()
		return nil, fmt.Errorf("listen for receiver data: %w", err)
	}
	return &PrecisionDFModule{
		commandListener: command, dataListener: data, cfg: cfg, position: position,
		stop:   make(chan struct{}),
		status: PrecisionDFSnapshot{Listening: true, CommandAddress: command.Addr().String(), DataAddress: data.Addr().String()},
	}, nil
}

func (m *PrecisionDFModule) Start() {
	log.Printf("[精准测向] 模拟器开始监听 command=%s data=%s", m.commandListener.Addr(), m.dataListener.Addr())
	m.wg.Add(2)
	go m.acceptCommands()
	go m.acceptData()
	m.wg.Wait()
}

func (m *PrecisionDFModule) Stop() {
	m.stopOnce.Do(func() {
		close(m.stop)
		_ = m.commandListener.Close()
		_ = m.dataListener.Close()
		m.mu.Lock()
		m.status.Listening = false
		s := m.session
		m.mu.Unlock()
		if s != nil {
			m.closeSession(s)
		}
	})
}

func (m *PrecisionDFModule) Snapshot() PrecisionDFSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := m.status
	snapshot.ClientAddresses = nil
	if s := m.session; s != nil {
		snapshot.CommandConnected = true
		snapshot.DataConnected = s.data != nil
		snapshot.ClientAddresses = []string{s.command.RemoteAddr().String()}
		snapshot.Running = s.running
	}
	return snapshot
}

func (m *PrecisionDFModule) acceptCommands() {
	defer m.wg.Done()
	for {
		conn, err := m.commandListener.Accept()
		if err != nil {
			return
		}
		m.mu.Lock()
		if !m.status.Listening || m.session != nil {
			m.mu.Unlock()
			_ = conn.Close()
			continue
		}
		s := &precisionDFSession{command: conn, done: make(chan struct{}), wake: make(chan struct{}, 1), task: newPrecisionDFTask()}
		m.session = s
		m.status.LastError = ""
		m.wg.Add(1)
		m.mu.Unlock()
		go func() {
			defer m.wg.Done()
			m.handleCommands(s)
		}()
	}
}

func (m *PrecisionDFModule) acceptData() {
	defer m.wg.Done()
	for {
		conn, err := m.dataListener.Accept()
		if err != nil {
			return
		}
		m.mu.Lock()
		s := m.session
		remote, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		commandRemote := ""
		if s != nil {
			commandRemote, _, _ = net.SplitHostPort(s.command.RemoteAddr().String())
		}
		if !m.status.Listening || s == nil || s.data != nil || commandRemote != remote {
			m.mu.Unlock()
			_ = conn.Close()
			continue
		}
		s.data = conn
		m.wg.Add(2)
		m.mu.Unlock()
		log.Printf("[精准测向] 客户端已连接 remote=%s", s.command.RemoteAddr())
		go func() {
			defer m.wg.Done()
			defer m.closeSession(s)
			_, _ = io.Copy(io.Discard, conn)
		}()
		go func() {
			defer m.wg.Done()
			defer m.closeSession(s)
			m.stream(s)
		}()
	}
}

func (m *PrecisionDFModule) closeSession(s *precisionDFSession) {
	s.closeOnce.Do(func() {
		m.mu.Lock()
		if m.session == s {
			m.session = nil
		}
		s.running = false
		data := s.data
		close(s.done)
		m.mu.Unlock()
		_ = s.command.Close()
		if data != nil {
			_ = data.Close()
		}
		log.Printf("[精准测向] 客户端已断开 remote=%s", s.command.RemoteAddr())
	})
}

func (m *PrecisionDFModule) writeCommand(s *precisionDFSession, text string) error {
	frame, err := protocol.EncodePrecisionDFCommand(text)
	if err != nil {
		return err
	}
	_ = s.command.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return writePrecisionDFBytes(s.command, frame)
}

func writePrecisionDFBytes(w io.Writer, frame []byte) error {
	for len(frame) > 0 {
		n, err := w.Write(frame)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		frame = frame[n:]
	}
	return nil
}

func (m *PrecisionDFModule) handleCommands(s *precisionDFSession) {
	defer m.closeSession(s)
	port := m.dataListener.Addr().(*net.TCPAddr).Port
	if err := m.writeCommand(s, fmt.Sprintf("DATA_AT_PORT %d", port)); err != nil {
		return
	}
	for {
		text, err := protocol.ReadPrecisionDFCommand(s.command)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				m.recordError(err)
			}
			return
		}
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			key, arg, _ := strings.Cut(line, " ")
			arg = strings.TrimSpace(arg)
			m.mu.Lock()
			m.status.ReceivedCommands++
			m.status.LastCommand = line
			m.status.LastActivityAt = time.Now().Format(time.RFC3339)
			err = m.command(s, key, arg)
			if err != nil {
				m.status.LastError = err.Error()
			} else {
				m.status.LastError = ""
				m.status.Mode = s.task.mode
				if s.task.antennaMode != "" {
					m.status.Mode = s.task.antennaMode
				}
			}
			taskID := s.taskID
			m.mu.Unlock()
			log.Printf("[精准测向] RX command=%s", line)
			reply := key + " OK"
			if err != nil {
				reply = key + " ERROR"
			} else if key == "START" {
				reply += fmt.Sprintf("\nTASK %d", taskID)
			} else if key == "ABOR" || key == "CLR" {
				// Do not acknowledge stop while a previous data frame is still being written.
				s.streamMu.Lock()
				s.streamMu.Unlock()
			}
			if err := m.writeCommand(s, reply); err != nil {
				return
			}
			if err == nil && key == "START" {
				select {
				case s.wake <- struct{}{}:
				default:
				}
			}
		}
	}
}

// command runs under m.mu; network I/O happens only after releasing it.
func (m *PrecisionDFModule) command(s *precisionDFSession, key, arg string) error {
	if m.session != s {
		return fmt.Errorf("receiver session closed")
	}
	switch key {
	case "CLR":
		if arg != "" {
			return fmt.Errorf("CLR takes no arguments")
		}
		s.running, s.task = false, newPrecisionDFTask()
	case "ABOR":
		if arg != s.task.business() {
			return fmt.Errorf("invalid receiver stop business")
		}
		s.running = false
	case "START":
		if s.running || s.data == nil || !s.task.configured || arg != s.task.business() {
			return fmt.Errorf("receiver is busy, unconfigured, or has no data connection")
		}
		if s.task.mode == "DFCAP" && len(s.task.antennas) != int(s.task.antennaCount) {
			return fmt.Errorf("configure every antenna code before IQ acquisition")
		}
		if s.task.mode == "FFM" && s.task.frequency == 0 {
			return fmt.Errorf("configure the center frequency before IQ acquisition")
		}
		m.status.TaskID++
		if m.status.TaskID == 0 {
			m.status.TaskID++
		}
		s.taskID, s.running = m.status.TaskID, true
	default:
		if s.running && key != "GET:GNSS" && key != "GET:RAWGNSS" {
			return fmt.Errorf("stop acquisition before changing receiver parameters")
		}
		return s.task.configure(key, arg)
	}
	return nil
}

func (m *PrecisionDFModule) recordError(err error) {
	m.mu.Lock()
	m.status.LastError = err.Error()
	m.mu.Unlock()
	log.Printf("[精准测向] error=%v", err)
}
