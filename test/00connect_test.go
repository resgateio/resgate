package test

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/resgateio/resgate/server"
)

// Test that the server starts and stops the server without error
func TestStart(t *testing.T) {
	runTest(t, func(s *Session) {})
}

// Test that a client can connect to the server without error
func TestConnectClient(t *testing.T) {
	runTest(t, func(s *Session) {
		s.Connect()
	})
}

func TestConnect_AllowOrigin_Connects(t *testing.T) {
	tbl := []struct {
		Origin        string // Request's Origin header. Empty means no Origin header.
		AllowOrigin   string // AllowOrigin config
		ExpectConnect bool   // Expects a successful WebSocket connection/upgrade
	}{
		// Valid Origin
		{"http://localhost", "*", true},
		{"http://localhost", "http://localhost", true},
		{"http://localhost:8080", "http://localhost:8080", true},
		{"http://localhost", "http://localhost;https://resgate.io", true},
		{"https://resgate.io", "http://localhost;https://resgate.io", true},
		// Missing Origin
		{"", "*", true},
		{"", "https://resgate.io", true},
		// Invalid Origin
		{"http://resgate.io", "https://resgate.io", false},
		{"https://resgate.io", "https://api.resgate.io", false},
		{"https://resgate.io:8080", "https://resgate.io", false},
		{"https://resgate.io", "https://resgate.io:8080", false},
	}

	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			var h http.Header
			if l.Origin != "" {
				h = http.Header{"Origin": {l.Origin}}
			}
			var c *Conn
			if l.ExpectConnect {
				c = s.ConnectWithHeader(h)
				// Test sending a version request
				creq := c.Request("version", versionRequest)
				creq.GetResponse(s.t)
			} else {
				AssertPanic(t, func() {
					c = s.ConnectWithHeader(h)
				})
			}
		}, func(cfg *server.Config) {
			cfg.AllowOrigin = &l.AllowOrigin
		})
	}
}

// Test that the server responds with 400 on request-target being *.
func TestStart_WithAsteriskAsRequestURI_BadRequestStatusResponse(t *testing.T) {
	runTest(t, func(s *Session) {
		hreq := s.HTTPRequest("GET", "", nil, func(r *http.Request) {
			r.RequestURI = "*"
		})
		hreq.GetResponse(t).AssertStatusCode(t, http.StatusBadRequest)
	})
}

// Test that the server starts and stops without error when enabling the HTTP server
func TestStart_WithHTTPServer_NoErrors(t *testing.T) {
	ref := os.Getenv("RESGATE_TEST_EXTENDED")
	if ref == "" {
		t.Skip("no RESGATE_TEST_EXTENDED environment value")
	}
	runTest(t, func(s *Session) {}, func(cfg *server.Config) {
		cfg.NoHTTP = false
		cfg.Port = 58080
		cfg.MetricsPort = 58090
	})
}
