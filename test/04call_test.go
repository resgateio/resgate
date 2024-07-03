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
	successResult := json.RawMessage(`{"payload":{"foo":"bar"}}`)
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
		{true, nil, fullCallAccess, successResponse, successResult},
		{false, nil, fullCallAccess, successResponse, successResult},
		{true, params, fullCallAccess, successResponse, successResult},
		{false, params, fullCallAccess, successResponse, successResult},
		// AccessResponse variants
		{true, nil, methodCallAccess, successResponse, successResult},
		{false, nil, methodCallAccess, successResponse, successResult},
		{true, nil, multiMethodCallAccess, successResponse, successResult},
		{false, nil, multiMethodCallAccess, successResponse, successResult},
		{false, nil, missingMethodCallAccess, noRequest, reserr.ErrAccessDenied},
		{false, nil, noCallAccess, noRequest, reserr.ErrAccessDenied},
		{false, nil, nil, noRequest, mq.ErrRequestTimeout},
		// CallResponse variants
		{true, nil, fullCallAccess, reserr.ErrInvalidParams, reserr.ErrInvalidParams},
		{false, nil, fullCallAccess, reserr.ErrInvalidParams, reserr.ErrInvalidParams},
		{true, nil, fullCallAccess, nil, json.RawMessage(`{"payload":null}`)},
		{false, nil, fullCallAccess, nil, json.RawMessage(`{"payload":null}`)},
		{true, nil, fullCallAccess, 42, json.RawMessage(`{"payload":42}`)},
		{false, nil, fullCallAccess, 42, json.RawMessage(`{"payload":42}`)},
		// Invalid service responses
		{false, nil, json.RawMessage(`{"get":"invalid"}`), noRequest, reserr.CodeInternalError},
		{false, nil, []byte(`{"broken":JSON}`), noRequest, reserr.CodeInternalError},
		{false, nil, methodCallAccess, []byte(`{"broken":JSON}`), reserr.CodeInternalError},
		{false, nil, methodCallAccess, []byte(`{}`), reserr.CodeInternalError},
		{false, nil, json.RawMessage("\r\n\t {\"get\":\r\n\t true,\"call\":\"*\"}\r\n\t "), successResponse, successResult},
		{true, nil, methodCallAccess, []byte(`{"result":{"foo":"bar"},"error":{"code":"system.custom","message":"Custom"}}`), "system.custom"},
		{false, nil, methodCallAccess, []byte(`{"result":{"foo":"bar"},"error":{"code":"system.custom","message":"Custom"}}`), "system.custom"},
		// Invalid call error response
		{true, nil, fullCallAccess, []byte(`{"error":[]}`), reserr.CodeInternalError},
		{false, nil, fullCallAccess, []byte(`{"error":[]}`), reserr.CodeInternalError},
		{true, nil, fullCallAccess, []byte(`{"error":{"message":"missing code"}}`), ""},
		{false, nil, fullCallAccess, []byte(`{"error":{"message":"missing code"}}`), ""},
		{true, nil, fullCallAccess, []byte(`{"error":{"code":12,"message":"integer code"}}`), reserr.CodeInternalError},
		{false, nil, fullCallAccess, []byte(`{"error":{"code":12,"message":"integer code"}}`), reserr.CodeInternalError},
		// Invalid call resource response
		{true, nil, fullCallAccess, []byte(`{"resource":"test.model"}`), reserr.CodeInternalError},
		{false, nil, fullCallAccess, []byte(`{"resource":"test.model"}`), reserr.CodeInternalError},
		{true, nil, fullCallAccess, []byte(`{"resource":{}}`), reserr.CodeInternalError},
		{false, nil, fullCallAccess, []byte(`{"resource":{}}`), reserr.CodeInternalError},
		{true, nil, fullCallAccess, []byte(`{"resource":{"rid":42}}`), reserr.CodeInternalError},
		{false, nil, fullCallAccess, []byte(`{"resource":{"rid":42}}`), reserr.CodeInternalError},
		{true, nil, fullCallAccess, []byte(`{"resource":{"rid":"test..model"}}`), reserr.CodeInternalError},
		{false, nil, fullCallAccess, []byte(`{"resource":{"rid":"test..model"}}`), reserr.CodeInternalError},
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
				req.AssertPathMissing(t, "isHttp")
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
					req.AssertPathMissing(t, "isHttp")
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
					req.AssertPathMissing(t, "isHttp")
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

