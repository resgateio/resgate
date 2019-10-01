package logger

import (
	"bytes"
	"log"
	"sync"
)

// MemLogger writes log messages to os.Stderr
type MemLogger struct {
	log   *log.Logger
	b     *bytes.Buffer
	debug bool
	trace bool
	mu    sync.Mutex
}

// NewMemLogger returns a new logger that writes to a bytes buffer
func NewMemLogger(debug bool, trace bool) *MemLogger {
	b := &bytes.Buffer{}
	return &MemLogger{
		log:   log.New(b, "", log.Ltime|log.Lmicroseconds),
		b:     b,
		debug: debug,
		trace: trace,
	}
}

// Log writes a log entry
func (l *MemLogger) Log(s string) {
	l.mu.Lock()
	l.log.Print("[INF] ", s)
	l.mu.Unlock()
}

// Error writes an error entry
func (l *MemLogger) Error(s string) {
	l.mu.Lock()
	l.log.Print("[ERR] ", s)
	l.mu.Unlock()
}

// Debug writes a debug entry
func (l *MemLogger) Debug(s string) {
	if l.debug {
		l.mu.Lock()
		l.log.Print("[DBG] ", s)
		l.mu.Unlock()
	}
}

// Trace writes a trace entry
func (l *MemLogger) Trace(s string) {
	if l.trace {
		l.mu.Lock()
		l.log.Print("[TRC] ", s)
		l.mu.Unlock()
	}
}

// String returns the log
func (l *MemLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}
