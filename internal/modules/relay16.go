package modules

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"virtual-device-ui/internal/config"
)

// Relay16Snapshot reports the simulated board's outputs, including reserved IO16.
type Relay16Snapshot struct {
	Listening         bool     `json:"listening"`
	Connected         bool     `json:"connected"`
	ListenAddress     string   `json:"listenAddress"`
	ActiveConnections int      `json:"activeConnections"`
	ClientAddresses   []string `json:"clientAddresses"`
	ReceivedFrames    uint64   `json:"receivedFrames"`
	SentFrames        uint64   `json:"sentFrames"`
	OutputMask        uint16   `json:"outputMask"`
	LastCommand       string   `json:"lastCommand,omitempty"`
	LastActivityAt    string   `json:"lastActivityAt,omitempty"`
	LastError         string   `json:"lastError,omitempty"`
}

// Relay16Module serves the type-12 board's Modbus TCP interface on unit 1.
type Relay16Module struct {
	ctx           context.Context
	cancel        context.CancelFunc
	listener      net.Listener
	listenAddress string
	responseDelay time.Duration
	stopOnce      sync.Once
	clientsWG     sync.WaitGroup

	mu             sync.Mutex
	listening      bool
	clients        map[net.Conn]string
	receivedFrames uint64
	sentFrames     uint64
	outputMask     uint16
	lastCommand    string
	lastActivityAt time.Time
	lastError      string
}

func NewRelay16Module(cfg config.Relay16Config) (*Relay16Module, error) {
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen for relay16 simulator on %s: %w", address, err)
	}
	if cfg.Port == 0 {
		address = listener.Addr().String()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Relay16Module{
		ctx:           ctx,
		cancel:        cancel,
		listener:      listener,
		listenAddress: address,
		responseDelay: time.Duration(cfg.ResponseDelayMS) * time.Millisecond,
		listening:     true,
		clients:       make(map[net.Conn]string),
	}, nil
}

func (module *Relay16Module) Start() {
	log.Printf("[定向窄带] 模拟器开始监听 address=%s unit=1", module.listenAddress)
	defer module.clientsWG.Wait()
	for {
		connection, err := module.listener.Accept()
		if err != nil {
			if module.ctx.Err() == nil {
				module.recordError(err)
			}
			return
		}
		module.mu.Lock()
		if module.ctx.Err() != nil {
			module.mu.Unlock()
			_ = connection.Close()
			continue
		}
		module.clients[connection] = connection.RemoteAddr().String()
		module.mu.Unlock()
		module.clientsWG.Add(1)
		go func() {
			defer module.clientsWG.Done()
			module.handleConnection(connection)
		}()
	}
}

func (module *Relay16Module) Stop() {
	module.stopOnce.Do(func() {
		module.cancel()
		_ = module.listener.Close()
		module.mu.Lock()
		module.listening = false
		module.outputMask = 0
		connections := make([]net.Conn, 0, len(module.clients))
		for connection := range module.clients {
			connections = append(connections, connection)
		}
		module.mu.Unlock()
		for _, connection := range connections {
			_ = connection.Close()
		}
	})
}

func (module *Relay16Module) handleConnection(connection net.Conn) {
	defer func() {
		_ = connection.Close()
		module.mu.Lock()
		delete(module.clients, connection)
		module.mu.Unlock()
	}()
	for {
		var header [7]byte
		if _, err := io.ReadFull(connection, header[:]); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				module.recordError(err)
			}
			return
		}
		length := int(binary.BigEndian.Uint16(header[4:6]))
		if binary.BigEndian.Uint16(header[2:4]) != 0 || header[6] != 1 || length < 2 || length > 254 {
			module.recordError(errors.New("invalid Modbus TCP header (protocol, unit or length)"))
			return
		}
		pdu := make([]byte, length-1)
		if _, err := io.ReadFull(connection, pdu); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				module.recordError(err)
			}
			return
		}
		module.mu.Lock()
		module.receivedFrames++
		module.lastActivityAt = time.Now()
		module.mu.Unlock()
		log.Printf("[定向窄带] RX remote=%s frame=% X % X", connection.RemoteAddr(), header, pdu)
		if module.responseDelay > 0 {
			timer := time.NewTimer(module.responseDelay)
			select {
			case <-module.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		responsePDU := module.handlePDU(pdu)
		binary.BigEndian.PutUint16(header[4:6], uint16(1+len(responsePDU)))
		response := append(header[:], responsePDU...)
		if _, err := io.Copy(connection, bytes.NewReader(response)); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				module.recordError(err)
			}
			return
		}
		module.mu.Lock()
		module.sentFrames++
		module.mu.Unlock()
		log.Printf("[定向窄带] TX remote=%s frame=% X", connection.RemoteAddr(), response)
	}
}

