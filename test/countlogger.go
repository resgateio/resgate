package test

import (
	"bytes"
	"log"
	"sync"
	"testing"
)

// CountLogger writes log messages to os.Stderr
type CountLogger struct {
	log    *log.Logger
	b      *bytes.Buffer
	debug  bool
	trace  bool
	errors int
	mu     sync.Mutex
}

// NewCountLogger returns a new logger that writes to a bytes buffer
func NewCountLogger(debug bool, trace bool) *CountLogger {
	b := &bytes.Buffer{}
	return &CountLogger{
		log:   log.New(b, "", log.Ltime|log.Lmicroseconds),
		b:     b,
		debug: debug,
		trace: trace,
	}
}

// Log writes a log entry
func (l *CountLogger) Log(s string) {
	l.mu.Lock()
	l.log.Print("[INF] ", s)
	l.mu.Unlock()
}

// Error writes an error entry
func (l *CountLogger) Error(s string) {
	l.mu.Lock()
	l.log.Print("[ERR] ", s)
	l.errors++
	l.mu.Unlock()
}

// Debug writes a debug entry
func (l *CountLogger) Debug(s string) {
	l.mu.Lock()
	l.log.Print("[DBG] ", s)
	l.mu.Unlock()
}

// Trace writes a trace entry
func (l *CountLogger) Trace(s string) {
	l.mu.Lock()
	l.log.Print("[TRC] ", s)
	l.mu.Unlock()
}

// String returns the log
func (l *CountLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// IsDebug returns true if debug logging is active
func (l *CountLogger) IsDebug() bool {
	return l.debug
}

// IsTrace returns true if trace logging is active
func (l *CountLogger) IsTrace() bool {
	return l.trace
}

// AssertErrorsLogged asserts that some error has been logged
// and clears the error count
func (l *CountLogger) AssertErrorsLogged(t *testing.T, count int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.errors != count {
		t.Fatalf("expected logged errors to be %d, but got %d", count, l.errors)
	}
	l.errors = 0
}

// AssertNoErrorsLogged asserts that no error has been logged
func (l *CountLogger) AssertNoErrorsLogged(t *testing.T) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.errors != 0 {
		t.Fatalf("expected no logged errors, but got %d", l.errors)
	}
}
