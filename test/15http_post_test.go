package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/resgateio/resgate/server"
	"github.com/resgateio/resgate/server/mq"
	"github.com/resgateio/resgate/server/reserr"
)

// Test response to a HTTP POST request to a primitive query model method
func TestHTTPPostOnPrimitiveQueryModel(t *testing.T) {
	encodings := []string{"json", "jsonFlat"}

	for _, enc := range encodings {
		runNamedTest(t, enc, func(s *Session) {
			successResponse := json.RawMessage(`{"foo":"bar"}`)

			hreq := s.HTTPRequest("POST", "/api/test/model/method?q=foo&f=bar", nil)

			// Handle query model access request
			s.
				GetRequest(t).
				AssertSubject(t, "access.test.model").
				AssertPathPayload(t, "token", nil).
				AssertPathPayload(t, "query", "q=foo&f=bar").
				AssertPathPayload(t, "isHttp", true).
				RespondSuccess(json.RawMessage(`{"call":"method"}`))
			// Handle query model call request
			s.
				GetRequest(t).
				AssertSubject(t, "call.test.model.method").
				AssertPathPayload(t, "query", "q=foo&f=bar").
				AssertPathPayload(t, "isHttp", true).
				RespondSuccess(successResponse)

			// Validate http response
			hreq.GetResponse(t).Equals(t, http.StatusOK, successResponse)
		}, func(c *server.Config) {
			c.APIEncoding = enc
		})
	}
}

// Test responses to HTTP post requests
func TestHTTPPostResponses(t *testing.T) {
	params := []byte(`{"value":42}`)
	successResponse := json.RawMessage(`{"foo":"bar"}`)
	// Access responses
	fullCallAccess := json.RawMessage(`{"get":true,"call":"*"}`)
	methodCallAccess := json.RawMessage(`{"get":true,"call":"method"}`)
	multiMethodCallAccess := json.RawMessage(`{"get":true,"call":"foo,method"}`)
	missingMethodCallAccess := json.RawMessage(`{"get":true,"call":"foo,bar"}`)
	noCallAccess := json.RawMessage(`{"get":true}`)
	// Response headers
	modelLocationHref := map[string]string{"Location": "/api/test/model"}

	tbl := []struct {
		Params          []byte            // Params to use as body in post request
		AccessResponse  interface{}       // Response on access request. nil means timeout
		CallResponse    interface{}       // Response on call request. requestTimeout means timeout. noRequest means no call request is expected
		ExpectedCode    int               // Expected response status code
		ExpectedHeaders map[string]string // Expected response Headers
		Expected        interface{}       // Expected response body
	}{
		// Params variants
		{nil, fullCallAccess, successResponse, http.StatusOK, nil, successResponse},
		{params, fullCallAccess, successResponse, http.StatusOK, nil, successResponse},
		// AccessResponse variants
		{nil, methodCallAccess, successResponse, http.StatusOK, nil, successResponse},
		{nil, multiMethodCallAccess, successResponse, http.StatusOK, nil, successResponse},
		{nil, missingMethodCallAccess, noRequest, http.StatusUnauthorized, nil, reserr.ErrAccessDenied},
		{nil, noCallAccess, noRequest, http.StatusUnauthorized, nil, reserr.ErrAccessDenied},
		{nil, nil, noRequest, http.StatusNotFound, nil, mq.ErrRequestTimeout},
		// CallResponse variants
		{nil, fullCallAccess, reserr.ErrInvalidParams, http.StatusBadRequest, nil, reserr.ErrInvalidParams},
		{nil, fullCallAccess, reserr.ErrMethodNotFound, http.StatusNotFound, nil, reserr.ErrMethodNotFound},
		{nil, fullCallAccess, nil, http.StatusNoContent, nil, []byte{}},
		// Valid call resource response
		{nil, fullCallAccess, []byte(`{"resource":{"rid":"test.model"}}`), http.StatusOK, modelLocationHref, nil},
		// Invalid call resource response
		{nil, fullCallAccess, []byte(`{"resource":"test.model"}`), http.StatusInternalServerError, nil, reserr.CodeInternalError},
		{nil, fullCallAccess, []byte(`{"resource":"test.model"}`), http.StatusInternalServerError, nil, reserr.CodeInternalError},
		{nil, fullCallAccess, []byte(`{"resource":{}}`), http.StatusInternalServerError, nil, reserr.CodeInternalError},
		{nil, fullCallAccess, []byte(`{"resource":{}}`), http.StatusInternalServerError, nil, reserr.CodeInternalError},
		{nil, fullCallAccess, []byte(`{"resource":{"rid":42}}`), http.StatusInternalServerError, nil, reserr.CodeInternalError},
		{nil, fullCallAccess, []byte(`{"resource":{"rid":42}}`), http.StatusInternalServerError, nil, reserr.CodeInternalError},
		{nil, fullCallAccess, []byte(`{"resource":{"rid":"test..model"}}`), http.StatusInternalServerError, nil, reserr.CodeInternalError},
		{nil, fullCallAccess, []byte(`{"resource":{"rid":"test..model"}}`), http.StatusInternalServerError, nil, reserr.CodeInternalError},
	}

	encodings := []string{"json", "jsonFlat"}

	for _, enc := range encodings {
		for i, l := range tbl {
			runNamedTest(t, fmt.Sprintf("#%d (%s)", i+1, enc), func(s *Session) {
				// Send HTTP post request
				hreq := s.HTTPRequest("POST", "/api/test/model/method", l.Params)

				req := s.GetRequest(t)
				req.AssertSubject(t, "access.test.model")
				req.AssertPathPayload(t, "isHttp", true)
				if l.AccessResponse == nil {
					req.Timeout()
				} else if err, ok := l.AccessResponse.(*reserr.Error); ok {
					req.RespondError(err)
				} else {
					req.RespondSuccess(l.AccessResponse)
				}

				if l.CallResponse != noRequest {
					// Get call request
					req = s.GetRequest(t)
					req.AssertSubject(t, "call.test.model.method")
					req.AssertPathPayload(t, "params", json.RawMessage(l.Params))
					if l.CallResponse == requestTimeout {
						req.Timeout()
					} else if err, ok := l.CallResponse.(*reserr.Error); ok {
						req.RespondError(err)
					} else if raw, ok := l.CallResponse.([]byte); ok {
						req.RespondRaw(raw)
					} else {
						req.RespondSuccess(l.CallResponse)
					}
				}

				// Validate client response
				hresp := hreq.GetResponse(t)
				hresp.AssertStatusCode(t, l.ExpectedCode)
				if err, ok := l.Expected.(*reserr.Error); ok {
					hresp.AssertError(t, err)
				} else if code, ok := l.Expected.(string); ok {
					hresp.AssertErrorCode(t, code)
				} else {
					hresp.AssertBody(t, l.Expected)
				}

				// Validate headers
				hresp.AssertHeaders(t, l.ExpectedHeaders)

			}, func(c *server.Config) {
				c.APIEncoding = enc
			})
		}
	}
}

