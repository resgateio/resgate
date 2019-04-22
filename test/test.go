package test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jirenius/resgate/logger"
	"github.com/jirenius/resgate/server"
	"github.com/posener/wstest"
)

const timeoutSeconds = 1

// Session represents a test session with a resgate server
type Session struct {
	t *testing.T
	*NATSTestClient
	s     *server.Service
	conns map[*Conn]struct{}
	l     *logger.MemLogger
}

func setup(t *testing.T) *Session {
	l := logger.NewMemLogger(true, true)

	c := NewNATSTestClient(l)
	serv := server.NewService(c, TestConfig())
	serv.SetLogger(l)

	s := &Session{
		t:              t,
		NATSTestClient: c,
		s:              serv,
		conns:          make(map[*Conn]struct{}),
		l:              l,
	}

	if err := serv.Start(); err != nil {
		panic("test: failed to start server: " + err.Error())
	}

	return s
}

// ConnectWithChannel makes a new mock client websocket connection
// with a ClientEvent channel.
func (s *Session) ConnectWithChannel(evs chan *ClientEvent) *Conn {
	d := wstest.NewDialer(s.s.GetWSHandlerFunc())
	c, _, err := d.Dial("ws://example.org/", nil)
	if err != nil {
		panic(err)
	}

	conn := NewConn(s, d, c, evs)
	s.conns[conn] = struct{}{}
	return conn
}

// Connect makes a new mock client websocket connection
func (s *Session) Connect() *Conn {
	return s.ConnectWithChannel(make(chan *ClientEvent, 256))
}

// HTTPRequest sends a request over HTTP
func (s *Session) HTTPRequest(method, url string, body []byte) *HTTPRequest {
	r := bytes.NewReader(body)

	req, err := http.NewRequest(method, url, r)
	if err != nil {
		panic("test: failed to create new http request: " + err.Error())
	}

	// Record the response into a httptest.ResponseRecorder
	rr := httptest.NewRecorder()

	hr := &HTTPRequest{
		req: req,
		rr:  rr,
		ch:  make(chan *HTTPResponse, 1),
	}

	go func() {
		s.l.Tracef("[HTTP] ", "--> %s %s: %s", method, url, body)
		s.s.ServeHTTP(rr, req)
		s.l.Tracef("[HTTP] ", "<-- %s %s: %s", method, url, rr.Body.String())
		hr.ch <- &HTTPResponse{ResponseRecorder: rr}
	}()

	return hr
}

func teardown(s *Session) {
	for conn := range s.conns {
		err := conn.Error()
		if err != nil {
			panic(err.Error())
		}
		conn.Disconnect()
		if s.t != nil {
			conn.AssertClosed(s.t)
		}
	}
	st := s.s.StopChannel()
	go s.s.Stop(nil)

	select {
	case <-st:
	case <-time.After(3 * time.Second):
		panic("test: failed to stop server: timeout")
	}
}

// TestConfig returns a default server configuration used for testing
func TestConfig() server.Config {
	var cfg server.Config
	cfg.SetDefault()
	cfg.NoHTTP = true
	return cfg
}

func runTest(t *testing.T, cb func(s *Session)) {
	var s *Session
	panicked := true
	defer func() {
		if panicked {
			t.Logf("Trace log:\n%s", s.l)
		}
	}()

	s = setup(t)
	cb(s)
	teardown(s)

	panicked = false
}
