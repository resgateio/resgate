package test

import (
	"encoding/json"
	"testing"

	"github.com/jirenius/resgate/mq"

	"github.com/jirenius/resgate/reserr"
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
		c.Request("get.test.model", nil)
		mreqs := s.GetParallelRequests(t, 2)

		// Validate get request
		req := mreqs.GetRequest(t, "get.test.model")
		req.AssertPayload(t, json.RawMessage(`{}`))

		// Validate access request
		req = mreqs.GetRequest(t, "access.test.model")
		req.AssertPathPayload(t, "token", json.RawMessage(`null`))
	})
}

func TestResponseOnPrimitiveModelRetrieval(t *testing.T) {
	model := resource["test.model"]
	brokenModel := json.RawMessage(`{"foo":"bar"}`)
	brokenAccess := json.RawMessage(`{"get":"foo"}`)
	modelGetResponse := json.RawMessage(`{"model":` + model + `}`)
	modelClientResponse := json.RawMessage(`{"models":{"test.model":` + model + `}}`)

	// *reserr.Error implies an error response. nil implies a timeout. Otherwise success.
	tbl := []struct {
		GetResponse    interface{}
		AccessResponse interface{}
		Expected       interface{}
	}{
		{modelGetResponse, json.RawMessage(`{"get":true}`), modelClientResponse},
		{modelGetResponse, json.RawMessage(`{"get":false}`), reserr.ErrAccessDenied},
		{modelGetResponse, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{modelGetResponse, reserr.ErrInternalError, reserr.ErrInternalError},
		{modelGetResponse, nil, mq.ErrRequestTimeout},
		{modelGetResponse, brokenAccess, reserr.CodeInternalError},
		{reserr.ErrNotFound, json.RawMessage(`{"get":true}`), reserr.ErrNotFound},
		{reserr.ErrNotFound, json.RawMessage(`{"get":false}`), reserr.ErrAccessDenied},
		{reserr.ErrNotFound, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{reserr.ErrNotFound, reserr.ErrInternalError, reserr.ErrInternalError},
		{reserr.ErrNotFound, nil, mq.ErrRequestTimeout},
		{reserr.ErrNotFound, brokenAccess, reserr.CodeInternalError},
		{reserr.ErrInternalError, json.RawMessage(`{"get":true}`), reserr.ErrInternalError},
		{reserr.ErrInternalError, json.RawMessage(`{"get":false}`), reserr.ErrAccessDenied},
		{reserr.ErrInternalError, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{reserr.ErrInternalError, reserr.ErrDisposing, reserr.ErrDisposing},
		{reserr.ErrInternalError, nil, mq.ErrRequestTimeout},
		{reserr.ErrInternalError, brokenAccess, reserr.CodeInternalError},
		{nil, json.RawMessage(`{"get":true}`), mq.ErrRequestTimeout},
		{nil, json.RawMessage(`{"get":false}`), reserr.ErrAccessDenied},
		{nil, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{nil, reserr.ErrInternalError, reserr.ErrInternalError},
		{nil, nil, mq.ErrRequestTimeout},
		{nil, brokenAccess, reserr.CodeInternalError},
		{brokenModel, json.RawMessage(`{"get":true}`), reserr.CodeInternalError},
		{brokenModel, json.RawMessage(`{"get":false}`), reserr.ErrAccessDenied},
		{brokenModel, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{brokenModel, reserr.ErrInternalError, reserr.ErrInternalError},
		{brokenModel, nil, mq.ErrRequestTimeout},
		{brokenModel, brokenAccess, reserr.CodeInternalError},
	}

	for _, l := range tbl {
		// Run both "get" and "subscribe" tests
		for _, method := range []string{"get", "subscribe"} {
			// Run both orders for "get" and "access" response
			for getFirst := true; getFirst; getFirst = false {
				runTest(t, func(s *Session) {
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
						if l.GetResponse == nil {
							req.Timeout()
						} else if err, ok := l.GetResponse.(*reserr.Error); ok {
							req.RespondError(err)
						} else {
							req.RespondSuccess(l.GetResponse)
						}
					}

					// Send access response
					req = mreqs.GetRequest(t, "access.test.model")
					if l.AccessResponse == nil {
						req.Timeout()
					} else if err, ok := l.AccessResponse.(*reserr.Error); ok {
						req.RespondError(err)
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

// Test that a response with linked models is sent to the client on a client
// subscribe request
func TestResponseOnLinkedModelSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resource["test.model"]
		modelParent := resource["test.model.parent"]

		c := s.Connect()
		creq := c.Request("get.test.model.parent", nil)

		// Handle parent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "get.test.model.parent")
		req.RespondSuccess(json.RawMessage(`{"model":` + modelParent + `}`))
		req = mreqs.GetRequest(t, "access.test.model.parent")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Handle child get request
		mreqs = s.GetParallelRequests(t, 1)
		req = mreqs.GetRequest(t, "get.test.model")
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`,"test.model.parent":`+modelParent+`}}`))
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
		creq := c.Request("get.test.model.grandparent", nil)

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
		creq := c.Request("get.test.model.a", nil)

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
		model := resource["test.model"]
		modelParent := resource["test.model.parent"]

		c := s.Connect()
		// Get model
		creq := c.Request("subscribe.test.model", nil)

		// Handle parent get and access request of
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "get.test.model")
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		req = mreqs.GetRequest(t, "access.test.model")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))

		// Get parent model
		creq = c.Request("subscribe.test.model.parent", nil)

		// Handle parent get and access request
		mreqs = s.GetParallelRequests(t, 2)
		req = mreqs.GetRequest(t, "get.test.model.parent")
		req.RespondSuccess(json.RawMessage(`{"model":` + modelParent + `}`))
		req = mreqs.GetRequest(t, "access.test.model.parent")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response
		cresp = creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model.parent":`+modelParent+`}}`))
	})
}

// Test that a response with a collection is sent to the client on a client
// subscribe request
func TestResponseOnPrimitiveCollectionSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		collection := resource["test.collection"]

		c := s.Connect()
		creq := c.Request("subscribe.test.collection", nil)

		// Handle collection get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.collection")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.collection")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+collection+`}}`))
	})
}

// Test that a response with linked collections is sent to the client on a client
// subscribe request
func TestResponseOnLinkedCollectionSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		collection := resource["test.collection"]
		collectionParent := resource["test.collection.parent"]

		c := s.Connect()
		creq := c.Request("subscribe.test.collection.parent", nil)

		// Handle parent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "get.test.collection.parent")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collectionParent + `}`))
		req = mreqs.GetRequest(t, "access.test.collection.parent")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Handle child get request
		mreqs = s.GetParallelRequests(t, 1)
		req = mreqs.GetRequest(t, "get.test.collection")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+collection+`,"test.collection.parent":`+collectionParent+`}}`))
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
		collection := resource["test.collection"]
		collectionParent := resource["test.collection.parent"]

		c := s.Connect()
		// Get collection
		creq := c.Request("subscribe.test.collection", nil)

		// Handle parent get and access request of
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "get.test.collection")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))
		req = mreqs.GetRequest(t, "access.test.collection")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+collection+`}}`))

		// Get parent collection
		creq = c.Request("subscribe.test.collection.parent", nil)

		// Handle parent get and access request
		mreqs = s.GetParallelRequests(t, 2)
		req = mreqs.GetRequest(t, "get.test.collection.parent")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collectionParent + `}`))
		req = mreqs.GetRequest(t, "access.test.collection.parent")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response
		cresp = creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"collections":{"test.collection.parent":`+collectionParent+`}}`))
	})
}
