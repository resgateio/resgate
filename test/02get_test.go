package test

import (
	"encoding/json"
	"testing"

	"github.com/resgateio/resgate/server/reserr"
)

// Test that events are not sent to a model fetched with a client get request
func TestNoEventsOnPrimitiveModelGet(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		creq := c.Request("get.test.model", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model").
			AssertPathMissing(t, "isHttp").
			RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.GetRequest(t, "get.test.model").
			AssertPathMissing(t, "isHttp").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate client response
		creq.GetResponse(t)

		// Send event on model and validate client did not get event
		s.ResourceEvent("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")
	})
}

// Test that events are not sent to a linked model fetched with a client get request
func TestNoEventOnLinkedModelGet(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")
		modelParent := resourceData("test.model.parent")
		event := json.RawMessage(`{"foo":"bar"}`)

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

		// Get client response
		creq.GetResponse(t)

		// Send event on model and validate client did not get event
		s.ResourceEvent("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")

		// Send event on model parent and validate client did not get event
		s.ResourceEvent("test.model.parent", "custom", event)
		c.AssertNoEvent(t, "test.model.parent")
	})
}

func TestGet_WithCIDPlaceholder_ReplacesCID(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		cid := getCID(t, s, c)

		creq := c.Request("get.test.{cid}.model", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test."+cid+".model")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test."+cid+".model")
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		// Validate client response
		creq.GetResponse(t)

		// Send event on model and validate client did not get event
		s.ResourceEvent("test."+cid+".model", "custom", event)
		c.AssertNoEvent(t, "test."+cid+".model")
	})
}

func TestGet_LongResourceID_ReturnsErrSubjectTooLong(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		c.Request("get.test."+generateString(10000), nil).
			GetResponse(t).
			AssertError(t, reserr.ErrSubjectTooLong)
	})
}
