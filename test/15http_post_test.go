package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/jirenius/resgate/server/mq"
	"github.com/jirenius/resgate/server/reserr"
)

// Test response to a HTTP POST request to a primitive query model method
func TestHTTPPostOnPrimitiveQueryModel(t *testing.T) {
	runTest(t, func(s *Session) {
		successResponse := json.RawMessage(`{"foo":"bar"}`)

		hreq := s.HTTPRequest("POST", "/api/test/model/method?q=foo&f=bar", nil)

		// Handle query model access request
		s.
			GetRequest(t).
			AssertSubject(t, "access.test.model").
			AssertPathPayload(t, "token", nil).
			AssertPathPayload(t, "query", "q=foo&f=bar").
			RespondSuccess(json.RawMessage(`{"call":"method"}`))
		// Handle query model call request
		s.
			GetRequest(t).
			AssertSubject(t, "call.test.model.method").
			AssertPathPayload(t, "query", "q=foo&f=bar").
			RespondSuccess(successResponse)

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, successResponse)
	})
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

	tbl := []struct {
		Params         []byte      // Params to use as body in post request
		AccessResponse interface{} // Response on access request. nil means timeout
		CallResponse   interface{} // Response on call request. requestTimeout means timeout. noRequest means no call request is expected
		ExpectedCode   int         // Expected response status code
		Expected       interface{} // Expected response body
	}{
		// Params variants
		{nil, fullCallAccess, successResponse, http.StatusOK, successResponse},
		{params, fullCallAccess, successResponse, http.StatusOK, successResponse},
		// AccessResponse variants
		{nil, methodCallAccess, successResponse, http.StatusOK, successResponse},
		{nil, multiMethodCallAccess, successResponse, http.StatusOK, successResponse},
		{nil, missingMethodCallAccess, noRequest, http.StatusUnauthorized, reserr.ErrAccessDenied},
		{nil, noCallAccess, noRequest, http.StatusUnauthorized, reserr.ErrAccessDenied},
		{nil, nil, noRequest, http.StatusNotFound, mq.ErrRequestTimeout},
		// CallResponse variants
		{nil, fullCallAccess, reserr.ErrInvalidParams, http.StatusBadRequest, reserr.ErrInvalidParams},
		{nil, fullCallAccess, nil, http.StatusNoContent, []byte{}},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			// Send HTTP post request
			hreq := s.HTTPRequest("POST", "/api/test/model/method", l.Params)

			req := s.GetRequest(t)
			req.AssertSubject(t, "access.test.model")
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
		})
	}
}

// Test HTTP post responses to new requests
func TestHTTPPostNewResponses(t *testing.T) {
	params := json.RawMessage(`{"value":42}`)
	callResponse := json.RawMessage(`{"rid":"test.model"}`)
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
	}{
		// Params variants
		{params, fullCallAccess, callResponse, http.StatusCreated, nil, modelLocationHref},
		{nil, fullCallAccess, callResponse, http.StatusCreated, nil, modelLocationHref},
		// CallAccessResponse variants
		{params, methodCallAccess, callResponse, http.StatusCreated, nil, modelLocationHref},
		{params, multiMethodCallAccess, callResponse, http.StatusCreated, nil, modelLocationHref},
		{params, missingMethodCallAccess, noRequest, http.StatusUnauthorized, reserr.ErrAccessDenied, nil},
		{params, noCallAccess, noRequest, http.StatusUnauthorized, reserr.ErrAccessDenied, nil},
		{params, requestTimeout, noRequest, http.StatusNotFound, mq.ErrRequestTimeout, nil},
		// CallResponse variants
		{params, fullCallAccess, reserr.ErrInvalidParams, http.StatusBadRequest, reserr.ErrInvalidParams, nil},
		{params, fullCallAccess, requestTimeout, http.StatusNotFound, mq.ErrRequestTimeout, nil},
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
