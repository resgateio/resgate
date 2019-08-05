package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/resgateio/resgate/server/mq"
	"github.com/resgateio/resgate/server/reserr"
)

// Test that a get- and access-request are sent to NATS on client subscribe
func TestGetAndAccessOnSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		c.Request("subscribe.test.model", nil)
		mreqs := s.GetParallelRequests(t, 2)

		// Validate get request
		req := mreqs.GetRequest(t, "get.test.model")
		req.AssertPayload(t, json.RawMessage(`{}`))

		// Validate access request
		req = mreqs.GetRequest(t, "access.test.model")
		req.AssertPathPayload(t, "token", json.RawMessage(`null`))
	})
}

// Test responses to client get and subscribe requests
func TestResponseOnPrimitiveModelRetrieval(t *testing.T) {
	model := resourceData("test.model")
	brokenModel := json.RawMessage(`{"foo":"bar"}`)
	brokenAccess := json.RawMessage(`{"get":"foo"}`)
	modelGetResponse := json.RawMessage(`{"model":` + model + `}`)
	fullAccess := json.RawMessage(`{"get":true}`)
	noAccess := json.RawMessage(`{"get":false}`)
	modelClientResponse := json.RawMessage(`{"models":{"test.model":` + model + `}}`)

	// *reserr.Error implies an error response. requestTimeout implies a timeout. Otherwise success.
	tbl := []struct {
		GetResponse    interface{}
		AccessResponse interface{}
		Expected       interface{}
	}{
		{modelGetResponse, fullAccess, modelClientResponse},
		{modelGetResponse, noAccess, reserr.ErrAccessDenied},
		{modelGetResponse, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{modelGetResponse, reserr.ErrInternalError, reserr.ErrInternalError},
		{modelGetResponse, requestTimeout, mq.ErrRequestTimeout},
		{modelGetResponse, brokenAccess, reserr.CodeInternalError},
		{reserr.ErrNotFound, fullAccess, reserr.ErrNotFound},
		{reserr.ErrNotFound, noAccess, reserr.ErrAccessDenied},
		{reserr.ErrNotFound, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{reserr.ErrNotFound, reserr.ErrInternalError, reserr.ErrInternalError},
		{reserr.ErrNotFound, requestTimeout, mq.ErrRequestTimeout},
		{reserr.ErrNotFound, brokenAccess, reserr.CodeInternalError},
		{reserr.ErrInternalError, fullAccess, reserr.ErrInternalError},
		{reserr.ErrInternalError, noAccess, reserr.ErrAccessDenied},
		{reserr.ErrInternalError, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{reserr.ErrInternalError, reserr.ErrDisposing, reserr.ErrDisposing},
		{reserr.ErrInternalError, requestTimeout, mq.ErrRequestTimeout},
		{reserr.ErrInternalError, brokenAccess, reserr.CodeInternalError},
		{requestTimeout, fullAccess, mq.ErrRequestTimeout},
		{requestTimeout, noAccess, reserr.ErrAccessDenied},
		{requestTimeout, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{requestTimeout, reserr.ErrInternalError, reserr.ErrInternalError},
		{requestTimeout, requestTimeout, mq.ErrRequestTimeout},
		{requestTimeout, brokenAccess, reserr.CodeInternalError},
		{brokenModel, fullAccess, reserr.CodeInternalError},
		{brokenModel, noAccess, reserr.ErrAccessDenied},
		{brokenModel, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{brokenModel, reserr.ErrInternalError, reserr.ErrInternalError},
		{brokenModel, requestTimeout, mq.ErrRequestTimeout},
		{brokenModel, brokenAccess, reserr.CodeInternalError},
		// Untrimmed whitespaces
		{json.RawMessage("\r\n \t{ \"model\":\t \r\n" + model + "\t} \t\n"), fullAccess, modelClientResponse},
		{modelGetResponse, json.RawMessage("\r\n \t{\"get\":\ttrue}\n"), modelClientResponse},
		// Invalid get response
		{[]byte(`{"invalid":JSON}`), fullAccess, reserr.CodeInternalError},
		{[]byte(``), fullAccess, reserr.CodeInternalError},
		{[]byte(`{"invalid":"response"}`), fullAccess, reserr.CodeInternalError},
		// Invalid access response
		{modelGetResponse, []byte(`{"invalid":JSON}`), reserr.CodeInternalError},
		{modelGetResponse, []byte(``), reserr.CodeInternalError},
		{modelGetResponse, []byte(`{"invalid":"response"}`), reserr.CodeInternalError},
		// Invalid get model or collection response
		{json.RawMessage(`{"model":["with","array","data"]}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"collection":{"with":"model data"}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"collection":[1,2],"model":{"and":"model"}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"array":[1,2]}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"prop":{"action":"delete"}}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"prop":{"action":"unknown"}}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"prop":{"unknown":"property"}}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"child":{"rid":false}}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"child":{"rid":false}}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"collection":["array",[1,2]]}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"collection":["prop",{"action":"delete"}]}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"collection":["prop",{"action":"unknown"}]}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"collection":["prop",{"unknown":"property"}]}`), fullAccess, reserr.CodeInternalError},
		// Invalid get error response
		{[]byte(`{"error":[]}`), fullAccess, reserr.CodeInternalError},
		{[]byte(`{"error":{"message":"missing code"}}`), fullAccess, ""},
		{[]byte(`{"error":{"code":12,"message":"integer code"}}`), fullAccess, reserr.CodeInternalError},
	}

	for i, l := range tbl {
		// Run both "get" and "subscribe" tests
		for _, method := range []string{"get", "subscribe"} {
			// Run both orders for "get" and "access" response
			for getFirst := true; getFirst; getFirst = false {
				s := fmt.Sprintf("#%d when using %#v", i+1, method)
				if getFirst {
					s += " with get response sent first"
				} else {
					s += " with access response sent first"
				}
				runNamedTest(t, s, func(s *Session) {
					c := s.Connect()
					var creq *ClientRequest
					switch method {
					case "get":
						creq = c.Request("get.test.model", nil)
					case "subscribe":
						creq = c.Request("subscribe.test.model", nil)
					}
					mreqs := s.GetParallelRequests(t, 2)

					var req *Request
					if getFirst {
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
					}

					// Send access response
					req = mreqs.GetRequest(t, "access.test.model")
					if l.AccessResponse == requestTimeout {
						req.Timeout()
					} else if err, ok := l.AccessResponse.(*reserr.Error); ok {
						req.RespondError(err)
					} else if raw, ok := l.AccessResponse.([]byte); ok {
						req.RespondRaw(raw)
					} else {
						req.RespondSuccess(l.AccessResponse)
					}

					if !getFirst {
						// Send get response
						req := mreqs.GetRequest(t, "get.test.model")
						if l.GetResponse == nil {
							req.Timeout()
						} else if err, ok := l.GetResponse.(*reserr.Error); ok {
							req.RespondError(err)
						} else if raw, ok := l.GetResponse.([]byte); ok {
							req.RespondRaw(raw)
						} else {
							req.RespondSuccess(l.GetResponse)
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
	}
}

// Test handle subscribe requests
func TestSubscribe(t *testing.T) {
	event := json.RawMessage(`{"foo":"bar"}`)

	responses := map[string][]string{
		// Model responses
		"test.model":              []string{"test.model"},
		"test.model.parent":       []string{"test.model.parent", "test.model"},
		"test.model.grandparent":  []string{"test.model.grandparent", "test.model.parent", "test.model"},
		"test.model.secondparent": []string{"test.model.secondparent", "test.model"},
		"test.model.brokenchild":  []string{"test.model.brokenchild", "test.err.notFound"},
		// Cyclic model responses
		"test.m.a": []string{"test.m.a"},
		"test.m.b": []string{"test.m.b", "test.m.c"},
		"test.m.d": []string{"test.m.d", "test.m.e", "test.m.f"},
		"test.m.g": []string{"test.m.d", "test.m.e", "test.m.f", "test.m.g"},
		"test.m.h": []string{"test.m.d", "test.m.e", "test.m.f", "test.m.h"},
		// Collection responses
		"test.collection":              []string{"test.collection"},
		"test.collection.parent":       []string{"test.collection.parent", "test.collection"},
		"test.collection.grandparent":  []string{"test.collection.grandparent", "test.collection.parent", "test.collection"},
		"test.collection.secondparent": []string{"test.collection.secondparent", "test.collection"},
		"test.collection.brokenchild":  []string{"test.collection.brokenchild", "test.err.notFound"},
		// Cyclic collection responses
		"test.c.a": []string{"test.c.a"},
		"test.c.b": []string{"test.c.b", "test.c.c"},
		"test.c.d": []string{"test.c.d", "test.c.e", "test.c.f"},
		"test.c.g": []string{"test.c.d", "test.c.e", "test.c.f", "test.c.g"},
		"test.c.h": []string{"test.c.d", "test.c.e", "test.c.f", "test.c.h"},
	}

	for i, l := range sequenceTable {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			var creq *ClientRequest
			var req *Request

			c := s.Connect()

			creqs := make(map[string]*ClientRequest)
			reqs := make(map[string]*Request)
			sentResources := make(map[string]bool)

			for _, ev := range l {
				switch ev.Event {
				case "subscribe":
					creqs[ev.RID] = c.Request("subscribe."+ev.RID, nil)
				case "access":
					for req = reqs["access."+ev.RID]; req == nil; req = reqs["access."+ev.RID] {
						treq := s.GetRequest(t)
						reqs[treq.Subject] = treq
					}
					req.RespondSuccess(json.RawMessage(`{"get":true}`))
				case "accessDenied":
					for req = reqs["access."+ev.RID]; req == nil; req = reqs["access."+ev.RID] {
						treq := s.GetRequest(t)
						reqs[treq.Subject] = treq
					}
					req.RespondSuccess(json.RawMessage(`{"get":false}`))
				case "get":
					for req = reqs["get."+ev.RID]; req == nil; req = reqs["get."+ev.RID] {
						req = s.GetRequest(t)
						reqs[req.Subject] = req
					}
					rsrc := resources[ev.RID]
					switch rsrc.typ {
					case typeModel:
						req.RespondSuccess(json.RawMessage(`{"model":` + rsrc.data + `}`))
					case typeCollection:
						req.RespondSuccess(json.RawMessage(`{"collection":` + rsrc.data + `}`))
					case typeError:
						req.RespondError(rsrc.err)
					}
				case "response":
					creq = creqs[ev.RID]
					rids := responses[ev.RID]
					models := make(map[string]json.RawMessage)
					collections := make(map[string]json.RawMessage)
					errors := make(map[string]*reserr.Error)
					for _, rid := range rids {
						if sentResources[rid] {
							continue
						}
						rsrc := resources[rid]
						switch rsrc.typ {
						case typeModel:
							models[rid] = json.RawMessage(rsrc.data)
						case typeCollection:
							collections[rid] = json.RawMessage(rsrc.data)
						case typeError:
							errors[rid] = rsrc.err
						}
						sentResources[rid] = true
					}
					m := make(map[string]interface{})
					if len(models) > 0 {
						m["models"] = models
					}
					if len(collections) > 0 {
						m["collections"] = collections
					}
					if len(errors) > 0 {
						m["errors"] = errors
					}
					creq.GetResponse(t).AssertResult(t, m)
				case "errorResponse":
					creq = creqs[ev.RID]
					creq.GetResponse(t).AssertIsError(t)
				case "event":
					s.ResourceEvent(ev.RID, "custom", event)
					c.GetEvent(t).Equals(t, ev.RID+".custom", event)
				case "noevent":
					s.ResourceEvent(ev.RID, "custom", event)
					c.AssertNoEvent(t, ev.RID)
				}
			}
		})
	}
}

// Test that a response with linked models is sent to the client on a client
// subscribe request when access response is delayed
func TestResponseOnLinkedModelSubscribeWithDelayedAccess(t *testing.T) {
	// Run multiple times as the bug GH-84 is caused by race conditions
	// and won't always be triggered.
	for i := 0; i < 100; i++ {
		runTest(t, func(s *Session) {
			c := s.Connect()
			subscribeToTestModelParentExt(t, s, c, false, true)
		})
	}
}
