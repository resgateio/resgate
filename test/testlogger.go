package test

import (
	"bytes"
	"fmt"
	"log"
)

// TestLogger implements logger.Logger, and writes log data to a buffer.
type TestLogger struct {
	log *log.Logger
	b   *bytes.Buffer
}

// NewTestLogger returns a new logger that writes to a buffer
func NewTestLogger() *TestLogger {
	b := &bytes.Buffer{}
	return &TestLogger{
		log: log.New(b, "", 0),
		b:   b,
	}
}

// Logf writes a log entry
func (l *TestLogger) Logf(prefix string, format string, v ...interface{}) {
	l.log.Print(prefix, fmt.Sprintf(format, v...))
}

// Debugf writes a debug entry
func (l *TestLogger) Debugf(prefix string, format string, v ...interface{}) {
	l.log.Print(prefix, fmt.Sprintf(format, v...))
}

// Tracef writes a trace entry
func (l *TestLogger) Tracef(prefix string, format string, v ...interface{}) {
	l.log.Print(prefix, fmt.Sprintf(format, v...))
}

func (l *TestLogger) String() string {
	return l.b.String()
}
