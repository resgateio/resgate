package test

import (
	"encoding/json"
	"testing"

	"github.com/jirenius/resgate/server/mq"
	"github.com/jirenius/resgate/server/reserr"
)

// Test responses to client auth requests
func TestAuthOnResource(t *testing.T) {

	params := json.RawMessage(`{"value":42}`)
	successResponse := json.RawMessage(`{"foo":"bar"}`)

	tbl := []struct {
		Params       interface{} // Params to use in call request
		AuthResponse interface{} // Response on call request. requestTimeout means timeout.
		Expected     interface{}
	}{
		{nil, successResponse, successResponse},
		{nil, reserr.ErrInvalidParams, reserr.ErrInvalidParams},
		{nil, nil, nil},
		{nil, requestTimeout, mq.ErrRequestTimeout},
		{params, successResponse, successResponse},
		// Invalid service responses
		{nil, []byte(`{"broken":JSON}`), reserr.CodeInternalError},
		{nil, []byte(`{}`), reserr.CodeInternalError},
		{nil, []byte(`{"result":{"foo":"bar"},"error":{"code":"system.custom","message":"Custom"}}`), "system.custom"},
		// Invalid auth error response
		{nil, []byte(`{"error":[]}`), reserr.CodeInternalError},
		{nil, []byte(`{"error":{"message":"missing code"}}`), ""},
		{nil, []byte(`{"error":{"code":12,"message":"integer code"}}`), reserr.CodeInternalError},
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

			// Send client call request
			creq := c.Request("auth.test.model.method", l.Params)

			// Get call request
			req := s.GetRequest(t)
			req.AssertSubject(t, "auth.test.model.method")
			req.AssertPathType(t, "cid", string(""))
			req.AssertPathPayload(t, "token", nil)
			req.AssertPathType(t, "header", map[string]interface{}(nil))
			req.AssertPathType(t, "host", string(""))
			req.AssertPathType(t, "uri", string(""))
			req.AssertPathPayload(t, "params", l.Params)
			if l.AuthResponse == requestTimeout {
				req.Timeout()
			} else if err, ok := l.AuthResponse.(*reserr.Error); ok {
				req.RespondError(err)
			} else if raw, ok := l.AuthResponse.([]byte); ok {
				req.RespondRaw(raw)
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
