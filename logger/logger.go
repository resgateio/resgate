package logger

import (
	"log"
	"os"
)

// Logger is used to write log messages
type Logger interface {
	// Log writes a log entry
	Log(s string)

	// Error writes an error entry
	Error(s string)

	// Debug writes a debug entry
	Debug(s string)

	// Trace writes a trace entry
	Trace(s string)
}

// StdLogger writes log messages to os.Stderr
type StdLogger struct {
	log   *log.Logger
	debug bool
	trace bool
}

// NewStdLogger returns a new logger that writes to os.Stderr
func NewStdLogger(debug bool, trace bool) *StdLogger {
	return &StdLogger{
		log:   log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds),
		debug: debug,
		trace: trace,
	}
}

// Log writes a log entry
func (l *StdLogger) Log(s string) {
	l.log.Print("[INF] ", s)
}

// Error writes an error entry
func (l *StdLogger) Error(s string) {
	l.log.Print("[ERR] ", s)
}

// Trace writes a trace entry
func (l *StdLogger) Trace(s string) {
	if l.trace {
		l.log.Print("[TRC] ", s)
	}
}

// Debug writes a debug entry
func (l *StdLogger) Debug(s string) {
	if l.debug {
		l.log.Print("[DBG] ", s)
	}
}