func (module *Relay16Module) handlePDU(pdu []byte) []byte {
	module.mu.Lock()
	defer module.mu.Unlock()
	module.lastActivityAt = time.Now()
	module.lastCommand = fmt.Sprintf("0x%02X", pdu[0])
	module.lastError = ""
	exception := func(code byte, detail string) []byte {
		module.lastError = detail
		return []byte{pdu[0] | 0x80, code}
	}
	if module.ctx.Err() != nil {
		return exception(0x04, "relay simulator has stopped")
	}
	if pdu[0] != 0x01 && pdu[0] != 0x0F {
		return exception(0x01, "unsupported Modbus function")
	}
	if len(pdu) < 5 || (pdu[0] == 0x01 && len(pdu) != 5) {
		return exception(0x03, "invalid coil request length")
	}
	start := int(binary.BigEndian.Uint16(pdu[1:3]))
	count := int(binary.BigEndian.Uint16(pdu[3:5]))
	if count == 0 {
		return exception(0x03, "coil count must be positive")
	}
	if start+count > 16 {
		return exception(0x02, "coil address is outside IO1–IO16")
	}
	module.lastCommand = fmt.Sprintf("0x%02X · IO%d–IO%d", pdu[0], start+1, start+count)
	byteCount := (count + 7) / 8
	if pdu[0] == 0x01 {
		response := make([]byte, 2+byteCount)
		response[0], response[1] = 0x01, byte(byteCount)
		mask := module.outputMask >> start
		for index := 0; index < count; index++ {
			if mask&(1<<index) != 0 {
				response[2+index/8] |= 1 << (index % 8)
			}
		}
		return response
	}
	if len(pdu) != 6+byteCount || int(pdu[5]) != byteCount {
		return exception(0x03, "invalid write coil byte count")
	}
	for index := 0; index < count; index++ {
		bit := uint16(1) << (start + index)
		if pdu[6+index/8]&(1<<(index%8)) != 0 {
			module.outputMask |= bit
		} else {
			module.outputMask &^= bit
		}
	}
	module.lastCommand += fmt.Sprintf(" · 0x%04X", module.outputMask)
	return append([]byte(nil), pdu[:5]...)
}

func (module *Relay16Module) recordError(err error) {
	module.mu.Lock()
	module.lastError = err.Error()
	module.mu.Unlock()
	log.Printf("[定向窄带] 通信错误 error=%v", err)
}

func (module *Relay16Module) Snapshot() Relay16Snapshot {
	module.mu.Lock()
	defer module.mu.Unlock()
	clients := make([]string, 0, len(module.clients))
	for _, address := range module.clients {
		clients = append(clients, address)
	}
	sort.Strings(clients)
	lastActivity := ""
	if !module.lastActivityAt.IsZero() {
		lastActivity = module.lastActivityAt.Format(time.RFC3339Nano)
	}
	// The platform uses short-lived connections and polls once a second.
	connected := len(clients) > 0 ||
		(!module.lastActivityAt.IsZero() && time.Since(module.lastActivityAt) < 5*time.Second)
	return Relay16Snapshot{
		Listening:         module.listening,
		Connected:         module.listening && connected,
		ListenAddress:     module.listenAddress,
		ActiveConnections: len(clients),
		ClientAddresses:   clients,
		ReceivedFrames:    module.receivedFrames,
		SentFrames:        module.sentFrames,
		OutputMask:        module.outputMask,
		LastCommand:       module.lastCommand,
		LastActivityAt:    lastActivity,
		LastError:         module.lastError,
	}
}
