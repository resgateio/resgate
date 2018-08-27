package test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jirenius/resgate/service"
	"github.com/posener/wstest"
)

type Session struct {
	*NATSTestClient
	s     *service.Service
	d     *websocket.Dialer
	conns map[*Conn]struct{}
	l     *TestLogger
}

func setup() *Session {
	l := NewTestLogger()

	c := NewNATSTestClient(l)
	serv := service.NewService(c, TestConfig())
	serv.SetLogger(l)

	s := &Session{
		NATSTestClient: c,
		d:              wstest.NewDialer(serv.GetWSHandlerFunc()),
		s:              serv,
		conns:          make(map[*Conn]struct{}),
		l:              l,
	}

	if err := serv.Start(); err != nil {
		panic("test: failed to start server: " + err.Error())
	}

	return s
}

func (s *Session) Connect() *Conn {
	c, _, err := s.d.Dial("ws://example.org/", nil)
	if err != nil {
		panic(err)
	}

	return NewConn(c)
}

func teardown(s *Session) {
	for conn := range s.conns {
		conn.Disconnect()
	}
	st := s.s.StopChannel()
	go s.s.Stop(nil)

	select {
	case <-st:
	case <-time.After(3 * time.Second):
		panic("test: failed to stop server: timeout")
	}
}

func TestConfig() service.Config {
	var cfg service.Config
	cfg.SetDefault()
	cfg.NoHTTP = true
	return cfg
}

func concatJSON(raws ...[]byte) json.RawMessage {
	l := 0
	for _, raw := range raws {
		l += len(raw)
	}

	out := make([]byte, 0, l)
	for _, raw := range raws {
		out = append(out, raw...)
	}

	return out
}

func runTest(t *testing.T, cb func(s *Session)) {
	var s *Session
	panicked := true
	defer func() {
		if panicked {
			t.Logf("Trace log:\n%s", s.l)
		}
	}()

	s = setup()
	cb(s)
	teardown(s)

	panicked = false
}
