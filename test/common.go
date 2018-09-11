package test

import (
	"encoding/json"
	"testing"
)

// subscribeToTestModel makes a successful subscription to test.model
// Returns the connection ID (cid)
func subscribeToTestModel(t *testing.T, s *Session, c *Conn) string {
	model := resource["test.model"]

	// Send subscribe request
	creq := c.Request("subscribe.test.model", nil)

	// Handle model get and access request
	mreqs := s.GetParallelRequests(t, 2)
	mreqs.GetRequest(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
	req := mreqs.GetRequest(t, "access.test.model")
	cid := req.PathPayload(t, "cid").(string)
	req.RespondSuccess(json.RawMessage(`{"get":true}`))

	// Validate client response and validate
	creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))

	return cid
}

// subscribeToTestModelParent makes a successful subscription to test.model.parent
// Returns the connection ID (cid)
func subscribeToTestModelParent(t *testing.T, s *Session, c *Conn, childIsSubscribed bool) string {
	model := resource["test.model"]
	modelParent := resource["test.model.parent"]

	// Send subscribe request
	creq := c.Request("subscribe.test.model.parent", nil)

	// Handle parent get and access request
	mreqs := s.GetParallelRequests(t, 2)
	mreqs.GetRequest(t, "get.test.model.parent").RespondSuccess(json.RawMessage(`{"model":` + modelParent + `}`))
	req := mreqs.GetRequest(t, "access.test.model.parent")
	cid := req.PathPayload(t, "cid").(string)
	req.RespondSuccess(json.RawMessage(`{"get":true}`))

	if childIsSubscribed {
		// Get client response
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model.parent":`+modelParent+`}}`))
	} else {
		// Handle child get request and validate
		mreqs = s.GetParallelRequests(t, 1)
		mreqs.GetRequest(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Get client response and validate
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`,"test.model.parent":`+modelParent+`}}`))
	}

	return cid
}

// subscribeToTestCollection makes a successful subscription to test.collection
// Returns the connection ID (cid) of the access request
func subscribeToTestCollection(t *testing.T, s *Session, c *Conn) string {
	collection := resource["test.collection"]

	// Send subscribe request
	creq := c.Request("subscribe.test.collection", nil)

	// Handle collection get and access request
	mreqs := s.GetParallelRequests(t, 2)
	mreqs.GetRequest(t, "get.test.collection").RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))
	req := mreqs.GetRequest(t, "access.test.collection")
	cid := req.PathPayload(t, "cid").(string)
	req.RespondSuccess(json.RawMessage(`{"get":true}`))

	// Validate client response and validate
	creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+collection+`}}`))

	return cid
}

// subscribeToTestCollectionParent makes a successful subscription to test.collection.parent
// Returns the connection ID (cid)
func subscribeToTestCollectionParent(t *testing.T, s *Session, c *Conn, childIsSubscribed bool) string {
	collection := resource["test.collection"]
	collectionParent := resource["test.collection.parent"]

	// Send subscribe request
	creq := c.Request("subscribe.test.collection.parent", nil)

	// Handle parent get and access request
	mreqs := s.GetParallelRequests(t, 2)
	mreqs.GetRequest(t, "get.test.collection.parent").RespondSuccess(json.RawMessage(`{"collection":` + collectionParent + `}`))
	req := mreqs.GetRequest(t, "access.test.collection.parent")
	cid := req.PathPayload(t, "cid").(string)
	req.RespondSuccess(json.RawMessage(`{"get":true}`))

	if childIsSubscribed {
		// Get client response and validate
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{"test.collection.parent":`+collectionParent+`}}`))
	} else {
		// Handle child get request
		mreqs = s.GetParallelRequests(t, 1)
		mreqs.GetRequest(t, "get.test.collection").RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		// Get client response and validate
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+collection+`,"test.collection.parent":`+collectionParent+`}}`))
	}

	return cid
}

// getCID extracts the connection ID (cid) using an auth request
// Returns the connection ID (cid)
func getCID(t *testing.T, s *Session, c *Conn) string {
	creq := c.Request("auth.test.method", nil)
	req := s.GetRequest(t).AssertSubject(t, "auth.test.method")
	cid := req.PathPayload(t, "cid").(string)
	req.RespondSuccess(nil)
	creq.GetResponse(t)
	return cid
}
