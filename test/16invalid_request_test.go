package test

import (
	"testing"

	"github.com/jirenius/resgate/reserr"
)

// Test responses to invalid client requests
func TestResponseOnInvalidRequests(t *testing.T) {
	tbl := []struct {
		Method   string
		Params   interface{}
		Expected interface{}
	}{
		{"", nil, reserr.ErrMethodNotFound},
		{"test", nil, reserr.ErrMethodNotFound},
		{"new", nil, reserr.ErrMethodNotFound},
		{"unknown.test", nil, reserr.ErrMethodNotFound},
		{"call.test", nil, reserr.ErrMethodNotFound},
		{"auth.test", nil, reserr.ErrMethodNotFound},
		{"subscribe..test.model", nil, reserr.ErrMethodNotFound},
		{"subscribe.test..model", nil, reserr.ErrMethodNotFound},
		{"subscribe.test.model.", nil, reserr.ErrMethodNotFound},
		{"subscribe?foo=bar", nil, reserr.ErrMethodNotFound},
		{"subscribe.test\tmodel", nil, reserr.ErrMethodNotFound},
		{"subscribe.test\nmodel", nil, reserr.ErrMethodNotFound},
		{"subscribe.test\rmodel", nil, reserr.ErrMethodNotFound},
		{"subscribe.test model", nil, reserr.ErrMethodNotFound},
		{"subscribe.test\ufffdmodel", nil, reserr.ErrMethodNotFound},
	}

	for i, l := range tbl {
		runTest(t, func(s *Session) {
			panicked := true
			defer func() {
				if panicked {
					t.Logf("Error in test %d", i)
				}
			}()

			c := s.Connect()
			creq := c.Request(l.Method, l.Params)

			// Validate client response
			cresp := creq.GetResponse(t)
			if err, ok := l.Expected.(*reserr.Error); ok {
				cresp.AssertError(t, err)
			} else if code, ok := l.Expected.(string); ok {
				cresp.AssertErrorCode(t, code)
			} else {
				cresp.AssertResult(t, l.Expected)
			}

			panicked = false
		})
	}
}
