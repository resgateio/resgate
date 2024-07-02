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
	return subscribeToResource(t, s, c, "test.model")
}

func subscribeToResource(t *testing.T, s *Session, c *Conn, rid string) string {
	rsrc, ok := resources[rid]
	if !ok {
		panic("no resource named " + rid)
	}
	return subscribeToCustomResource(t, s, c, rid, rsrc)
}

func subscribeToCachedResource(t *testing.T, s *Session, c *Conn, rid string) string {
	rsrc, ok := resources[rid]
	if !ok {
		panic("no resource named " + rid)
	}
	return subscribeToCustomResourceExt(t, s, c, rid, rsrc, true)
}

func subscribeToCustomResource(t *testing.T, s *Session, c *Conn, rid string, rsrc resource) string {
	return subscribeToCustomResourceExt(t, s, c, rid, rsrc, false)
}

func subscribeToCustomResourceExt(t *testing.T, s *Session, c *Conn, rid string, rsrc resource, isCached bool) string {
	var r string
	if rsrc.typ == typeError {
		b, _ := json.Marshal(rsrc.err)
		r = string(b)
	} else {
		r = rsrc.data
	}

	// Send subscribe request
	creq := c.Request("subscribe."+rid, nil)

	// Handle model get and access request
	reqCount := 2
	if isCached {
		reqCount = 1
	}
	mreqs := s.GetParallelRequests(t, reqCount)
	// Handle access
	req := mreqs.GetRequest(t, "access."+rid)
	cid := req.PathPayload(t, "cid").(string)
	req.RespondSuccess(json.RawMessage(`{"get":true}`))
	// Handle resource and validate client response
	switch rsrc.typ {
	case typeModel:
		if !isCached {
			mreqs.GetRequest(t, "get."+rid).RespondSuccess(json.RawMessage(`{"model":` + r + `}`))
		}
		creq.GetResponse(t) // .AssertResult(t, json.RawMessage(`{"models":{"`+rid+`":`+r+`}}`))
	case typeCollection:
		if !isCached {
			mreqs.GetRequest(t, "get."+rid).RespondSuccess(json.RawMessage(`{"collection":` + r + `}`))
		}
		creq.GetResponse(t) // .AssertResult(t, json.RawMessage(`{"collections":{"`+rid+`":`+r+`}}`))
	default:
		panic("invalid type")
	}

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
	return subscribeToResource(t, s, c, "test.collection")
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
	return subscribeToQueryResource(t, s, c, "test.model", q, normq)
}

// subscribeToTestQueryCollection makes a successful subscription to test.collection
// with a query and the normalized query. Returns the connection ID (cid)
func subscribeToTestQueryCollection(t *testing.T, s *Session, c *Conn, q, normq string) string {
	return subscribeToQueryResource(t, s, c, "test.collection", q, normq)
}

func subscribeToQueryResource(t *testing.T, s *Session, c *Conn, rid, q, normq string) string {
	rsrc, ok := resources[rid]
	if !ok {
		panic("no resource named " + rid)
	}
	var r string
	if rsrc.typ == typeError {
		b, _ := json.Marshal(rsrc.err)
		r = string(b)
	} else {
		r = rsrc.data
	}

	normqj, err := json.Marshal(normq)
	if err != nil {
		panic("test: failed to marshal normalized query: " + err.Error())
	}

	fullrid := rid
	if q != "" {
		fullrid += "?" + q
	}
	qj, err := json.Marshal(fullrid)
	if err != nil {
		panic("test: failed to marshal query: " + err.Error())
	}

	// Send subscribe request
	creq := c.Request("subscribe."+fullrid, nil)

	// Handle resource get and access request
	mreqs := s.GetParallelRequests(t, 2)
	// Handle access request
	req := mreqs.GetRequest(t, "access."+rid)
	if q != "" {
		req.AssertPathPayload(t, "query", q)
	}
	cid := req.PathPayload(t, "cid").(string)
	req.RespondSuccess(json.RawMessage(`{"get":true}`))
	// Handle resource and validate client response
	req = mreqs.GetRequest(t, "get."+rid)
	if q != "" {
		req.AssertPathPayload(t, "query", q)
	}
	switch rsrc.typ {
	case typeModel:
		req.RespondSuccess(json.RawMessage(`{"model":` + r + `,"query":` + string(normqj) + `}`))
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{`+string(qj)+`:`+r+`}}`))
	case typeCollection:
		req.RespondSuccess(json.RawMessage(`{"collection":` + r + `,"query":` + string(normqj) + `}`))
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{`+string(qj)+`:`+r+`}}`))
	default:
		panic("invalid type")
	}
	return cid
}
