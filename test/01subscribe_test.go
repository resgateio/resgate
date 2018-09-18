package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jirenius/resgate/server/mq"
	"github.com/jirenius/resgate/server/reserr"
)

// Test that the server starts and stops the server without error
func TestStart(t *testing.T) {
	runTest(t, func(s *Session) {})
}

// Test that a client can connect to the server without error
func TestConnectClient(t *testing.T) {
	runTest(t, func(s *Session) {
		s.Connect()
	})
}

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
	model := resource["test.model"]
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
				runTest(t, func(s *Session) {
					panicked := true
					defer func() {
						if panicked {
							s := fmt.Sprintf("Error in test %d when using %#v", i, method)
							if getFirst {
								s += " with get response sent first"
							} else {
								s += " with access response sent first"
							}
							t.Logf("%s", s)
						}
					}()

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

					panicked = false
				})
			}
		}
	}
}

// Test that a response with linked models is sent to the client on a client
// subscribe request
func TestResponseOnLinkedModelSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModelParent(t, s, c, false)
	})
}

// Test that events are subscribed to on linked model subscription
func TestEventsOnLinkedModelSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		subscribeToTestModelParent(t, s, c, false)

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.custom", event)

		// Send event on model parent and validate client event
		s.ResourceEvent("test.model.parent", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.parent.custom", event)
	})
}

// Test that a response with chain-linked models is sent to the client on a client
// subscribe request
func TestResponseOnChainLinkedModelSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resource["test.model"]
		modelParent := resource["test.model.parent"]
		modelGrandparent := resource["test.model.grandparent"]

		c := s.Connect()
		creq := c.Request("subscribe.test.model.grandparent", nil)

		// Handle grandparent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "get.test.model.grandparent")
		req.RespondSuccess(json.RawMessage(`{"model":` + modelGrandparent + `}`))
		req = mreqs.GetRequest(t, "access.test.model.grandparent")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Handle parent get request
		mreqs = s.GetParallelRequests(t, 1)
		req = mreqs.GetRequest(t, "get.test.model.parent")
		req.RespondSuccess(json.RawMessage(`{"model":` + modelParent + `}`))

		// Handle child get request
		mreqs = s.GetParallelRequests(t, 1)
		req = mreqs.GetRequest(t, "get.test.model")
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`,"test.model.parent":`+modelParent+`,"test.model.grandparent":`+modelGrandparent+`}}`))
	})
}

// Test that a response with circular linked models is sent to the client on a client
// subscribe request
func TestResponseOnCircularModelSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		modelA := resource["test.model.a"]
		modelB := resource["test.model.b"]

		c := s.Connect()
		creq := c.Request("subscribe.test.model.a", nil)

		// Handle parent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "get.test.model.a")
		req.RespondSuccess(json.RawMessage(`{"model":` + modelA + `}`))
		req = mreqs.GetRequest(t, "access.test.model.a")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Handle child get request
		mreqs = s.GetParallelRequests(t, 1)
		req = mreqs.GetRequest(t, "get.test.model.b")
		req.RespondSuccess(json.RawMessage(`{"model":` + modelB + `}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model.a":`+modelA+`,"test.model.b":`+modelB+`}}`))
	})
}

// Test that a subscribe response contains only unsubscribed resources of linked
// models is sent to the client on a client subscribe request
func TestResponseOnLinkedModelSubscribeWithOverlappingSubscription(t *testing.T) {
	runTest(t, func(s *Session) {
		modelSecondParent := resource["test.model.secondparent"]

		c := s.Connect()
		subscribeToTestModelParent(t, s, c, false)

		// Get second parent model
		creq := c.Request("subscribe.test.model.secondparent", nil)

		// Handle parent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "get.test.model.secondparent").RespondSuccess(json.RawMessage(`{"model":` + modelSecondParent + `}`))
		mreqs.GetRequest(t, "access.test.model.secondparent").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model.secondparent":`+modelSecondParent+`}}`))
	})
}

// Test that a response with a collection is sent to the client on a client
// subscribe request
func TestResponseOnPrimitiveCollectionSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestCollection(t, s, c)
	})
}

// Test that a response with linked collections is sent to the client on a client
// subscribe request
func TestResponseOnLinkedCollectionSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestCollectionParent(t, s, c, false)
	})
}

// Test that a response with chain-linked collections is sent to the client on a client
// subscribe request
func TestResponseOnChainLinkedCollectionSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		collection := resource["test.collection"]
		collectionParent := resource["test.collection.parent"]
		collectionGrandparent := resource["test.collection.grandparent"]

		c := s.Connect()
		creq := c.Request("subscribe.test.collection.grandparent", nil)

		// Handle grandparent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "get.test.collection.grandparent")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collectionGrandparent + `}`))
		req = mreqs.GetRequest(t, "access.test.collection.grandparent")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Handle parent get request
		mreqs = s.GetParallelRequests(t, 1)
		req = mreqs.GetRequest(t, "get.test.collection.parent")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collectionParent + `}`))

		// Handle child get request
		mreqs = s.GetParallelRequests(t, 1)
		req = mreqs.GetRequest(t, "get.test.collection")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+collection+`,"test.collection.parent":`+collectionParent+`,"test.collection.grandparent":`+collectionGrandparent+`}}`))
	})
}

// Test that a response with circular linked collections is sent to the client on a client
// subscribe request
func TestResponseOnCircularCollectionSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		collectionA := resource["test.collection.a"]
		collectionB := resource["test.collection.b"]

		c := s.Connect()
		creq := c.Request("subscribe.test.collection.a", nil)

		// Handle parent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "get.test.collection.a")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collectionA + `}`))
		req = mreqs.GetRequest(t, "access.test.collection.a")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Handle child get request
		mreqs = s.GetParallelRequests(t, 1)
		req = mreqs.GetRequest(t, "get.test.collection.b")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collectionB + `}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"collections":{"test.collection.a":`+collectionA+`,"test.collection.b":`+collectionB+`}}`))
	})
}

// Test that a subscribe response contains only unsubscribed resources of linked
// collections is sent to the client on a client subscribe request
func TestResponseOnLinkedCollectionSubscribeWithOverlappingSubscription(t *testing.T) {
	runTest(t, func(s *Session) {
		collectionSecondParent := resource["test.collection.secondparent"]

		c := s.Connect()
		subscribeToTestCollectionParent(t, s, c, false)

		// Get second parent collection
		creq := c.Request("subscribe.test.collection.secondparent", nil)

		// Handle parent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "get.test.collection.secondparent").RespondSuccess(json.RawMessage(`{"collection":` + collectionSecondParent + `}`))
		mreqs.GetRequest(t, "access.test.collection.secondparent").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{"test.collection.secondparent":`+collectionSecondParent+`}}`))
	})
}