// Test Legacy HTTP post responses to new requests
func TestHTTPPostNewResponses(t *testing.T) {
	params := json.RawMessage(`{"value":42}`)
	legacyCallResponse := json.RawMessage(`{"rid":"test.model"}`)
	nonlegacyCallResponse := []byte(`{"resource":{"rid":"test.model"}}`)
	// Access responses
	fullCallAccess := json.RawMessage(`{"get":true,"call":"*"}`)
	methodCallAccess := json.RawMessage(`{"get":true,"call":"new"}`)
	multiMethodCallAccess := json.RawMessage(`{"get":true,"call":"foo,new"}`)
	missingMethodCallAccess := json.RawMessage(`{"get":true,"call":"foo,bar"}`)
	noCallAccess := json.RawMessage(`{"get":true}`)
	modelLocationHref := map[string]string{"Location": "/api/test/model"}

	tbl := []struct {
		Params             []byte            // Params to use as body in post request
		CallAccessResponse interface{}       // Response on access request. requestTimeout means timeout
		CallResponse       interface{}       // Response on new request. requestTimeout means timeout. noRequest means no request is expected
		ExpectedCode       int               // Expected response status code
		Expected           interface{}       // Expected response body
		ExpectedHeaders    map[string]string // Expected response Headers
		ExpectedErrors     int               // Expected logged errors
	}{
		// Params variants
		{params, fullCallAccess, legacyCallResponse, http.StatusOK, nil, modelLocationHref, 1},
		{nil, fullCallAccess, legacyCallResponse, http.StatusOK, nil, modelLocationHref, 1},
		// CallAccessResponse variants
		{params, methodCallAccess, legacyCallResponse, http.StatusOK, nil, modelLocationHref, 1},
		{params, multiMethodCallAccess, legacyCallResponse, http.StatusOK, nil, modelLocationHref, 1},
		{params, missingMethodCallAccess, noRequest, http.StatusUnauthorized, reserr.ErrAccessDenied, nil, 0},
		{params, noCallAccess, noRequest, http.StatusUnauthorized, reserr.ErrAccessDenied, nil, 0},
		{params, requestTimeout, noRequest, http.StatusNotFound, mq.ErrRequestTimeout, nil, 0},
		// CallResponse variants
		{params, fullCallAccess, reserr.ErrInvalidParams, http.StatusBadRequest, reserr.ErrInvalidParams, nil, 0},
		{params, fullCallAccess, requestTimeout, http.StatusNotFound, mq.ErrRequestTimeout, nil, 0},
		// Non-legacy call response
		{params, fullCallAccess, nonlegacyCallResponse, http.StatusOK, nil, modelLocationHref, 0},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			// Send HTTP post request
			hreq := s.HTTPRequest("POST", "/api/test/collection/new", l.Params)

			req := s.GetRequest(t)
			req.AssertSubject(t, "access.test.collection")
			if l.CallAccessResponse == requestTimeout {
				req.Timeout()
			} else if err, ok := l.CallAccessResponse.(*reserr.Error); ok {
				req.RespondError(err)
			} else {
				req.RespondSuccess(l.CallAccessResponse)
			}

			if l.CallResponse != noRequest {
				// Get call request
				req = s.GetRequest(t)
				req.AssertSubject(t, "call.test.collection.new")
				req.AssertPathPayload(t, "params", json.RawMessage(l.Params))
				if l.CallResponse == requestTimeout {
					req.Timeout()
				} else if err, ok := l.CallResponse.(*reserr.Error); ok {
					req.RespondError(err)
				} else if raw, ok := l.CallResponse.([]byte); ok {
					req.RespondRaw(raw)
				} else {
					req.RespondSuccess(l.CallResponse)
				}
			}

			// Validate HTTP post response
			hresp := hreq.GetResponse(t)
			hresp.AssertStatusCode(t, l.ExpectedCode)
			if err, ok := l.Expected.(*reserr.Error); ok {
				hresp.AssertError(t, err)
			} else if code, ok := l.Expected.(string); ok {
				hresp.AssertErrorCode(t, code)
			} else {
				hresp.AssertBody(t, l.Expected)
			}
			hresp.AssertHeaders(t, l.ExpectedHeaders)

			// Validate logged errors
			s.AssertErrorsLogged(t, l.ExpectedErrors)
		})
	}
}

