package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/jirenius/resgate/mq"
	"github.com/jirenius/resgate/reserr"
)

// Test getting a primitive model with a HTTP GET request
func TestHTTPGetOnPrimitiveModel(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resource["test.model"]

		hreq := s.HTTPRequest("GET", "/api/test/model", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.model")
		req.AssertPathPayload(t, "token", nil).RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.model")
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(model))
	})
}

// Test getting a linked model with a HTTP GET request
func TestHTTPGetOnLinkedModel(t *testing.T) {
	runTest(t, func(s *Session) {
		modelParent := resource["test.model.parent"]
		model := resource["test.model"]

		hreq := s.HTTPRequest("GET", "/api/test/model/parent", nil)

		// Handle parent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "get.test.model.parent").RespondSuccess(json.RawMessage(`{"model":` + modelParent + `}`))
		mreqs.GetRequest(t, "access.test.model.parent").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Handle child get request and validate
		mreqs = s.GetParallelRequests(t, 1)
		mreqs.GetRequest(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(`{"name":"parent","child":{"href":"/api/test/model","model":`+model+`}}`))
	})
}

// Test getting a chain-linked model with a HTTP GET request
func TestHTTPGetOnChainLinkedModel(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resource["test.model"]
		modelParent := resource["test.model.parent"]
		modelGrandparent := resource["test.model.grandparent"]

		hreq := s.HTTPRequest("GET", "/api/test/model/grandparent", nil)

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

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(`{"name":"grandparent","child":{"href":"/api/test/model/parent","model":{"name":"parent","child":{"href":"/api/test/model","model":`+model+`}}}}`))
	})
}

// Test getting a circularly linked model with a HTTP GET request
func TestHTTPGetOnCircularModel(t *testing.T) {
	runTest(t, func(s *Session) {
		modelA := resource["test.model.a"]
		modelB := resource["test.model.b"]

		hreq := s.HTTPRequest("GET", "/api/test/model/a", nil)

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

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(`{"bref":{"href":"/api/test/model/b","model":{"aref":{"href":"/api/test/model/a"},"bref":{"href":"/api/test/model/b"}}}}`))
	})
}

// Test getting a primitive collection with a HTTP GET request
func TestHTTPGetOnPrimitiveCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		collection := resource["test.collection"]

		hreq := s.HTTPRequest("GET", "/api/test/collection", nil)

		// Handle collection get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.collection")
		req.AssertPathPayload(t, "token", nil).RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.collection")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(collection))
	})
}

// Test getting a linked collection with a HTTP GET request
func TestHTTPGetOnLinkedCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		collectionParent := resource["test.collection.parent"]
		collection := resource["test.collection"]

		hreq := s.HTTPRequest("GET", "/api/test/collection/parent", nil)

		// Handle parent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "get.test.collection.parent").RespondSuccess(json.RawMessage(`{"collection":` + collectionParent + `}`))
		mreqs.GetRequest(t, "access.test.collection.parent").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Handle child get request and validate
		mreqs = s.GetParallelRequests(t, 1)
		mreqs.GetRequest(t, "get.test.collection").RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(`["parent",{"href":"/api/test/collection","collection":`+collection+`}]`))
	})
}

// Test getting a chain-linked collection with a HTTP GET request
func TestHTTPGetOnChainLinkedCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		collection := resource["test.collection"]
		collectionParent := resource["test.collection.parent"]
		collectionGrandparent := resource["test.collection.grandparent"]

		hreq := s.HTTPRequest("GET", "/api/test/collection/grandparent", nil)

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

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(`["grandparent",{"href":"/api/test/collection/parent","collection":["parent",{"href":"/api/test/collection","collection":`+collection+`}]},null]`))
	})
}

// Test getting a circularly linked collection with a HTTP GET request
func TestHTTPGetOnCircularCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		collectionA := resource["test.collection.a"]
		collectionB := resource["test.collection.b"]

		hreq := s.HTTPRequest("GET", "/api/test/collection/a", nil)

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

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(`[{"href":"/api/test/collection/b","collection":[{"href":"/api/test/collection/a"},{"href":"/api/test/collection/b"}]}]`))
	})
}

