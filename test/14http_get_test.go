package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jirenius/resgate/server/reserr"
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

// Test getting a primitive query model with a HTTP GET request
func TestHTTPGetOnPrimitiveQueryModel(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resource["test.model"]

		hreq := s.HTTPRequest("GET", "/api/test/model?q=foo&f=bar", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.
			GetRequest(t, "access.test.model").
			AssertPathPayload(t, "token", nil).
			AssertPathPayload(t, "query", "q=foo&f=bar").
			RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.
			GetRequest(t, "get.test.model").
			AssertPathPayload(t, "query", "q=foo&f=bar").
			RespondSuccess(json.RawMessage(`{"model":` + model + `,"query":"q=foo&f=bar"}`))

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(model))
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

// Test getting a primitive query collection with a HTTP GET request
func TestHTTPGetOnPrimitiveQueryCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		collection := resource["test.collection"]

		hreq := s.HTTPRequest("GET", "/api/test/collection?q=foo&f=bar", nil)

		// Handle collection get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.
			GetRequest(t, "access.test.collection").
			AssertPathPayload(t, "token", nil).
			AssertPathPayload(t, "query", "q=foo&f=bar").
			RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.
			GetRequest(t, "get.test.collection").
			AssertPathPayload(t, "query", "q=foo&f=bar").
			RespondSuccess(json.RawMessage(`{"collection":` + collection + `,"query":"q=foo&f=bar"}`))

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(collection))
	})
}

// Test invalid urls for HTTP get requests
func TestHTTPGetInvalidURLs(t *testing.T) {
	tbl := []struct {
		URL          string // Url path
		ExpectedCode int
		Expected     interface{}
	}{
		{"/wrong_prefix/test/model", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/test.model", http.StatusNotFound, reserr.ErrNotFound},
	}

	for i, l := range tbl {
		runTest(t, func(s *Session) {
			panicked := true
			defer func() {
				if panicked {
					t.Logf("Error in test %d", i)
				}
			}()

			hreq := s.HTTPRequest("GET", l.URL, nil)
			hresp := hreq.
				GetResponse(t).
				AssertStatusCode(t, l.ExpectedCode)

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
