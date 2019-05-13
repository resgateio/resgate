package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/resgateio/resgate/server/mq"
	"github.com/resgateio/resgate/server/reserr"
)

// Test responses to client call requests
func TestCallOnResource(t *testing.T) {

	model := resourceData("test.model")
	params := json.RawMessage(`{"value":42}`)
	successResponse := json.RawMessage(`{"foo":"bar"}`)
	// Access responses
	fullCallAccess := json.RawMessage(`{"get":true,"call":"*"}`)
	methodCallAccess := json.RawMessage(`{"get":true,"call":"method"}`)
	multiMethodCallAccess := json.RawMessage(`{"get":true,"call":"foo,method"}`)
	missingMethodCallAccess := json.RawMessage(`{"get":true,"call":"foo,bar"}`)
	noCallAccess := json.RawMessage(`{"get":true}`)

	tbl := []struct {
		Subscribe      bool        // Flag if model is subscribed prior to call
		Params         interface{} // Params to use in call request
		AccessResponse interface{} // Response on access request. nil means timeout
		CallResponse   interface{} // Response on call request. requestTimeout means timeout. noRequest means no call request is expected
		Expected       interface{}
	}{
		// Params variants
		{true, nil, fullCallAccess, successResponse, successResponse},
		{false, nil, fullCallAccess, successResponse, successResponse},
		{true, params, fullCallAccess, successResponse, successResponse},
		{false, params, fullCallAccess, successResponse, successResponse},
		// AccessResponse variants
		{true, nil, methodCallAccess, successResponse, successResponse},
		{false, nil, methodCallAccess, successResponse, successResponse},
		{true, nil, multiMethodCallAccess, successResponse, successResponse},
		{false, nil, multiMethodCallAccess, successResponse, successResponse},
		{false, nil, missingMethodCallAccess, noRequest, reserr.ErrAccessDenied},
		{false, nil, noCallAccess, noRequest, reserr.ErrAccessDenied},
		{false, nil, nil, noRequest, mq.ErrRequestTimeout},
		// CallResponse variants
		{true, nil, fullCallAccess, reserr.ErrInvalidParams, reserr.ErrInvalidParams},
		{false, nil, fullCallAccess, reserr.ErrInvalidParams, reserr.ErrInvalidParams},
		{true, nil, fullCallAccess, nil, json.RawMessage(`null`)},
		{false, nil, fullCallAccess, nil, json.RawMessage(`null`)},
		{true, nil, fullCallAccess, 42, json.RawMessage(`42`)},
		{false, nil, fullCallAccess, 42, json.RawMessage(`42`)},
		// Invalid service responses
		{false, nil, json.RawMessage(`{"get":"invalid"}`), noRequest, reserr.CodeInternalError},
		{false, nil, []byte(`{"broken":JSON}`), noRequest, reserr.CodeInternalError},
		{false, nil, methodCallAccess, []byte(`{"broken":JSON}`), reserr.CodeInternalError},
		{false, nil, methodCallAccess, []byte(`{}`), reserr.CodeInternalError},
		{false, nil, json.RawMessage("\r\n\t {\"get\":\r\n\t true,\"call\":\"*\"}\r\n\t "), successResponse, successResponse},
		{true, nil, methodCallAccess, []byte(`{"result":{"foo":"bar"},"error":{"code":"system.custom","message":"Custom"}}`), "system.custom"},
		{false, nil, methodCallAccess, []byte(`{"result":{"foo":"bar"},"error":{"code":"system.custom","message":"Custom"}}`), "system.custom"},
		// Invalid call error response
		{true, nil, fullCallAccess, []byte(`{"error":[]}`), reserr.CodeInternalError},
		{false, nil, fullCallAccess, []byte(`{"error":[]}`), reserr.CodeInternalError},
		{true, nil, fullCallAccess, []byte(`{"error":{"message":"missing code"}}`), ""},
		{false, nil, fullCallAccess, []byte(`{"error":{"message":"missing code"}}`), ""},
		{true, nil, fullCallAccess, []byte(`{"error":{"code":12,"message":"integer code"}}`), reserr.CodeInternalError},
		{false, nil, fullCallAccess, []byte(`{"error":{"code":12,"message":"integer code"}}`), reserr.CodeInternalError},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			c := s.Connect()
			var creq *ClientRequest

			if l.Subscribe {
				creq = c.Request("subscribe.test.model", nil)

				// Handle model get and access request
				mreqs := s.GetParallelRequests(t, 2)
				req := mreqs.GetRequest(t, "access.test.model")
				if l.AccessResponse == nil {
					req.Timeout()
				} else if err, ok := l.AccessResponse.(*reserr.Error); ok {
					req.RespondError(err)
				} else {
					req.RespondSuccess(l.AccessResponse)
				}
				req = mreqs.GetRequest(t, "get.test.model")
				req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

				// Get client response
				creq.GetResponse(t)

				// Send client call request
				creq = c.Request("call.test.model.method", l.Params)
				if l.CallResponse != noRequest {
					// Get call request
					req = s.GetRequest(t)
					req.AssertSubject(t, "call.test.model.method")
					req.AssertPathType(t, "cid", string(""))
					req.AssertPathPayload(t, "token", nil)
					req.AssertPathPayload(t, "params", l.Params)
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
			} else {
				// Send client call request
				creq = c.Request("call.test.model.method", l.Params)

				req := s.GetRequest(t)
				req.AssertSubject(t, "access.test.model")
				if l.AccessResponse == nil {
					req.Timeout()
				} else if err, ok := l.AccessResponse.(*reserr.Error); ok {
					req.RespondError(err)
				} else if raw, ok := l.AccessResponse.([]byte); ok {
					req.RespondRaw(raw)
				} else {
					req.RespondSuccess(l.AccessResponse)
				}

				if l.CallResponse != noRequest {
					// Get call request
					req = s.GetRequest(t)
					req.AssertSubject(t, "call.test.model.method")
					req.AssertPathPayload(t, "params", l.Params)
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
		})
	}
}

// Test responses to client call requests after an initial access error
func TestCallOnResourceAfterAccessError(t *testing.T) {
	params := json.RawMessage(`{"value":42}`)
	successResponse := json.RawMessage(`{"foo":"bar"}`)
	// Access responses
	fullCallAccess := json.RawMessage(`{"get":true,"call":"*"}`)
	noCallAccess := json.RawMessage(`{"get":true}`)

	tbl := []struct {
		FirstAccessResponse  interface{} // Response on first access request. nil means timeout
		FirstExpectCall      bool        // Flag if a first call request is expected
		FirstExpected        interface{}
		SecondAccessResponse interface{} // Response on second access request. noRequest means no second access request is expected. nil means timeout.
		SecondExpectCall     bool        // Flag if a first call request is expected
		SecondExpected       interface{}
	}{
		{fullCallAccess, true, successResponse, noRequest, true, successResponse},
		{noCallAccess, false, reserr.ErrAccessDenied, noRequest, false, reserr.ErrAccessDenied},
		{reserr.ErrAccessDenied, false, reserr.ErrAccessDenied, noRequest, false, reserr.ErrAccessDenied},
		{nil, false, reserr.ErrTimeout, fullCallAccess, true, successResponse},
		{nil, false, reserr.ErrTimeout, nil, false, reserr.ErrTimeout},
		{nil, false, reserr.ErrTimeout, reserr.ErrAccessDenied, false, reserr.ErrAccessDenied},
		{reserr.ErrInternalError, false, reserr.ErrInternalError, fullCallAccess, true, successResponse},
		{reserr.ErrInternalError, false, reserr.ErrInternalError, nil, false, reserr.ErrTimeout},
		{reserr.ErrInternalError, false, reserr.ErrInternalError, reserr.ErrAccessDenied, false, reserr.ErrAccessDenied},
	}

	for i, l := range tbl {
		for subscribe := true; subscribe; subscribe = false {
			runNamedTest(t, fmt.Sprintf("#%d where subscribe is %+v", i+1, subscribe), func(s *Session) {
				c := s.Connect()
				var creq *ClientRequest

				if subscribe {
					// Subscribe to parent model
					subscribeToTestModelParent(t, s, c, false)
				}

				accessResponse := l.FirstAccessResponse
				expectCall := l.FirstExpectCall
				expected := l.FirstExpected
				for round := 0; round < 2; round++ {
					// Send client call request
					creq = c.Request("call.test.model.method", params)

					// Handle access request if one is expected
					if accessResponse != noRequest {
						req := s.GetRequest(t)
						req.AssertSubject(t, "access.test.model")
						if accessResponse == nil {
							req.Timeout()
						} else if err, ok := accessResponse.(*reserr.Error); ok {
							req.RespondError(err)
						} else if raw, ok := accessResponse.([]byte); ok {
							req.RespondRaw(raw)
						} else {
							req.RespondSuccess(accessResponse)
						}
					}

					if expectCall {
						// Get call request
						req := s.GetRequest(t)
						req.AssertSubject(t, "call.test.model.method")
						req.RespondSuccess(successResponse)
					}

					// Validate client response
					cresp := creq.GetResponse(t)
					if err, ok := expected.(*reserr.Error); ok {
						cresp.AssertError(t, err)
					} else if code, ok := expected.(string); ok {
						cresp.AssertErrorCode(t, code)
					} else {
						cresp.AssertResult(t, expected)
					}

					accessResponse = l.SecondAccessResponse
					expectCall = l.SecondExpectCall
					expected = l.SecondExpected
				}
			})
		}
	}
}