// Test different type of responses on getting a model with a HTTP GET request
func TestHTTPGetResponsesOnPrimitiveModel(t *testing.T) {
	model := resource["test.model"]
	brokenModel := json.RawMessage(`{"foo":"bar"}`)
	brokenAccess := json.RawMessage(`{"get":"foo"}`)
	modelGetResponse := json.RawMessage(`{"model":` + model + `}`)

	// *reserr.Error implies an error response. requestTimeout implies a timeout. Otherwise success.
	tbl := []struct {
		GetResponse    interface{}
		AccessResponse interface{}
		ExpectedCode   int
		Expected       interface{}
	}{
		{modelGetResponse, json.RawMessage(`{"get":true}`), http.StatusOK, json.RawMessage(model)},
		{modelGetResponse, json.RawMessage(`{"get":false}`), http.StatusUnauthorized, reserr.ErrAccessDenied},
		{modelGetResponse, reserr.ErrAccessDenied, http.StatusUnauthorized, reserr.ErrAccessDenied},
		{modelGetResponse, reserr.ErrInternalError, http.StatusInternalServerError, reserr.ErrInternalError},
		{modelGetResponse, requestTimeout, http.StatusNotFound, mq.ErrRequestTimeout},
		{modelGetResponse, brokenAccess, http.StatusInternalServerError, reserr.CodeInternalError},
		{reserr.ErrNotFound, json.RawMessage(`{"get":true}`), http.StatusNotFound, reserr.ErrNotFound},
		{reserr.ErrNotFound, json.RawMessage(`{"get":false}`), http.StatusUnauthorized, reserr.ErrAccessDenied},
		{reserr.ErrNotFound, reserr.ErrAccessDenied, http.StatusUnauthorized, reserr.ErrAccessDenied},
		{reserr.ErrNotFound, reserr.ErrInternalError, http.StatusInternalServerError, reserr.ErrInternalError},
		{reserr.ErrNotFound, requestTimeout, http.StatusNotFound, mq.ErrRequestTimeout},
		{reserr.ErrNotFound, brokenAccess, http.StatusInternalServerError, reserr.CodeInternalError},
		{reserr.ErrInternalError, json.RawMessage(`{"get":true}`), http.StatusInternalServerError, reserr.ErrInternalError},
		{reserr.ErrInternalError, json.RawMessage(`{"get":false}`), http.StatusUnauthorized, reserr.ErrAccessDenied},
		{reserr.ErrInternalError, reserr.ErrAccessDenied, http.StatusUnauthorized, reserr.ErrAccessDenied},
		{reserr.ErrInternalError, reserr.ErrDisposing, http.StatusInternalServerError, reserr.ErrDisposing},
		{reserr.ErrInternalError, requestTimeout, http.StatusNotFound, mq.ErrRequestTimeout},
		{reserr.ErrInternalError, brokenAccess, http.StatusInternalServerError, reserr.CodeInternalError},
		{requestTimeout, json.RawMessage(`{"get":true}`), http.StatusNotFound, mq.ErrRequestTimeout},
		{requestTimeout, json.RawMessage(`{"get":false}`), http.StatusUnauthorized, reserr.ErrAccessDenied},
		{requestTimeout, reserr.ErrAccessDenied, http.StatusUnauthorized, reserr.ErrAccessDenied},
		{requestTimeout, reserr.ErrInternalError, http.StatusInternalServerError, reserr.ErrInternalError},
		{requestTimeout, requestTimeout, http.StatusNotFound, mq.ErrRequestTimeout},
		{requestTimeout, brokenAccess, http.StatusInternalServerError, reserr.CodeInternalError},
		{brokenModel, json.RawMessage(`{"get":true}`), http.StatusInternalServerError, reserr.CodeInternalError},
		{brokenModel, json.RawMessage(`{"get":false}`), http.StatusUnauthorized, reserr.ErrAccessDenied},
		{brokenModel, reserr.ErrAccessDenied, http.StatusUnauthorized, reserr.ErrAccessDenied},
		{brokenModel, reserr.ErrInternalError, http.StatusInternalServerError, reserr.ErrInternalError},
		{brokenModel, requestTimeout, http.StatusNotFound, mq.ErrRequestTimeout},
		{brokenModel, brokenAccess, http.StatusInternalServerError, reserr.CodeInternalError},
	}

	for i, l := range tbl {
		// Run both orders for "get" and "access" response
		for getFirst := true; getFirst; getFirst = false {
			runTest(t, func(s *Session) {
				panicked := true
				defer func() {
					if panicked {
						s := fmt.Sprintf("Error in test %d", i)
						if getFirst {
							s += " with get response sent first"
						} else {
							s += " with access response sent first"
						}
						t.Logf("%s", s)
					}
				}()

				hreq := s.HTTPRequest("GET", "/api/test/model", nil)

				mreqs := s.GetParallelRequests(t, 2)

				var req *Request
				if getFirst {
					// Send get response
					req = mreqs.GetRequest(t, "get.test.model")
					if l.GetResponse == requestTimeout {
						req.Timeout()
					} else if err, ok := l.GetResponse.(*reserr.Error); ok {
						req.RespondError(err)
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
				hresp := hreq.GetResponse(t)
				hresp.AssertStatusCode(t, l.ExpectedCode)
				if err, ok := l.Expected.(*reserr.Error); ok {
					hresp.AssertError(t, err)
				} else if code, ok := l.Expected.(string); ok {
					hresp.AssertErrorCode(t, code)
				} else {
					hresp.AssertBody(t, l.Expected)
				}

				panicked = false
			})
		}
	}
}
