package test

import (
	"errors"
	"fmt"
)

type Testing interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

type LogTesting struct {
	// If true, does not panic when Fatal or Fatalf is called.
	NoPanic bool
	Err     error
}

func (t *LogTesting) Error(args ...interface{}) {
	t.Err = errors.New(fmt.Sprint(args...))
}

func (t *LogTesting) Errorf(format string, args ...interface{}) {
	t.Err = fmt.Errorf(format, args...)

}

func (t *LogTesting) Fatal(args ...interface{}) {
	if t.NoPanic {
		t.Error(args...)
	} else {
		panic(errors.New(fmt.Sprint(args...)))
	}
}

func (t *LogTesting) Fatalf(format string, args ...interface{}) {
	if t.NoPanic {
		t.Errorf(format, args...)
	} else {
		panic(fmt.Errorf(format, args...))
	}
}

func (t *LogTesting) Defer() {
	if i := recover(); i != nil {
		switch v := i.(type) {
		case error:
			t.Err = v
		default:
			t.Err = fmt.Errorf("%#v", i)
		}
	}
}
