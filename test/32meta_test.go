// Tests for responses containing meta data.
package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/resgateio/resgate/server"
)

func TestMeta_WSHeaderAuthWithMeta_ExpectedResponse(t *testing.T) {
	origin := "http://example.com"
	// href := "http://example.com/test/ref"
	customError := `{"code":"system.custom","message":"Custom"}`

	tbl := []struct {
		Name           string // Name of the test
		AuthResponse   []byte // Raw json auth response
		ExpectedStatus int    // Expected response Status
		// The websocket test client does seem to allow a body. Because of that,
		// these tests is not working properly and ExpectBody has been commented
		// out.
		// ExpectedBody    interface{}         // Expected response Body
		ExpectedHeaders map[string][]string // Expected response Headers
		ExpectedErrors  int                 // Expected logged errors
	}{
		// Auth header
		{
			Name:            "custom header in auth response",
			AuthResponse:    []byte(`{"result":null,"meta":{"header":{"Test-Header":["foo"]}}}`),
			ExpectedStatus:  http.StatusSwitchingProtocols,
			ExpectedHeaders: map[string][]string{"Test-Header": {"foo"}},
		},
		{
			Name:            "Set-Cookie header in auth response",
			AuthResponse:    []byte(`{"result":null,"meta":{"header":{"Set-Cookie":["id=foo; Max-Age=86400"]}}}`),
			ExpectedStatus:  http.StatusSwitchingProtocols,
			ExpectedHeaders: map[string][]string{"Set-Cookie": {"id=foo; Max-Age=86400"}},
		},

		// Auth status code

		// The websocket client does not record headers in the response on bad handshake.
		// Because of that, these tests is not working properly and has been commented out.
		// {
		// 	Name:            "3XX (307) status code in auth response",
		// 	AuthResponse:    []byte(`{"result":null,"meta":{"status":307,"header":{"Location":["` + href + `"]}}}`),
		// 	ExpectedStatus:  http.StatusTemporaryRedirect,
		// 	ExpectedHeaders: map[string][]string{"Location": {href}},
		// },
		// {
		// 	Name:            "resource reference and 3XX (307) status code in auth response",
		// 	AuthResponse:    []byte(`{"resource":{"rid":"test.redirect"},"meta":{"status":307}}`),
		// 	ExpectedStatus:  http.StatusTemporaryRedirect,
		// 	ExpectedHeaders: map[string][]string{"Location": {"/api/test/redirect"}},
		// },
		// {
		// 	Name:            "error status code with custom header in auth response",
		// 	AuthResponse:    []byte(`{"error":` + customError + `,"meta":{"status":407,"header":{"Test-Header":["foo"]}}}`),
		// 	ExpectedStatus:  http.StatusProxyAuthRequired,
		// 	ExpectedBody:    json.RawMessage(customError),
		// 	ExpectedHeaders: map[string][]string{"Test-Header": {"foo"}},
		// },
		{
			Name:           "error status code (4XX) with custom error in auth response",
			AuthResponse:   []byte(`{"error":` + customError + `,"meta":{"status":407}}`),
			ExpectedStatus: http.StatusProxyAuthRequired,
			// ExpectedBody:   json.RawMessage(customError),
		},
		{
			Name:           "error status code (5XX) with custom error in auth response",
			AuthResponse:   []byte(`{"error":` + customError + `,"meta":{"status":507}}`),
			ExpectedStatus: http.StatusInsufficientStorage,
			// ExpectedBody:   json.RawMessage(customError),
		},

		// Invalid meta
		{
			Name:           "protected headers in auth response not overridden or included",
			AuthResponse:   []byte(`{"result":null,"meta":{"header":{"Sec-Websocket-Extensions":["foo"],"Sec-Websocket-Protocol":["foo"],"Access-Control-Allow-Credentials":["foo"],"Access-Control-Allow-Origin":["foo"],"Content-Type":["text/html; charset=utf-8"]}}}`),
			ExpectedStatus: http.StatusSwitchingProtocols,
			ExpectedHeaders: map[string][]string{
				"Sec-Websocket-Extensions":         nil,
				"Sec-Websocket-Protocol":           nil,
				"Access-Control-Allow-Credentials": nil,
				"Access-Control-Allow-Origin":      nil,
				"Content-Type":                     nil,
			},
		},
		{
			Name:           "2XX (206) status code in auth response is ignored",
			AuthResponse:   []byte(`{"result":null,"meta":{"status":206}}`),
			ExpectedStatus: http.StatusSwitchingProtocols,
			ExpectedErrors: 1,
		},
		{
			Name:           "invalid status code in auth response is ignored",
			AuthResponse:   []byte(`{"result":null,"meta":{"status":601}}`),
			ExpectedStatus: http.StatusSwitchingProtocols,
			ExpectedErrors: 1,
		},
	}

	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d - %s", i+1, l.Name), func(s *Session) {
			authDone := make(chan struct{})

			if l.AuthResponse != nil {
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
					req.RespondRaw(l.AuthResponse)
				}()
			} else {
				close(authDone)
			}

			_, r, err := s.ConnectWithResponse()
			// If we expect an error body, we can also assume to receive a bad handshake error.
			if err != nil && !s.IsBadHandshake(err) {
				t.Fatalf("error connecting: %s", err)
			}

			<-authDone

			// Validate http response
			AssertResponseStatusCode(t, r, l.ExpectedStatus)
			AssertResponseMultiHeaders(t, r, l.ExpectedHeaders)
			// AssertResponseBody(t, r.Body, l.ExpectedBody)

			// Validated expected logged errors
			s.AssertErrorsLogged(t, l.ExpectedErrors)
		}, func(cfg *server.Config) {
			wsHeaderAuth := "vault.method"
			cfg.WSHeaderAuth = &wsHeaderAuth
			cfg.AllowOrigin = &origin
		})
	}
}
