package test

import (
	"bytes"
	"fmt"
	"log"
)

type TestLogger struct {
	log   *log.Logger
	b     *bytes.Buffer
	debug bool
	trace bool
}

// NewStdLogger returns a new logger that writes to os.Stderr
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
