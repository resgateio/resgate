package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/resgateio/resgate/server/mq"
	"github.com/resgateio/resgate/server/reserr"
)

// Test responses to client new requests
func TestLegacyNewOnResource(t *testing.T) {

	model := resourceData("test.model")
	params := json.RawMessage(`{"value":42}`)
	callResponse := json.RawMessage(`{"rid":"test.model"}`)
	modelGetResponse := json.RawMessage(`{"model":` + model + `}`)
	modelClientResponse := json.RawMessage(`{"rid":"test.model","models":{"test.model":` + resourceData("test.model") + `}}`)
	modelClientInvalidParamsResponse := json.RawMessage(`{"rid":"test.model","errors":{"test.model":{"code":"system.invalidParams","message":"Invalid parameters"}}}`)
	modelClientRequestTimeoutResponse := json.RawMessage(`{"rid":"test.model","errors":{"test.model":{"code":"system.timeout","message":"Request timeout"}}}`)
	modelClientRequestAccessDeniedResponse := json.RawMessage(`{"rid":"test.model","errors":{"test.model":{"code":"system.accessDenied","message":"Access denied"}}}`)
	// Access responses
	fullCallAccess := json.RawMessage(`{"get":true,"call":"*"}`)
	methodCallAccess := json.RawMessage(`{"get":true,"call":"new"}`)
	multiMethodCallAccess := json.RawMessage(`{"get":true,"call":"foo,new"}`)
	missingMethodCallAccess := json.RawMessage(`{"get":true,"call":"foo,bar"}`)
	noCallAccess := json.RawMessage(`{"get":true}`)

	tbl := []struct {
		Params              interface{} // Params to use in call request
		CallAccessResponse  interface{} // Response on access request. requestTimeout means timeout
		CallResponse        interface{} // Response on new request. requestTimeout means timeout. noRequest means no request is expected
		GetResponse         interface{} // Response on get request of the newly created model.test. noRequest means no request is expected
		ModelAccessResponse interface{} // Response on access request of the newly created model.test.
		Expected            interface{} // Expected response to client
		ExpectedErrors      int         // Expected logged errors
	}{
		// Params variants
		{params, fullCallAccess, callResponse, modelGetResponse, fullCallAccess, modelClientResponse, 1},
		{nil, fullCallAccess, callResponse, modelGetResponse, fullCallAccess, modelClientResponse, 1},
		// CallAccessResponse variants
		{params, methodCallAccess, callResponse, modelGetResponse, fullCallAccess, modelClientResponse, 1},
		{params, multiMethodCallAccess, callResponse, modelGetResponse, fullCallAccess, modelClientResponse, 1},
		{params, missingMethodCallAccess, noRequest, noRequest, noRequest, reserr.ErrAccessDenied, 0},
		{params, noCallAccess, noRequest, noRequest, noRequest, reserr.ErrAccessDenied, 0},
		{params, requestTimeout, noRequest, noRequest, noRequest, mq.ErrRequestTimeout, 0},
		// CallResponse variants
		{params, fullCallAccess, reserr.ErrInvalidParams, noRequest, noRequest, reserr.ErrInvalidParams, 0},
		{params, fullCallAccess, requestTimeout, noRequest, noRequest, mq.ErrRequestTimeout, 0},
		// GetResponse variants
		{params, fullCallAccess, callResponse, reserr.ErrInvalidParams, fullCallAccess, modelClientInvalidParamsResponse, 1},
		{params, fullCallAccess, callResponse, requestTimeout, fullCallAccess, modelClientRequestTimeoutResponse, 1},
		// ModelAccessResponse variants
		{params, fullCallAccess, callResponse, modelGetResponse, json.RawMessage(`{"get":false}`), modelClientRequestAccessDeniedResponse, 1},
		{params, fullCallAccess, callResponse, modelGetResponse, reserr.ErrInvalidParams, modelClientInvalidParamsResponse, 1},
		{params, fullCallAccess, callResponse, modelGetResponse, reserr.ErrAccessDenied, modelClientRequestAccessDeniedResponse, 1},
		{params, fullCallAccess, callResponse, modelGetResponse, requestTimeout, modelClientRequestTimeoutResponse, 1},
		// Invalid service responses
		{nil, fullCallAccess, []byte(`{"broken":JSON}`), noRequest, noRequest, reserr.CodeInternalError, 0},
		{nil, fullCallAccess, []byte(`42`), noRequest, noRequest, reserr.CodeInternalError, 0},
		{nil, fullCallAccess, []byte(`{}`), noRequest, noRequest, reserr.CodeInternalError, 0},
		{nil, fullCallAccess, []byte(`{"result":{"foo":"bar"},"error":{"code":"system.custom","message":"Custom"}}`), noRequest, noRequest, "system.custom", 0},
		{nil, fullCallAccess, json.RawMessage(`{}`), noRequest, noRequest, reserr.CodeInternalError, 0},
		{nil, fullCallAccess, json.RawMessage(`{"rid":""}`), noRequest, noRequest, reserr.CodeInternalError, 1},
		{nil, fullCallAccess, json.RawMessage(`{"rid":"?q=foo"}`), noRequest, noRequest, reserr.CodeInternalError, 1},
		{nil, fullCallAccess, json.RawMessage(`{"rid":"test\tmodel"}`), noRequest, noRequest, reserr.CodeInternalError, 1},
		{nil, fullCallAccess, json.RawMessage(`{"rid":"test\nmodel"}`), noRequest, noRequest, reserr.CodeInternalError, 1},
		{nil, fullCallAccess, json.RawMessage(`{"rid":"test\rmodel"}`), noRequest, noRequest, reserr.CodeInternalError, 1},
		{nil, fullCallAccess, json.RawMessage(`{"rid":"test model"}`), noRequest, noRequest, reserr.CodeInternalError, 1},
		{nil, fullCallAccess, json.RawMessage(`{"rid":"test\ufffdmodel"}`), noRequest, noRequest, reserr.CodeInternalError, 1},
		{nil, fullCallAccess, json.RawMessage(`{"rid":42}`), noRequest, noRequest, reserr.CodeInternalError, 0},
		{nil, fullCallAccess, json.RawMessage(`{"rid":"test.model","foo":42}`), noRequest, noRequest, reserr.CodeInternalError, 0},
		// Invalid auth error response
		{nil, fullCallAccess, []byte(`{"error":[]}`), noRequest, noRequest, reserr.CodeInternalError, 0},
		{nil, fullCallAccess, []byte(`{"error":{"message":"missing code"}}`), noRequest, noRequest, "", 0},
		{nil, fullCallAccess, []byte(`{"error":{"code":12,"message":"integer code"}}`), noRequest, noRequest, reserr.CodeInternalError, 0},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			c := s.Connect()

			// Send client new request
			creq := c.Request("new.test.collection", l.Params)

			req := s.GetRequest(t)
			req.AssertSubject(t, "access.test.collection")
			req.AssertPathMissing(t, "isHttp")
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

			if l.GetResponse != noRequest {
				mreqs := s.GetParallelRequests(t, 2)

				// Send get response
				req := mreqs.GetRequest(t, "get.test.model")
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

			// Validate logged errors
			s.AssertErrorsLogged(t, l.ExpectedErrors)
		})
	}
}
