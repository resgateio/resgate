// Tests for responses containing meta data.
package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/resgateio/resgate/server"
	"github.com/resgateio/resgate/server/reserr"
)

func TestMeta_HTTPGetRequestWithMeta_ExpectedResponse(t *testing.T) {
	model := resourceData("test.model")
	origin := "http://example.com"
	href := "http://example.com/test/ref"
	customError := `{"code":"system.custom","message":"Custom"}`

	tbl := []struct {
		Name            string              // Name of the test
		AuthResponse    []byte              // Raw json auth response
		AccessResponse  []byte              // Raw json access response. nil means no expected access request.
		GetResponse     []byte              // Raw json get response. nil means no expected get request.
		ExpectedStatus  int                 // Expected response Status
		ExpectedBody    interface{}         // Expected response Body
		ExpectedHeaders map[string][]string // Expected response Headers
	}{
		// Auth header only
		{
			Name:            "custom header in auth response",
			AuthResponse:    []byte(`{"result":null,"meta":{"header":{"Test-Header":["foo"]}}}`),
			AccessResponse:  []byte(`{"result":{"get":true}}`),
			GetResponse:     []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus:  http.StatusOK,
			ExpectedBody:    json.RawMessage(model),
			ExpectedHeaders: map[string][]string{"Test-Header": {"foo"}},
		},
		{
			Name:            "canonicalization of header in auth response",
			AuthResponse:    []byte(`{"result":null,"meta":{"header":{"test-header":["foo"]}}}`),
			AccessResponse:  []byte(`{"result":{"get":true}}`),
			GetResponse:     []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus:  http.StatusOK,
			ExpectedBody:    json.RawMessage(model),
			ExpectedHeaders: map[string][]string{"Test-Header": {"foo"}},
		},
		{
			Name:            "duplicate custom header in auth response",
			AuthResponse:    []byte(`{"result":null,"meta":{"header":{"Test-Header":["foo","bar"]}}}`),
			AccessResponse:  []byte(`{"result":{"get":true}}`),
			GetResponse:     []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus:  http.StatusOK,
			ExpectedBody:    json.RawMessage(model),
			ExpectedHeaders: map[string][]string{"Test-Header": {"foo", "bar"}},
		},
		{
			Name:            "Set-Cookie header in auth response",
			AuthResponse:    []byte(`{"result":null,"meta":{"header":{"Set-Cookie":["id=foo; Max-Age=86400"]}}}`),
			AccessResponse:  []byte(`{"result":{"get":true}}`),
			GetResponse:     []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus:  http.StatusOK,
			ExpectedBody:    json.RawMessage(model),
			ExpectedHeaders: map[string][]string{"Set-Cookie": {"id=foo; Max-Age=86400"}},
		},
		// Auth status code
		{
			Name:            "3XX (307) status code in auth response",
			AuthResponse:    []byte(`{"result":null,"meta":{"status":307,"header":{"Location":["` + href + `"]}}}`),
			ExpectedStatus:  http.StatusTemporaryRedirect,
			ExpectedHeaders: map[string][]string{"Location": {href}},
		},
		{
			Name:            "resource reference and 3XX (307) status code in auth response",
			AuthResponse:    []byte(`{"resource":{"rid":"test.redirect"},"meta":{"status":307}}`),
			ExpectedStatus:  http.StatusTemporaryRedirect,
			ExpectedHeaders: map[string][]string{"Location": {"/api/test/redirect"}},
		},
		{
			Name:           "error status code (4XX) with custom error in auth response",
			AuthResponse:   []byte(`{"error":` + customError + `,"meta":{"status":407}}`),
			ExpectedStatus: http.StatusProxyAuthRequired,
			ExpectedBody:   json.RawMessage(customError),
		},
		{
			Name:           "error status code (5XX) with custom error in auth response",
			AuthResponse:   []byte(`{"error":` + customError + `,"meta":{"status":507}}`),
			ExpectedStatus: http.StatusInsufficientStorage,
			ExpectedBody:   json.RawMessage(customError),
		},
		{
			Name:            "error status code with custom header in auth response",
			AuthResponse:    []byte(`{"error":` + customError + `,"meta":{"status":407,"header":{"Test-Header":["foo"]}}}`),
			ExpectedStatus:  http.StatusProxyAuthRequired,
			ExpectedBody:    json.RawMessage(customError),
			ExpectedHeaders: map[string][]string{"Test-Header": {"foo"}},
		},
		// Access header only
		{
			Name:            "custom header in access response",
			AuthResponse:    []byte(`{"result":null}`),
			AccessResponse:  []byte(`{"result":{"get":true},"meta":{"header":{"Test-Header":["foo"]}}}`),
			GetResponse:     []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus:  http.StatusOK,
			ExpectedBody:    json.RawMessage(model),
			ExpectedHeaders: map[string][]string{"Test-Header": {"foo"}},
		},
		{
			Name:            "canonicalization of header in access response",
			AuthResponse:    []byte(`{"result":null}`),
			AccessResponse:  []byte(`{"result":{"get":true},"meta":{"header":{"test-header":["foo"]}}}`),
			GetResponse:     []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus:  http.StatusOK,
			ExpectedBody:    json.RawMessage(model),
			ExpectedHeaders: map[string][]string{"Test-Header": {"foo"}},
		},
		{
			Name:            "duplicate custom header in access response",
			AuthResponse:    []byte(`{"result":null}`),
			AccessResponse:  []byte(`{"result":{"get":true},"meta":{"header":{"Test-Header":["foo","bar"]}}}`),
			GetResponse:     []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus:  http.StatusOK,
			ExpectedBody:    json.RawMessage(model),
			ExpectedHeaders: map[string][]string{"Test-Header": {"foo", "bar"}},
		},
		{
			Name:            "Set-Cookie header in access response",
			AuthResponse:    []byte(`{"result":null}`),
			AccessResponse:  []byte(`{"result":{"get":true},"meta":{"header":{"Set-Cookie":["id=foo; Max-Age=86400"]}}}`),
			GetResponse:     []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus:  http.StatusOK,
			ExpectedBody:    json.RawMessage(model),
			ExpectedHeaders: map[string][]string{"Set-Cookie": {"id=foo; Max-Age=86400"}},
		},
		{
			Name:            "duplicate custom header in auth and access response overrides auth",
			AuthResponse:    []byte(`{"result":null,"meta":{"header":{"Test-Header":["foo"]}}}`),
			AccessResponse:  []byte(`{"result":{"get":true},"meta":{"header":{"Test-Header":["bar"]}}}`),
			GetResponse:     []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus:  http.StatusOK,
			ExpectedBody:    json.RawMessage(model),
			ExpectedHeaders: map[string][]string{"Test-Header": {"bar"}},
		},
		{
			Name:            "custom headers in auth and access response",
			AuthResponse:    []byte(`{"result":null,"meta":{"header":{"Test-Auth":["foo"]}}}`),
			AccessResponse:  []byte(`{"result":{"get":true},"meta":{"header":{"Test-Access":["bar"]}}}`),
			GetResponse:     []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus:  http.StatusOK,
			ExpectedBody:    json.RawMessage(model),
			ExpectedHeaders: map[string][]string{"Test-Auth": {"foo"}, "Test-Access": {"bar"}},
		},
		{
			Name:            "Set-Cookie header in both auth and access",
			AuthResponse:    []byte(`{"result":null,"meta":{"header":{"Set-Cookie":["id=foo; Max-Age=86400"]}}}`),
			AccessResponse:  []byte(`{"result":{"get":true},"meta":{"header":{"Set-Cookie":["id=bar; Max-Age=43200"]}}}`),
			GetResponse:     []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus:  http.StatusOK,
			ExpectedBody:    json.RawMessage(model),
			ExpectedHeaders: map[string][]string{"Set-Cookie": {"id=foo; Max-Age=86400", "id=bar; Max-Age=43200"}},
		},
		// Access status code
		{
			Name:            "3XX (307) status code in access response",
			AuthResponse:    []byte(`{"result":null}`),
			AccessResponse:  []byte(`{"result":{"get":true},"meta":{"status":307,"header":{"Location":["` + href + `"]}}}`),
			ExpectedStatus:  http.StatusTemporaryRedirect,
			ExpectedHeaders: map[string][]string{"Location": {href}},
		},
		{
			Name:            "resource reference in auth and 3XX (307) status code in access response does not redirect to resource",
			AuthResponse:    []byte(`{"resource":{"rid":"test.redirect"}}`),
			AccessResponse:  []byte(`{"result":{"get":true},"meta":{"status":307}}`),
			ExpectedStatus:  http.StatusTemporaryRedirect,
			ExpectedHeaders: map[string][]string{"Location": nil},
		},
		{
			Name:           "error status code (4XX) with custom error in access response",
			AuthResponse:   []byte(`{"result":null}`),
			AccessResponse: []byte(`{"error":` + customError + `,"meta":{"status":407}}`),
			ExpectedStatus: http.StatusProxyAuthRequired,
			ExpectedBody:   json.RawMessage(customError),
		},
		{
			Name:           "error status code (5XX) with custom error in access response",
			AuthResponse:   []byte(`{"result":null}`),
			AccessResponse: []byte(`{"error":` + customError + `,"meta":{"status":507}}`),
			ExpectedStatus: http.StatusInsufficientStorage,
			ExpectedBody:   json.RawMessage(customError),
		},
		{
			Name:            "error status code with custom header in access response",
			AuthResponse:    []byte(`{"result":null}`),
			AccessResponse:  []byte(`{"error":` + customError + `,"meta":{"status":407,"header":{"Test-Header":["foo"]}}}`),
			ExpectedStatus:  http.StatusProxyAuthRequired,
			ExpectedBody:    json.RawMessage(customError),
			ExpectedHeaders: map[string][]string{"Test-Header": {"foo"}},
		},
		{
			Name:            "custom header in auth response and error status code in access response",
			AuthResponse:    []byte(`{"result":null,"meta":{"header":{"Test-Header":["foo"]}}}`),
			AccessResponse:  []byte(`{"error":` + customError + `,"meta":{"status":407}}`),
			ExpectedStatus:  http.StatusProxyAuthRequired,
			ExpectedBody:    json.RawMessage(customError),
			ExpectedHeaders: map[string][]string{"Test-Header": {"foo"}},
		},

		// Invalid meta
		{
			Name:           "protected headers in auth response not overridden or included",
			AuthResponse:   []byte(`{"result":null,"meta":{"header":{"Sec-Websocket-Extensions":["foo"],"Sec-Websocket-Protocol":["foo"],"Access-Control-Allow-Credentials":["foo"],"Access-Control-Allow-Origin":["foo"],"Content-Type":["text/html; charset=utf-8"]}}}`),
			AccessResponse: []byte(`{"result":{"get":true}}`),
			GetResponse:    []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   json.RawMessage(model),
			ExpectedHeaders: map[string][]string{
				"Sec-Websocket-Extensions":         nil,
				"Sec-Websocket-Protocol":           nil,
				"Access-Control-Allow-Credentials": {"true"},
				"Access-Control-Allow-Origin":      {origin},
				"Content-Type":                     {"application/json; charset=utf-8"},
			},
		},
		{
			Name:           "2XX (206) status code in auth response is ignored",
			AuthResponse:   []byte(`{"result":null,"meta":{"status":206}}`),
			AccessResponse: []byte(`{"result":{"get":true}}`),
			GetResponse:    []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   json.RawMessage(model),
		},
		{
			Name:           "invalid status code in auth response is ignored",
			AuthResponse:   []byte(`{"result":null,"meta":{"status":601}}`),
			AccessResponse: []byte(`{"result":{"get":true}}`),
			GetResponse:    []byte(`{"result":{"model":` + model + `}}`),
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   json.RawMessage(model),
		},
	}

	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d - %s", i+1, l.Name), func(s *Session) {
			hreq := s.HTTPRequest("GET", "/api/test/model", nil, func(req *http.Request) {
				req.Header.Set("Origin", origin)
			})

			// Handle auth request
			s.GetRequest(t).
				AssertSubject(t, "auth.vault.method").
				AssertPathPayload(t, "isHttp", true).
				RespondRaw(l.AuthResponse)

			if l.AccessResponse != nil || l.GetResponse != nil {
				// Handle model get and access request
				mreqs := s.GetParallelRequests(t, 2)
				mreqs.
					GetRequest(t, "access.test.model").
					AssertPathPayload(t, "isHttp", true).
					RespondRaw(l.AccessResponse)
				mreqs.
					GetRequest(t, "get.test.model").
					RespondRaw(l.GetResponse)
			}

			// Validate http response
			hreq.GetResponse(t).
				AssertStatusCode(t, l.ExpectedStatus).
				AssertMultiHeaders(t, l.ExpectedHeaders).
				AssertBody(t, l.ExpectedBody)
		}, func(cfg *server.Config) {
			headerAuth := "vault.method"
			cfg.HeaderAuth = &headerAuth
			cfg.AllowOrigin = &origin
		})
	}
}

