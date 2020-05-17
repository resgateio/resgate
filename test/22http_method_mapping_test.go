package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/raphaelpereira/resgate/server"
	"github.com/raphaelpereira/resgate/server/reserr"
)

func TestHTTPMethod_MappedMethod_ExpectedResponse(t *testing.T) {
	params := json.RawMessage(`{"foo":"bar"}`)
	result := json.RawMessage(`"zoo"`)
	method := "mappedMethod"

	tbl := []struct {
		Config       func(cfg *server.Config) // Server config
		Method       string                   // HTTP method to use
		CallResponse interface{}              // Response on call request. requestTimeout means timeout. noRequest means no call request is expected
		ExpectedCode int                      // Expected response status code
		Expected     interface{}              // Expected response body
	}{
		// With mapped methods and success
		{func(cfg *server.Config) { cfg.PUTMethod = &method }, "PUT", result, http.StatusOK, result},
		{func(cfg *server.Config) { cfg.DELETEMethod = &method }, "DELETE", result, http.StatusOK, result},
		{func(cfg *server.Config) { cfg.PATCHMethod = &method }, "PATCH", result, http.StatusOK, result},
		// With mapped methods and error
		{func(cfg *server.Config) { cfg.PUTMethod = &method }, "PUT", reserr.ErrInvalidParams, http.StatusBadRequest, reserr.ErrInvalidParams},
		{func(cfg *server.Config) { cfg.DELETEMethod = &method }, "DELETE", reserr.ErrInvalidParams, http.StatusBadRequest, reserr.ErrInvalidParams},
		{func(cfg *server.Config) { cfg.PATCHMethod = &method }, "PATCH", reserr.ErrInvalidParams, http.StatusBadRequest, reserr.ErrInvalidParams},
		{func(cfg *server.Config) { cfg.PUTMethod = &method }, "PUT", requestTimeout, http.StatusNotFound, reserr.ErrTimeout},
		{func(cfg *server.Config) { cfg.DELETEMethod = &method }, "DELETE", requestTimeout, http.StatusNotFound, reserr.ErrTimeout},
		{func(cfg *server.Config) { cfg.PATCHMethod = &method }, "PATCH", requestTimeout, http.StatusNotFound, reserr.ErrTimeout},
		{func(cfg *server.Config) { cfg.PUTMethod = &method }, "PUT", reserr.ErrAccessDenied, http.StatusUnauthorized, reserr.ErrAccessDenied},
		{func(cfg *server.Config) { cfg.DELETEMethod = &method }, "DELETE", reserr.ErrAccessDenied, http.StatusUnauthorized, reserr.ErrAccessDenied},
		{func(cfg *server.Config) { cfg.PATCHMethod = &method }, "PATCH", reserr.ErrAccessDenied, http.StatusUnauthorized, reserr.ErrAccessDenied},
		{func(cfg *server.Config) { cfg.PUTMethod = &method }, "PUT", reserr.ErrMethodNotFound, http.StatusMethodNotAllowed, reserr.ErrMethodNotAllowed},
		{func(cfg *server.Config) { cfg.DELETEMethod = &method }, "DELETE", reserr.ErrMethodNotFound, http.StatusMethodNotAllowed, reserr.ErrMethodNotAllowed},
		{func(cfg *server.Config) { cfg.PATCHMethod = &method }, "PATCH", reserr.ErrMethodNotFound, http.StatusMethodNotAllowed, reserr.ErrMethodNotAllowed},
		// Without mapping
		{func(cfg *server.Config) { cfg.DELETEMethod = &method }, "PUT", noRequest, http.StatusMethodNotAllowed, reserr.ErrMethodNotAllowed},
		{func(cfg *server.Config) { cfg.PATCHMethod = &method }, "DELETE", noRequest, http.StatusMethodNotAllowed, reserr.ErrMethodNotAllowed},
		{func(cfg *server.Config) { cfg.PUTMethod = &method }, "PATCH", noRequest, http.StatusMethodNotAllowed, reserr.ErrMethodNotAllowed},
	}

	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			hreq := s.HTTPRequest(l.Method, "/api/test/model", params)

			if l.CallResponse != noRequest {
				// Handle access request
				s.GetRequest(t).
					AssertSubject(t, "access.test.model").
					RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))

				// Handle call request
				req := s.GetRequest(t).
					AssertSubject(t, "call.test.model."+method).
					AssertPathPayload(t, "params", json.RawMessage(params))
				if l.CallResponse == requestTimeout {
					req.Timeout()
				} else if err, ok := l.CallResponse.(*reserr.Error); ok {
					req.RespondError(err)
				} else {
					req.RespondSuccess(l.CallResponse)
				}
			}

			// Validate HTTP response
			hresp := hreq.GetResponse(t)
			hresp.AssertStatusCode(t, l.ExpectedCode)
			if err, ok := l.Expected.(*reserr.Error); ok {
				hresp.AssertError(t, err)
			} else if code, ok := l.Expected.(string); ok {
				hresp.AssertErrorCode(t, code)
			} else {
				hresp.AssertBody(t, l.Expected)
			}
		}, l.Config)
	}
}
