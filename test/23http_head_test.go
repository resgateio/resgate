package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/raphaelpereira/resgate/server/reserr"
)

// Test invalid urls for HTTP get requests
func TestHTTPMethodHEAD_InvalidURLs_CorrectStatus(t *testing.T) {
	tbl := []struct {
		URL          string // Url path
		ExpectedCode int
	}{
		{"/wrong_prefix/test/model", http.StatusNotFound},
		{"/api/", http.StatusNotFound},
		{"/api/test.model", http.StatusNotFound},
		{"/api/test/model/", http.StatusNotFound},
		{"/api/test//model", http.StatusNotFound},
		{"/api/test/mådel/action", http.StatusNotFound},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			s.HTTPRequest("HEAD", l.URL, nil).
				GetResponse(t).
				AssertStatusCode(t, l.ExpectedCode)
			// We don't check the Body as the httptest.ResponseRecorder
			// does not discard the written bytes to the body, unlike the
			// actual http package.
		})
	}
}

func TestHTTPHead_OnSuccess_NoBody(t *testing.T) {
	model := resourceData("test.model")
	runTest(t, func(s *Session) {
		hreq := s.HTTPRequest("HEAD", "/api/test/model", nil)

		/// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.model")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.model")
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate http response
		hreq.GetResponse(t).
			AssertStatusCode(t, http.StatusOK)
		// We don't check the Body as the httptest.ResponseRecorder
		// does not discard the written bytes to the body, unlike the
		// actual http package.
	})
}

func TestHTTPHead_OnError_NoBody(t *testing.T) {
	runTest(t, func(s *Session) {
		hreq := s.HTTPRequest("HEAD", "/api/test/model", nil)

		/// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.model")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.model")
		req.RespondError(reserr.ErrNotFound)

		// Validate http response
		hreq.GetResponse(t).
			AssertStatusCode(t, http.StatusNotFound)
		// We don't check the Body as the httptest.ResponseRecorder
		// does not discard the written bytes to the body, unlike the
		// actual http package.
	})
}
