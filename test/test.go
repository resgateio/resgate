package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/posener/wstest"
	"github.com/resgateio/resgate/server"
)

const timeoutSeconds = 1

var (
	versionLatest  = "1.999.999"
	versionRequest = json.RawMessage(fmt.Sprintf(`{"protocol":"%s"}`, versionLatest))
	versionResult  = json.RawMessage(fmt.Sprintf(`{"protocol":"%s"}`, server.ProtocolVersion))
)

// Session represents a test session with a resgate server
type Session struct {
	t *testing.T
	*NATSTestClient
	s      *server.Service
	conns  map[*Conn]struct{}
	dcCh   chan struct{}
	unsubs chan string
	*CountLogger
}

func setup(t *testing.T, cfgs ...func(*server.Config)) *Session {
	l := NewCountLogger(true, true)

	c := NewNATSTestClient(l)
	serv, err := server.NewService(c, DefaultConfig(cfgs...))
	if err != nil {
		t.Fatalf("error creating new service: %s", err)
	}
	serv.SetLogger(l)

	s := &Session{
		t:              t,
		NATSTestClient: c,
		s:              serv,
		conns:          make(map[*Conn]struct{}),
		CountLogger:    l,
		unsubs:         make(chan string, 256),
	}

	// Set on WS close handler to synchronize tests with WebSocket disconnects.
	serv.SetOnWSClose(func(_ *websocket.Conn) {
		ch := s.dcCh
		s.dcCh = nil
		if ch != nil {
			close(ch)
		}
	})
	serv.SetOnUnsubscribe(func(rid string) {
		s.unsubs <- rid
	})

	if err := serv.Start(); err != nil {
		panic("test: failed to start server: " + err.Error())
	}

	return s
}

// ConnectWithChannel makes a new mock client websocket connection
// with a ClientEvent channel.
func (s *Session) ConnectWithChannel(evs chan *ClientEvent) *Conn {
	return assertConnect(s.connect(evs, nil))
}

func (s *Session) connect(evs chan *ClientEvent, h http.Header) (*Conn, *http.Response, error) {
	d := wstest.NewDialer(s.s.GetWSHandlerFunc())
	c, response, err := d.Dial("ws://example.org/", h)
	if err != nil {
		return nil, response, err
	}

	conn := NewConn(s, d, c, evs)
	s.conns[conn] = struct{}{}
	return conn, response, nil
}

func assertConnect(c *Conn, r *http.Response, err error) *Conn {
	if err != nil {
		panic(err)
	}
	if c == nil {
		panic(fmt.Errorf("connection responded with status %d", r.StatusCode))
	}
	return c
}

// Connect makes a new mock client websocket connection
// that handshakes with version v1.999.999.
func (s *Session) Connect() *Conn {
	c := assertConnect(s.connect(make(chan *ClientEvent, 256), nil))

	// Send version connect
	creq := c.Request("version", versionRequest)
	cresp := creq.GetResponse(s.t)
	cresp.AssertResult(s.t, versionResult)
	return c
}

// ConnectWithVersion makes a new mock client websocket connection
// that handshakes with the version string provided.
func (s *Session) ConnectWithVersion(version string) *Conn {
	c := assertConnect(s.connect(make(chan *ClientEvent, 256), nil))

	// Send version connect
	creq := c.Request("version", struct {
		Protocol string `json:"protocol"`
	}{version})
	cresp := creq.GetResponse(s.t)
	cresp.AssertResult(s.t, versionResult)
	return c
}

// ConnectWithoutVersion makes a new mock client websocket connection
// without any version handshake.
func (s *Session) ConnectWithoutVersion() *Conn {
	return s.ConnectWithChannel(make(chan *ClientEvent, 256))
}

// ConnectWithHeader makes a new mock client websocket connection
// using provided headers. It does not send a version handshake.
func (s *Session) ConnectWithHeader(h http.Header) *Conn {
	return assertConnect(s.connect(make(chan *ClientEvent, 256), h))
}

// ConnectWithResponse makes a new mock client websocket connection that
// handshakes with version v1.999.999, if a connection is established. If an
// error occurs, it returns the error without handshake.
func (s *Session) ConnectWithResponse() (*Conn, *http.Response, error) {
	c, resp, err := s.connect(make(chan *ClientEvent, 256), nil)
	if err != nil {
		return c, resp, err
	}

	// Send version connect
	creq := c.Request("version", versionRequest)
	cresp := creq.GetResponse(s.t)
	cresp.AssertResult(s.t, versionResult)
	return c, resp, err
}

