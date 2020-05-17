package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/raphaelpereira/resgate/server"
)

func TestHTTPOptions_AllowOrigin_ExpectedResponseHeaders(t *testing.T) {
	tbl := []struct {
		Origin                 string            // Request's Origin header. Empty means no Origin header.
		AllowOrigin            string            // AllowOrigin config
		ExpectedHeaders        map[string]string // Expected response Headers
		ExpectedMissingHeaders []string          // Expected response headers not to be included
	}{
		{"http://localhost", "*", map[string]string{"Access-Control-Allow-Origin": "*"}, []string{"Vary"}},
		{"http://localhost", "http://localhost", map[string]string{"Access-Control-Allow-Origin": "http://localhost", "Vary": "Origin"}, nil},
		{"https://resgate.io", "http://localhost;https://resgate.io", map[string]string{"Access-Control-Allow-Origin": "https://resgate.io", "Vary": "Origin"}, nil},
		{"http://example.com", "http://localhost;https://resgate.io", map[string]string{"Access-Control-Allow-Origin": "http://localhost", "Vary": "Origin"}, nil},
		// No Origin header in request
		{"", "*", map[string]string{"Access-Control-Allow-Origin": "*"}, []string{"Vary"}},
		{"", "http://localhost", nil, []string{"Access-Control-Allow-Origin", "Vary"}},
	}

	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			hreq := s.HTTPRequest("OPTIONS", "/api/test/model", nil, func(req *http.Request) {
				if l.Origin != "" {
					req.Header.Set("Origin", l.Origin)
				}
			})
			// Validate http response
			hreq.GetResponse(t).
				Equals(t, http.StatusOK, nil).
				AssertHeaders(t, l.ExpectedHeaders).
				AssertMissingHeaders(t, l.ExpectedMissingHeaders)
		}, func(cfg *server.Config) {
			cfg.AllowOrigin = &l.AllowOrigin
		})
	}
}
