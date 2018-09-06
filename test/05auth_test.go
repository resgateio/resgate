package test

import (
	"encoding/json"
	"testing"

	"github.com/jirenius/resgate/mq"
	"github.com/jirenius/resgate/reserr"
)

// Test responses to client auth requests
func TestAuthOnResource(t *testing.T) {

	params := json.RawMessage(`{"value":42}`)
	successResponse := json.RawMessage(`{"foo":"bar"}`)
	// Call responses
	callTimeout := &struct{}{}

	tbl := []struct {
		Params       interface{} // Params to use in call request
		AuthResponse interface{} // Response on call request. callTimeout means timeout. noCall means no call request is expected
		Expected     interface{}
	}{
		{nil, successResponse, successResponse},
		{nil, reserr.ErrInvalidParams, reserr.ErrInvalidParams},
		{nil, nil, nil},
		{nil, callTimeout, mq.ErrRequestTimeout},
		{params, successResponse, successResponse},
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
			var creq *ClientRequest

			// Send client call request
			creq = c.Request("auth.test.model.method", l.Params)

			// Get call request
			req := s.GetRequest(t)
			req.AssertSubject(t, "auth.test.model.method")
			req.AssertPathType(t, "cid", string(""))
			req.AssertPathPayload(t, "token", nil)
			req.AssertPathType(t, "header", map[string]interface{}(nil))
			req.AssertPathType(t, "host", string(""))
			req.AssertPathType(t, "uri", string(""))
			req.AssertPathPayload(t, "params", l.Params)
			if l.AuthResponse == callTimeout {
				req.Timeout()
			} else if err, ok := l.AuthResponse.(*reserr.Error); ok {
				req.RespondError(err)
			} else {
				req.RespondSuccess(l.AuthResponse)
			}

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
