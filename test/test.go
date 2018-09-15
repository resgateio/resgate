package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jirenius/resgate/service"
	"github.com/posener/wstest"
)

const timeoutSeconds = 3600

type Session struct {
	*NATSTestClient
	s     *service.Service
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
	d := wstest.NewDialer(s.s.GetWSHandlerFunc())
	c, _, err := d.Dial("ws://example.org/", nil)
	if err != nil {
		panic(err)
	}

	return NewConn(s, d, c)
}

// HTTPRequest sends a request over HTTP
func (s *Session) HTTPRequest(method, url string, body []byte) *HTTPRequest {
	var r io.Reader
	r = bytes.NewReader(body)

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
