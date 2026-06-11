package engine

import (
	"bytes"
	"fmt"
	"sync"
)

// LogWriter 将 log 包输出桥接到 channel
type LogWriter struct {
	mu      sync.Mutex
	Ch      chan string
	buf     bytes.Buffer
	dropped int
}

func NewLogWriter(bufSize int) *LogWriter {
	return &LogWriter{
		Ch: make(chan string, bufSize),
	}
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)
	for {
		idx := bytes.IndexByte(w.buf.Bytes(), '\n')
		if idx < 0 {
			break
		}
		line := string(w.buf.Bytes()[:idx])
		w.buf.Next(idx + 1)
		w.enqueueLocked(line)
	}
	return len(p), nil
}

func (w *LogWriter) enqueueLocked(line string) {
	if w.dropped > 0 {
		summary := fmt.Sprintf("[WARN] 日志队列繁忙，已丢弃 %d 条日志", w.dropped)
		select {
		case w.Ch <- summary:
			w.dropped = 0
		default:
			w.dropped++
			return
		}
	}

	select {
	case w.Ch <- line:
	default:
		w.dropped++
	}
}
