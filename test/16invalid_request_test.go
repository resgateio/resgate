package test

import (
	"testing"

	"github.com/jirenius/resgate/server/reserr"
)

// Test responses to invalid client requests
func TestResponseOnInvalidRequests(t *testing.T) {
	tbl := []struct {
		Method   string
		Params   interface{}
		Expected interface{}
	}{
		{"", nil, reserr.ErrInvalidRequest},
		{"test", nil, reserr.ErrInvalidRequest},
		{"new", nil, reserr.ErrInvalidRequest},
		{"unknown.test", nil, reserr.ErrInvalidRequest},
		{"call.test", nil, reserr.ErrInvalidRequest},
		{"call.test", nil, reserr.ErrInvalidRequest},
		{"call.test.methöd", nil, reserr.ErrInvalidRequest},
		{"call.test?foo", nil, reserr.ErrInvalidRequest},
		{"call.test.method?foo", nil, reserr.ErrInvalidRequest},
		{"auth.test", nil, reserr.ErrInvalidRequest},
		{"subscribe..test.model", nil, reserr.ErrInvalidRequest},
		{"subscribe.test..model", nil, reserr.ErrInvalidRequest},
		{"subscribe.test.model.", nil, reserr.ErrInvalidRequest},
		{".subscribe.test.model", nil, reserr.ErrInvalidRequest},
		{"subscribe?foo=bar", nil, reserr.ErrInvalidRequest},
		{"subscribe.test\tmodel", nil, reserr.ErrInvalidRequest},
		{"subscribe.test\nmodel", nil, reserr.ErrInvalidRequest},
		{"subscribe.test\rmodel", nil, reserr.ErrInvalidRequest},
		{"subscribe.test model", nil, reserr.ErrInvalidRequest},
		{"subscribe.test\ufffdmodel", nil, reserr.ErrInvalidRequest},
		{"subscribe.täst.model", nil, reserr.ErrInvalidRequest},
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