// Test invalid urls for HTTP post requests
func TestHTTPPostInvalidURLs(t *testing.T) {
	tbl := []struct {
		URL          string // Url path
		ExpectedCode int
		Expected     interface{}
	}{
		{"/wrong_prefix/test/model/action", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/action", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/test.model/action", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/test/model/action/", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/test//model/action", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/test/model/äction", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/test/mådel/action", http.StatusNotFound, reserr.ErrNotFound},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			hreq := s.HTTPRequest("POST", l.URL, nil)
			hresp := hreq.
				GetResponse(t).
				AssertStatusCode(t, l.ExpectedCode)

			if l.Expected != nil {
				if err, ok := l.Expected.(*reserr.Error); ok {
					hresp.AssertError(t, err)
				} else if code, ok := l.Expected.(string); ok {
					hresp.AssertErrorCode(t, code)
				} else {
					hresp.AssertBody(t, l.Expected)
				}
			}
		})
	}
}

func TestHTTPPost_AllowOrigin_ExpectedResponse(t *testing.T) {
	successResponse := json.RawMessage(`{"get":true,"call":"*"}`)

	tbl := []struct {
		Origin                 string            // Request's Origin header. Empty means no Origin header.
		ContentType            string            // Request's Content-Type header. Empty means no Content-Type header.
		AllowOrigin            string            // AllowOrigin config
		ExpectedCode           int               // Expected response status code
		ExpectedHeaders        map[string]string // Expected response Headers
		ExpectedMissingHeaders []string          // Expected response headers not to be included
		ExpectedBody           interface{}       // Expected response body
	}{
		{"http://localhost", "", "*", http.StatusOK, map[string]string{"Access-Control-Allow-Origin": "*"}, []string{"Vary", "Access-Control-Allow-Credentials"}, successResponse},
		{"http://localhost", "", "http://localhost", http.StatusOK, map[string]string{"Access-Control-Allow-Origin": "http://localhost", "Vary": "Origin"}, []string{"Access-Control-Allow-Credentials"}, successResponse},
		{"https://resgate.io", "", "http://localhost;https://resgate.io", http.StatusOK, map[string]string{"Access-Control-Allow-Origin": "https://resgate.io", "Vary": "Origin"}, []string{"Access-Control-Allow-Credentials"}, successResponse},
		// Invalid requests
		{"http://example.com", "", "http://localhost;https://resgate.io", http.StatusForbidden, map[string]string{"Access-Control-Allow-Origin": "http://localhost", "Vary": "Origin"}, []string{"Access-Control-Allow-Credentials"}, reserr.ErrForbiddenOrigin},
		// No Origin header in request
		{"", "", "*", http.StatusOK, map[string]string{"Access-Control-Allow-Origin": "*"}, []string{"Vary"}, successResponse},
		{"", "", "http://localhost", http.StatusOK, nil, []string{"Access-Control-Allow-Origin", "Vary"}, successResponse},
	}

	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			hreq := s.HTTPRequest("POST", "/api/test/model/method", nil, func(req *http.Request) {
				if l.Origin != "" {
					req.Header.Set("Origin", l.Origin)
				}
				if l.ContentType != "" {
					req.Header.Set("Content-Type", l.ContentType)
				}
			})

			if l.ExpectedCode == http.StatusOK {
				// Get access request
				req := s.GetRequest(t)
				req.AssertSubject(t, "access.test.model")
				req.RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))
				// Get call request
				req = s.GetRequest(t)
				req.AssertSubject(t, "call.test.model.method")
				req.RespondSuccess(successResponse)
			}

			// Validate http response
			hreq.GetResponse(t).
				Equals(t, l.ExpectedCode, l.ExpectedBody).
				AssertHeaders(t, l.ExpectedHeaders).
				AssertMissingHeaders(t, l.ExpectedMissingHeaders)
		}, func(cfg *server.Config) {
			cfg.AllowOrigin = &l.AllowOrigin
		})
	}
}