func (s *Session) IsBadHandshake(err error) bool {
	return errors.Is(err, websocket.ErrBadHandshake)
}

// HTTPRequest sends a request over HTTP
func (s *Session) HTTPRequest(method, url string, body []byte, opts ...func(r *http.Request)) *HTTPRequest {
	r := bytes.NewReader(body)

	req, err := http.NewRequest(method, url, r)
	if err != nil {
		panic("test: failed to create new http request: " + err.Error())
	}

	for _, opt := range opts {
		opt(req)
	}

	// Record the response into a httptest.ResponseRecorder
	rr := httptest.NewRecorder()

	hr := &HTTPRequest{
		req: req,
		rr:  rr,
		ch:  make(chan *HTTPResponse, 1),
	}

	go func() {
		s.Tracef("H-> %s %s: %s", method, url, body)
		s.s.ServeHTTP(rr, req)
		s.Tracef("<-H %s %s: (%d) %s", method, url, rr.Code, rr.Body.String())
		hr.ch <- &HTTPResponse{ResponseRecorder: rr}
	}()

	return hr
}

// MetricsHTTPRequest sends a request over HTTP to the metrics handler.
func (s *Session) MetricsHTTPRequest(opts ...func(r *http.Request)) *http.Response {
	req, err := http.NewRequest(http.MethodGet, server.MetricsPattern, nil)
	if err != nil {
		panic("test: failed to create new metrics http request: " + err.Error())
	}

	for _, opt := range opts {
		opt(req)
	}

	// Record the response into a httptest.ResponseRecorder
	rr := httptest.NewRecorder()

	s.Tracef("M-> GET %s", server.MetricsPattern)
	s.s.MetricsHandler().ServeHTTP(rr, req)
	s.Tracef("<-M GET %s: (%d)\n%s", server.MetricsPattern, rr.Code, rr.Body.String())

	if rr.Code != http.StatusOK {
		s.t.Fatalf("metrics http request responded with code %d", rr.Code)
	}

	return rr.Result()
}

// AssertUnsubscribe awaits for one or more resources to be unsubscribed by the
// cache, and asserts that they match the provided resource IDs.
func (s *Session) AssertUnsubscribe(rids ...string) *Session {
	if len(rids) == 0 {
		return s
	}

	var rs []string
	for count := 0; count < len(rids); count++ {
		select {
		case r := <-s.unsubs:
			rs = append(rs, r)
		case <-time.After(3 * time.Second):
			if s.t != nil {
				s.t.Fatalf("expected %d resource(s) to be unsubscribed from cache, but got %d", len(rids), count)
			} else {
				panic("test: assert unsubscribe called outside of test")
			}
		}
	}

	sort.StringSlice(rs).Sort()
	sort.StringSlice(rids).Sort()

NextRID:
	for _, rid := range rids {
		for _, r := range rs {
			if rid == r {
				continue NextRID
			}
		}
		if s.t != nil {
			s.t.Fatalf("expected unsubscribed resources:\n\t%#v\nbut got :\n\t%#v", rids, rs)
		} else {
			panic("test: assert unsubscribe called outside of test")
		}
	}
	return s
}

// AwaitUnsubscribe awaits for a resource to be unsubscribed by the cache,
// returning the resource ID.
func (s *Session) AwaitUnsubscribe() string {
	var rid string
	select {
	case rid = <-s.unsubs:
	case <-time.After(3 * time.Second):
		if s.t != nil {
			s.t.Fatalf("expected a cache unsubscribe but got none.")
		} else {
			panic("test: await unsubscribe timeout")
		}
	}
	return rid
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
	if s.t != nil {
		s.AssertNoErrorsLogged(s.t)
	}
}

// DefaultConfig returns a default server configuration used for testing
func DefaultConfig(cfgs ...func(*server.Config)) server.Config {
	var cfg server.Config
	cfg.SetDefault()
	cfg.NoHTTP = true
	for _, cb := range cfgs {
		cb(&cfg)
	}
	return cfg
}

func runTest(t *testing.T, cb func(*Session), cfgs ...func(*server.Config)) {
	runNamedTest(t, "", cb, cfgs...)
}

func runNamedTest(t *testing.T, name string, cb func(*Session), cfgs ...func(*server.Config)) {
	var s *Session
	panicked := true
	defer func() {
		if panicked {
			if name != "" {
				t.Logf("Failed test %s", name)
			}
			t.Logf("Trace log:\n%s", s.l)
		}
	}()

	s = setup(t, cfgs...)
	cb(s)
	teardown(s)

	panicked = false
}
