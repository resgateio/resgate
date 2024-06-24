package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/resgateio/resgate/server"
	"github.com/resgateio/resgate/server/reserr"
)

// Test subscribing to a resource with WebSocket compression enabled
func TestWebSocketOptions_WithCompressionEnabled_CanSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModel(t, s, c)
	}, func(cfg *server.Config) {
		cfg.WSCompression = true
	})
}

// Test that an auth request is sent on connect when WSHeaderAuth is set, and
// that the response of a subsequent get request is as expected.
func TestWebSocketOptions_WSHeaderAuth_ExpectedResponse(t *testing.T) {
	model := resourceData("test.model")
	token := json.RawMessage(`{"user":"foo"}`)

	tbl := []struct {
		AuthResponse interface{} // Response on auth request. requestTimeout means timeout.
		Token        interface{} // Token to send. noToken means no token events should be sent.
	}{
		// Without token
		{requestTimeout, noToken},
		{reserr.ErrNotFound, noToken},
		{[]byte(`{]`), noToken},
		{nil, noToken},
		// With token
		{requestTimeout, token},
		{reserr.ErrNotFound, token},
		{[]byte(`{]`), token},
		{nil, token},
		// With nil token
		{requestTimeout, nil},
		{reserr.ErrNotFound, nil},
		{[]byte(`{]`), nil},
		{nil, nil},
	}

	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			authDone := make(chan struct{})
			expectedToken := l.Token

			// Create LogTesting to log errors in goroutine
			logt := &LogTesting{
				NoPanic: true,
			}

			//  Handle requests send during Connect
			go func() {
				defer close(authDone)
				defer logt.Defer()
				req := s.GetRequest(logt)
				req.AssertSubject(logt, "auth.vault.method")
				req.AssertPathPayload(logt, "isHttp", true)
				// Send token
				if l.Token != noToken {
					cid := req.PathPayload(logt, "cid").(string)
					s.ConnEvent(cid, "token", struct {
						Token interface{} `json:"token"`
					}{l.Token})
				} else {
					expectedToken = nil
				}
				// Respond to auth request
				if l.AuthResponse == requestTimeout {
					req.Timeout()
				} else if err, ok := l.AuthResponse.(*reserr.Error); ok {
					req.RespondError(err)
				} else if raw, ok := l.AuthResponse.([]byte); ok {
					req.RespondRaw(raw)
				} else {
					req.RespondSuccess(l.AuthResponse)
				}
			}()

			c := s.Connect()

			<-authDone

			// Check for errors during connect
			if logt.Err != nil {
				t.Fatal(logt.Err)
			}

			// Send subscribe request
			creq := c.Request("subscribe.test.model", nil)

			// Handle model get and access request
			mreqs := s.GetParallelRequests(t, 2)
			mreqs.
				GetRequest(t, "access.test.model").
				AssertPathPayload(t, "token", expectedToken).
				AssertPathMissing(t, "isHttp").
				RespondSuccess(json.RawMessage(`{"get":true}`))
			mreqs.
				GetRequest(t, "get.test.model").
				RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

			// Get client response
			creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))
		}, func(cfg *server.Config) {
			headerAuth := "vault.method"
			cfg.WSHeaderAuth = &headerAuth
		})
	}
}
