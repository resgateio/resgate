package logger

import (
	"fmt"
	"log"
	"os"
)

// Logger is used to write log messages
type Logger interface {
	Logf(prefix string, format string, v ...interface{})
	Debugf(prefix string, format string, v ...interface{})
	Tracef(prefix string, format string, v ...interface{})
}

// StdLogger writes log messages to os.Stderr
type StdLogger struct {
	log   *log.Logger
	debug bool
	trace bool
}

// NewStdLogger returns a new logger that writes to os.Stderr
func NewStdLogger(debug bool, trace bool) *StdLogger {
	logFlags := log.LstdFlags
	if debug {
		logFlags = log.Ltime
	}

	return &StdLogger{
		log:   log.New(os.Stderr, "", logFlags),
		debug: debug,
		trace: trace,
	}
}

// Logf writes a log entry
func (l *StdLogger) Logf(prefix string, format string, v ...interface{}) {
	l.log.Print(prefix, fmt.Sprintf(format, v...))
}

// Debugf writes a debug entry
func (l *StdLogger) Debugf(prefix string, format string, v ...interface{}) {
	if l.debug {
		l.log.Print(prefix, fmt.Sprintf(format, v...))
	}
}

// Tracef writes a trace entry
func (l *StdLogger) Tracef(prefix string, format string, v ...interface{}) {
	if l.trace {
		l.log.Print(prefix, fmt.Sprintf(format, v...))
	}
}
