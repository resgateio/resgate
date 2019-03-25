package logger

import (
	"bytes"
	"fmt"
	"log"
)

// MemLogger writes log messages to os.Stderr
type MemLogger struct {
	log   *log.Logger
	b     *bytes.Buffer
	debug bool
	trace bool
}

// NewMemLogger returns a new logger that writes to a bytes buffer
func NewMemLogger(debug bool, trace bool) *MemLogger {
	logFlags := log.LstdFlags
	if debug {
		logFlags = log.Ltime
	}

	b := &bytes.Buffer{}

	return &MemLogger{
		log:   log.New(b, "", logFlags),
		b:     b,
		debug: debug,
		trace: trace,
	}
}

// Logf writes a log entry
func (l *MemLogger) Logf(prefix string, format string, v ...interface{}) {
	l.log.Print(prefix, fmt.Sprintf(format, v...))
}

// Debugf writes a debug entry
func (l *MemLogger) Debugf(prefix string, format string, v ...interface{}) {
	if l.debug {
		l.log.Print(prefix, fmt.Sprintf(format, v...))
	}
}

// Tracef writes a trace entry
func (l *MemLogger) Tracef(prefix string, format string, v ...interface{}) {
	if l.trace {
		l.log.Print(prefix, fmt.Sprintf(format, v...))
	}
}

// String returns the log
func (l *MemLogger) String() string {
	return l.b.String()
}