func TestHTTPPost_HeaderAuth_ExpectedResponse(t *testing.T) {
	token := json.RawMessage(`{"user":"foo"}`)
	successResponse := json.RawMessage(`{"foo":"bar"}`)

	tbl := []struct {
		AuthResponse    interface{}       // Response on auth request. requestTimeout means timeout.
		Token           interface{}       // Token to send. noToken means no token events should be sent.
		ExpectedHeaders map[string]string // Expected response Headers
	}{
		// Without token
		{requestTimeout, noToken, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{reserr.ErrNotFound, noToken, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{[]byte(`{]`), noToken, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{nil, noToken, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		// With token
		{requestTimeout, token, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{reserr.ErrNotFound, token, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{[]byte(`{]`), token, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{nil, token, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		// With nil token
		{requestTimeout, nil, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{reserr.ErrNotFound, nil, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{[]byte(`{]`), nil, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{nil, nil, map[string]string{"Access-Control-Allow-Credentials": "true"}},
	}

	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			hreq := s.HTTPRequest("POST", "/api/test/model/method", nil, func(req *http.Request) {
				req.Header.Set("Origin", "example.com")
			})

			req := s.GetRequest(t)
			req.AssertSubject(t, "auth.vault.method")
			req.AssertPathPayload(t, "header.Origin", []string{"example.com"})
			// Send token
			expectedToken := l.Token
			if l.Token != noToken {
				cid := req.PathPayload(t, "cid").(string)
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

			// Handle model get and access request
			s.GetRequest(t).
				AssertSubject(t, "access.test.model").
				AssertPathPayload(t, "token", expectedToken).
				RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))
			s.GetRequest(t).
				AssertSubject(t, "call.test.model.method").
				RespondSuccess(successResponse)

			// Validate http response
			hreq.GetResponse(t).
				Equals(t, http.StatusOK, successResponse).
				AssertHeaders(t, l.ExpectedHeaders)
		}, func(cfg *server.Config) {
			headerAuth := "vault.method"
			cfg.HeaderAuth = &headerAuth
		})
	}
}

func TestHTTPPost_LongResourceID_ReturnsStatus414(t *testing.T) {
	longStr := generateString(10000)
	runTest(t, func(s *Session) {
		hreq := s.HTTPRequest("POST", "/api/test/"+longStr+"/method", nil)

		// Validate http response
		hreq.GetResponse(t).
			AssertError(t, reserr.ErrSubjectTooLong).
			AssertStatusCode(t, http.StatusRequestURITooLong)
	})
}

func TestHTTPPost_LongResourceMethod_ReturnsStatus414(t *testing.T) {
	longStr := generateString(10000)
	runTest(t, func(s *Session) {
		hreq := s.HTTPRequest("POST", "/api/test/"+longStr, nil)

		s.GetRequest(t).
			AssertSubject(t, "access.test").
			RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))

		// Validate http response
		hreq.GetResponse(t).
			AssertError(t, reserr.ErrSubjectTooLong).
			AssertStatusCode(t, http.StatusRequestURITooLong)
	})
}

func TestHTTPCall_LongModelQuery_ReturnsResult(t *testing.T) {
	query := "q=" + generateString(10000)
	successResponse := json.RawMessage(`{"foo":"bar"}`)

	runTest(t, func(s *Session) {
		hreq := s.HTTPRequest("POST", "/api/test/model/method?"+query, nil)

		// Get access request
		s.GetRequest(t).
			AssertSubject(t, "access.test.model").
			AssertPathPayload(t, "query", query).
			RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))
		// Get call request
		s.GetRequest(t).
			AssertSubject(t, "call.test.model.method").
			AssertPathPayload(t, "query", query).
			RespondSuccess(successResponse)

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, successResponse)
	})
}
