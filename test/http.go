package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/jirenius/resgate/reserr"
)

type HTTPRequest struct {
	ch  chan *HTTPResponse
	req *http.Request
	rr  *httptest.ResponseRecorder
}

type HTTPResponse struct {
	*httptest.ResponseRecorder
}

// GetResponse awaits for a response and returns it.
// Fails if a response hasn't arrived within 1 second.
func (hr *HTTPRequest) GetResponse(t *testing.T) *HTTPResponse {
	select {
	case resp := <-hr.ch:
		return resp
	case <-time.After(timeoutSeconds * time.Second):
		if t == nil {
			pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
			panic(fmt.Sprintf("expected a response to http request %#v, but found none", hr.req.URL.Path))
		} else {
			t.Fatalf("expected a response to http request %#v, but found none", hr.req.URL.Path)
		}
	}
	return nil
}

// Equals asserts that the response has the expected status code and body
func (hr *HTTPResponse) Equals(t *testing.T, code int, body interface{}) *HTTPResponse {
	hr.AssertStatusCode(t, code)
	hr.AssertBody(t, body)
	return hr
}

// AssertStatusCode asserts that the response has the expected status code
func (hr *HTTPResponse) AssertStatusCode(t *testing.T, code int) *HTTPResponse {
	if hr.Code != code {
		t.Fatalf("expected response code to be %d, but got %d", code, hr.Code)
	}
	return hr
}

// AssertBody asserts that the response has the expected body
func (hr *HTTPResponse) AssertBody(t *testing.T, body interface{}) *HTTPResponse {
	var err error
	var bj []byte
	var ab interface{}
	if body != nil {
		bj, err = json.Marshal(body)
		if err != nil {
			panic("test: error marshalling assertion body: " + err.Error())
		}

		err = json.Unmarshal(bj, &ab)
		if err != nil {
			panic("test: error unmarshalling assertion body: " + err.Error())
		}
	}

	bb := hr.Body.Bytes()
	// Quick exit if both are empty
	if len(bb) == 0 && body == nil {
		return hr
	}

	var b interface{}
	err = json.Unmarshal(bb, &b)
	if err != nil {
		t.Fatalf("expected response body to be: \n%s\nbut got:\n%s", bj, hr.Body.String())
	}

	if !reflect.DeepEqual(ab, b) {
		t.Fatalf("expected response body to be: \n%s\nbut got:\n%s", bj, hr.Body.String())
	}
	return hr
}

// AssertError asserts that the response does not have status 200, and has
// the expected error
func (hr *HTTPResponse) AssertError(t *testing.T, err *reserr.Error) *HTTPResponse {
	if hr.Code == http.StatusOK {
		t.Fatalf("expected response code not to be 200, but it was")
	}
	hr.AssertBody(t, err)
	return hr
}

// AssertErrorCode asserts that the response does not have status 200, and
// has the expected error code
func (hr *HTTPResponse) AssertErrorCode(t *testing.T, code string) *HTTPResponse {
	if hr.Code == http.StatusOK {
		t.Fatalf("expected response code not to be 200, but it was")
	}

	var rerr reserr.Error
	err := json.Unmarshal(hr.Body.Bytes(), &rerr)
	if err != nil {
		t.Fatalf("expected error response, but got body:\n%s", hr.Body.String())
	}

	if rerr.Code != code {
		t.Fatalf("expected response error code to be:\n%#v\nbut got:\n%#v", code, rerr.Code)
	}
	return hr
}

// AssertHeaders asserts that the response includes the expected headers
func (hr *HTTPResponse) AssertHeaders(t *testing.T, h map[string]string) *HTTPResponse {
	for k, v := range h {
		hv := hr.Result().Header.Get(k)
		if hr.Result().Header.Get(k) != v {
			if hv == "" {
				t.Fatalf("expected response header %s to be %s, but header not found", k, v)
			} else {
				t.Fatalf("expected response header %s to be %s, but got %s", k, v, hv)
			}
		}
	}
	return hr
}