// Test resource responses to client call requests
func TestCall_WithPrimitiveModelResourceResponse_ReturnsExpected(t *testing.T) {
	params := json.RawMessage(`{"value":42}`)
	model := resourceData("test.model")
	modelGetResponse := json.RawMessage(`{"model":` + model + `}`)
	modelClientResponse := json.RawMessage(`{"rid":"test.model","models":{"test.model":` + model + `}}`)
	modelClientInvalidParamsResponse := json.RawMessage(`{"rid":"test.model","errors":{"test.model":{"code":"system.invalidParams","message":"Invalid parameters"}}}`)
	modelClientRequestTimeoutResponse := json.RawMessage(`{"rid":"test.model","errors":{"test.model":{"code":"system.timeout","message":"Request timeout"}}}`)
	modelClientRequestAccessDeniedResponse := json.RawMessage(`{"rid":"test.model","errors":{"test.model":{"code":"system.accessDenied","message":"Access denied"}}}`)
	fullAccess := json.RawMessage(`{"get":true}`)

	tbl := []struct {
		GetResponse    interface{} // Response on get request of the newly created model.test. noRequest means no request is expected
		AccessResponse interface{} // Response on access request of the newly created model.test.
		Expected       interface{} // Expected response to client
	}{
		// Params variants
		{modelGetResponse, fullAccess, modelClientResponse},
		// GetResponse variants
		{reserr.ErrInvalidParams, fullAccess, modelClientInvalidParamsResponse},
		{requestTimeout, fullAccess, modelClientRequestTimeoutResponse},
		// AccessResponse variants
		{modelGetResponse, json.RawMessage(`{"get":false}`), modelClientRequestAccessDeniedResponse},
		{modelGetResponse, reserr.ErrInvalidParams, modelClientInvalidParamsResponse},
		{modelGetResponse, reserr.ErrAccessDenied, modelClientRequestAccessDeniedResponse},
		{modelGetResponse, requestTimeout, modelClientRequestTimeoutResponse},
	}

	for i, l := range tbl {
		// Run both "get" and "subscribe" tests
		for _, method := range []string{"get", "subscribe"} {
			// Run both orders for "get" and "access" response

			s := fmt.Sprintf("#%d when using %#v", i+1, method)
			runNamedTest(t, s, func(s *Session) {
				c := s.Connect()

				// Send client call request
				creq := c.Request("call.test.collection.method", params)
				s.GetRequest(t).
					AssertSubject(t, "access.test.collection").
					RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))
				// Get call request
				s.GetRequest(t).
					AssertPathPayload(t, "params", params).
					AssertSubject(t, "call.test.collection.method").
					RespondResource("test.model")

				mreqs := s.GetParallelRequests(t, 2)
				// Send access response
				req := mreqs.GetRequest(t, "access.test.model")
				if l.AccessResponse == requestTimeout {
					req.Timeout()
				} else if err, ok := l.AccessResponse.(*reserr.Error); ok {
					req.RespondError(err)
				} else if raw, ok := l.AccessResponse.([]byte); ok {
					req.RespondRaw(raw)
				} else {
					req.RespondSuccess(l.AccessResponse)
				}
				// Send get response
				req = mreqs.GetRequest(t, "get.test.model")
				if l.GetResponse == requestTimeout {
					req.Timeout()
				} else if err, ok := l.GetResponse.(*reserr.Error); ok {
					req.RespondError(err)
				} else if raw, ok := l.GetResponse.([]byte); ok {
					req.RespondRaw(raw)
				} else {
					req.RespondSuccess(l.GetResponse)
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
}

// Test legacy responses to client call requests
func TestLegacyCallOnResource(t *testing.T) {
	params := json.RawMessage(`{"value":42}`)

	tbl := []struct {
		CallResponse interface{} // Response on call request.
		Expected     interface{}
	}{
		// Result response
		{json.RawMessage(`{"foo":"bar"}`), json.RawMessage(`{"foo":"bar"}`)},
		{json.RawMessage(`{"rid":"test.model"}`), json.RawMessage(`{"rid":"test.model"}`)},
		{nil, nil},
		// Resource response
		{[]byte(`{"resource":{"rid":"test.model"}}`), json.RawMessage(`{"rid":"test.model"}`)},
		// Invalid call resource response
		{[]byte(`{"resource":"test.model"}`), reserr.CodeInternalError},
		{[]byte(`{"resource":"test.model"}`), reserr.CodeInternalError},
		{[]byte(`{"resource":{}}`), reserr.CodeInternalError},
		{[]byte(`{"resource":{}}`), reserr.CodeInternalError},
		{[]byte(`{"resource":{"rid":42}}`), reserr.CodeInternalError},
		{[]byte(`{"resource":{"rid":42}}`), reserr.CodeInternalError},
		{[]byte(`{"resource":{"rid":"test..model"}}`), reserr.CodeInternalError},
		{[]byte(`{"resource":{"rid":"test..model"}}`), reserr.CodeInternalError},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			c := s.ConnectWithoutVersion()

			// Send client call request
			creq := c.Request("call.test.model.method", params)
			s.GetRequest(t).
				AssertSubject(t, "access.test.model").
				RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))

			// Get call request
			req := s.GetRequest(t)
			req.AssertPathPayload(t, "params", params)
			req.AssertSubject(t, "call.test.model.method")
			if l.CallResponse == requestTimeout {
				req.Timeout()
			} else if err, ok := l.CallResponse.(*reserr.Error); ok {
				req.RespondError(err)
			} else if raw, ok := l.CallResponse.([]byte); ok {
				req.RespondRaw(raw)
			} else {
				req.RespondSuccess(l.CallResponse)
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
	successResult := json.RawMessage(`{"payload":{"foo":"bar"}}`)
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
		{fullCallAccess, true, successResult, noRequest, true, successResult},
		{noCallAccess, false, reserr.ErrAccessDenied, noRequest, false, reserr.ErrAccessDenied},
		{reserr.ErrAccessDenied, false, reserr.ErrAccessDenied, noRequest, false, reserr.ErrAccessDenied},
		{nil, false, reserr.ErrTimeout, fullCallAccess, true, successResult},
		{nil, false, reserr.ErrTimeout, nil, false, reserr.ErrTimeout},
		{nil, false, reserr.ErrTimeout, reserr.ErrAccessDenied, false, reserr.ErrAccessDenied},
		{reserr.ErrInternalError, false, reserr.ErrInternalError, fullCallAccess, true, successResult},
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

func TestCall_WithResourceResponse(t *testing.T) {

	customError := &reserr.Error{Code: "custom.error", Message: "Custom error"}
	model := resourceData("test.model")
	params := json.RawMessage(`{"value":42}`)
	modelGetResponse := json.RawMessage(`{"model":` + model + `}`)
	modelClientResponse := json.RawMessage(`{"rid":"test.model","models":{"test.model":` + model + `}}`)
	modelClientCustomErrorResponse := json.RawMessage(`{"rid":"test.model","errors":{"test.model":{"code":"custom.error","message":"Custom error"}}}`)
	modelClientRequestTimeoutResponse := json.RawMessage(`{"rid":"test.model","errors":{"test.model":{"code":"system.timeout","message":"Request timeout"}}}`)
	modelClientRequestAccessDeniedResponse := json.RawMessage(`{"rid":"test.model","errors":{"test.model":{"code":"system.accessDenied","message":"Access denied"}}}`)
	// Access responses
	fullGetAccess := json.RawMessage(`{"get":true,"call":"*"}`)

	tbl := []struct {
		GetResponse         interface{} // Response on get request of the newly created model.test. noRequest means no request is expected
		ModelAccessResponse interface{} // Response on access request of the newly created model.test.
		Expected            interface{} // Expected response to client
	}{
		// GetResponse variants
		{modelGetResponse, fullGetAccess, modelClientResponse},
		{customError, fullGetAccess, modelClientCustomErrorResponse},
		{requestTimeout, fullGetAccess, modelClientRequestTimeoutResponse},
		// ModelAccessResponse variants
		{modelGetResponse, json.RawMessage(`{"get":false}`), modelClientRequestAccessDeniedResponse},
		{modelGetResponse, customError, modelClientCustomErrorResponse},
		{modelGetResponse, reserr.ErrAccessDenied, modelClientRequestAccessDeniedResponse},
		{modelGetResponse, requestTimeout, modelClientRequestTimeoutResponse},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			c := s.Connect()

			// Send client request
			creq := c.Request("call.test.collection.create", params)

			req := s.GetRequest(t)
			req.AssertSubject(t, "access.test.collection")
			req.RespondSuccess(fullGetAccess)

			// Get call request
			req = s.GetRequest(t)
			req.AssertSubject(t, "call.test.collection.create")
			req.AssertPathPayload(t, "params", params)
			req.RespondResource("test.model")

			// Get model get/access requests
			mreqs := s.GetParallelRequests(t, 2)

			// Send get response
			req = mreqs.GetRequest(t, "get.test.model")
			if l.GetResponse == requestTimeout {
				req.Timeout()
			} else if err, ok := l.GetResponse.(*reserr.Error); ok {
				req.RespondError(err)
			} else {
				req.RespondSuccess(l.GetResponse)
			}

			// Send access response
			req = mreqs.GetRequest(t, "access.test.model")
			if l.ModelAccessResponse == requestTimeout {
				req.Timeout()
			} else if err, ok := l.ModelAccessResponse.(*reserr.Error); ok {
				req.RespondError(err)
			} else {
				req.RespondSuccess(l.ModelAccessResponse)
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

func TestCall_WithCIDPlaceholder_ReplacesCID(t *testing.T) {
	runTest(t, func(s *Session) {
		params := json.RawMessage(`{"foo":"bar"}`)
		result := json.RawMessage(`"zoo"`)

		c := s.Connect()
		cid := getCID(t, s, c)

		creq := c.Request("call.test.{cid}.model.method", params)

		// Handle access request
		s.GetRequest(t).
			AssertSubject(t, "access.test."+cid+".model").
			RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))

		// Handle call request
		s.GetRequest(t).
			AssertSubject(t, "call.test."+cid+".model.method").
			AssertPathPayload(t, "params", params).
			RespondSuccess(result)

		// Validate response
		creq.GetResponse(t).
			AssertResult(t, json.RawMessage(`{"payload":"zoo"}`))
	})
}

func TestCall_LongResourceMethod_ReturnsErrSubjectTooLong(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		creq := c.Request("call.test."+generateString(10000), nil)

		s.GetRequest(t).
			AssertSubject(t, "access.test").
			RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))

		creq.GetResponse(t).
			AssertError(t, reserr.ErrSubjectTooLong)
	})
}

func TestCall_LongResourceID_ReturnsErrSubjectTooLong(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		creq := c.Request("call.test."+generateString(10000)+".method", nil)

		creq.GetResponse(t).
			AssertError(t, reserr.ErrSubjectTooLong)
	})
}