func TestMeta_HTTPGetRequestWithMetaErrorStatus_ExpectedError(t *testing.T) {
	tbl := []struct {
		Status        int
		ExpectedError error
	}{
		// Auth errors from 4XX status code
		// Access denied
		{401, reserr.ErrAccessDenied},
		{402, reserr.ErrAccessDenied},
		{407, reserr.ErrAccessDenied},
		// Forbidden
		{403, reserr.ErrForbidden},
		{451, reserr.ErrForbidden},
		// Not found
		{410, reserr.ErrNotFound},
		{404, reserr.ErrNotFound},
		// Method not allowed
		{405, reserr.ErrMethodNotAllowed},
		// Timeout
		{408, reserr.ErrTimeout},
		// Bad request
		{400, reserr.ErrBadRequest},
		{406, reserr.ErrBadRequest},
		{409, reserr.ErrBadRequest},
		{411, reserr.ErrBadRequest},
		// Auth errors from 5XX status code
		// Access denied
		{501, reserr.ErrNotImplemented},
		// Service unavailable
		{503, reserr.ErrServiceUnavailable},
		// Timeout
		{504, reserr.ErrTimeout},
		// Internal error
		{500, reserr.ErrInternalError},
		{505, reserr.ErrInternalError},
		{506, reserr.ErrInternalError},
		{507, reserr.ErrInternalError},
		{508, reserr.ErrInternalError},
		{511, reserr.ErrInternalError},
	}

	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d - status %d", i+1, l.Status), func(s *Session) {
			hreq := s.HTTPRequest("GET", "/api/test/model", nil)

			// Handle auth request
			s.GetRequest(t).
				AssertSubject(t, "auth.vault.method").
				AssertPathPayload(t, "isHttp", true).
				RespondRaw([]byte(`{"result":null,"meta":{"status":` + fmt.Sprint(l.Status) + `}}`))

			// Validate http response
			hreq.GetResponse(t).
				AssertStatusCode(t, l.Status).
				AssertBody(t, l.ExpectedError)
		}, func(cfg *server.Config) {
			headerAuth := "vault.method"
			cfg.HeaderAuth = &headerAuth
		})
	}
}
