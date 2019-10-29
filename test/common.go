package test

import (
	"encoding/json"
	"testing"
)

type commonData struct{}

var common = commonData{}

func (c *commonData) CustomEvent() json.RawMessage { return json.RawMessage(`{"foo":"bar"}`) }

// subscribeToTestModel makes a successful subscription to test.model
// Returns the connection ID (cid)
func subscribeToTestModel(t *testing.T, s *Session, c *Conn) string {
	model := resourceData("test.model")

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
	return subscribeToTestModelParentExt(t, s, c, childIsSubscribed, false)
}

// subscribeToTestModelParentExt makes a successful subscription to test.model.parent
// and allows to delay access response.
// Returns the connection ID (cid)
func subscribeToTestModelParentExt(t *testing.T, s *Session, c *Conn, childIsSubscribed bool, delayAccess bool) string {
	var cid string
	model := resourceData("test.model")
	modelParent := resourceData("test.model.parent")

	// Send subscribe request
	creq := c.Request("subscribe.test.model.parent", nil)

	// Handle parent get and access request
	mreqs1 := s.GetParallelRequests(t, 2)
	mreqs1.GetRequest(t, "get.test.model.parent").RespondSuccess(json.RawMessage(`{"model":` + modelParent + `}`))
	if !delayAccess || childIsSubscribed {
		req := mreqs1.GetRequest(t, "access.test.model.parent")
		cid = req.PathPayload(t, "cid").(string)
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
	}

	if childIsSubscribed {
		// Get client response
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model.parent":`+modelParent+`}}`))
	} else {
		// Handle child get request and validate
		mreqs2 := s.GetParallelRequests(t, 1)
		mreqs2.GetRequest(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		if delayAccess {
			req := mreqs1.GetRequest(t, "access.test.model.parent")
			cid = req.PathPayload(t, "cid").(string)
			req.RespondSuccess(json.RawMessage(`{"get":true}`))
		}

		// Get client response and validate
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`,"test.model.parent":`+modelParent+`}}`))
	}

	return cid
}

// subscribeToTestCollection makes a successful subscription to test.collection
// Returns the connection ID (cid) of the access request
func subscribeToTestCollection(t *testing.T, s *Session, c *Conn) string {
	collection := resourceData("test.collection")

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
	collection := resourceData("test.collection")
	collectionParent := resourceData("test.collection.parent")

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

// subscribeToTestQueryModel makes a successful subscription to test.model
// with a query and the normalized query. Returns the connection ID (cid)
func subscribeToTestQueryModel(t *testing.T, s *Session, c *Conn, q, normq string) string {
	model := resourceData("test.model")

	normqj, err := json.Marshal(normq)
	if err != nil {
		panic("test: failed to marshal normalized query: " + err.Error())
	}

	rid := "test.model"
	if q != "" {
		rid += "?" + q
	}
	qj, err := json.Marshal(rid)
	if err != nil {
		panic("test: failed to marshal query: " + err.Error())
	}

	// Send subscribe request
	creq := c.Request("subscribe."+rid, nil)

	// Handle model get and access request
	mreqs := s.GetParallelRequests(t, 2)
	req := mreqs.GetRequest(t, "get.test.model")
	if q != "" {
		req.AssertPathPayload(t, "query", q)
	}
	req.RespondSuccess(json.RawMessage(`{"model":` + model + `,"query":` + string(normqj) + `}`))
	req = mreqs.GetRequest(t, "access.test.model")
	if q != "" {
		req.AssertPathPayload(t, "query", q)
	}
	cid := req.PathPayload(t, "cid").(string)
	req.RespondSuccess(json.RawMessage(`{"get":true}`))

	// Validate client response and validate
	creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{`+string(qj)+`:`+model+`}}`))

	return cid
}

// subscribeToTestQueryCollection makes a successful subscription to test.collection
// with a query and the normalized query. Returns the connection ID (cid)
func subscribeToTestQueryCollection(t *testing.T, s *Session, c *Conn, q, normq string) string {
	collection := resourceData("test.collection")

	normqj, err := json.Marshal(normq)
	if err != nil {
		panic("test: failed to marshal normalized query: " + err.Error())
	}

	qj, err := json.Marshal("test.collection?" + q)
	if err != nil {
		panic("test: failed to marshal query: " + err.Error())
	}

	// Send subscribe request
	creq := c.Request("subscribe.test.collection?"+q, nil)

	// Handle collection get and access request
	mreqs := s.GetParallelRequests(t, 2)
	mreqs.
		GetRequest(t, "get.test.collection").
		AssertPathPayload(t, "query", q).
		RespondSuccess(json.RawMessage(`{"collection":` + collection + `,"query":` + string(normqj) + `}`))
	req := mreqs.GetRequest(t, "access.test.collection").AssertPathPayload(t, "query", q)
	cid := req.PathPayload(t, "cid").(string)
	req.RespondSuccess(json.RawMessage(`{"get":true}`))

	// Validate client response and validate
	creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{`+string(qj)+`:`+collection+`}}`))

	return cid
}
