package test

import (
	"encoding/json"
	"testing"
)

// Test that a client can unsubscribe to a model
func TestUnsubscribeModel(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resource["test.model"]
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		creq := c.Request("subscribe.test.model", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.model")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.model")
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Get client response
		creq.GetResponse(t)

		// Call unsubscribe
		c.Request("unsubscribe.test.model", nil).GetResponse(t)

		// Send event on model and validate no event was sent to client
		s.Event("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")
	})
}

// Test that a client can unsubscribe to linked models
func TestUnsubscribeLinkedModel(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resource["test.model"]
		modelParent := resource["test.model.parent"]
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		creq := c.Request("subscribe.test.model.parent", nil)

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

		// Call unsubscribe
		c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)

		// Send event on model and validate no event was sent to client
		s.Event("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")

		// Send event on model parent and validate no event was sent to client
		s.Event("test.model.parent", "custom", event)
		c.AssertNoEvent(t, "test.model.parent")
	})
}

// Test that an overlapping indirectly subscribed model is still subscribed
// after one parent is unsubscribed
func TestUnsubscribeOnOverlappingLinkedModel(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resource["test.model"]
		modelParent := resource["test.model.parent"]
		modelSecondParent := resource["test.model.secondparent"]
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		// Get model
		creq := c.Request("subscribe.test.model.parent", nil)

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

		// Get second parent model
		creq = c.Request("subscribe.test.model.secondparent", nil)

		// Handle parent get and access request
		mreqs = s.GetParallelRequests(t, 2)
		req = mreqs.GetRequest(t, "get.test.model.secondparent")
		req.RespondSuccess(json.RawMessage(`{"model":` + modelSecondParent + `}`))
		req = mreqs.GetRequest(t, "access.test.model.secondparent")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Get client response
		creq.GetResponse(t)

		// Call unsubscribe
		c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)

		// Send event on model and validate no event was sent to client
		s.Event("test.model.parent", "custom", event)
		c.AssertNoEvent(t, "test.model.parent")

		// Send event on model parent and validate client event
		s.Event("test.model.secondparent", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.secondparent.custom", event)
	})
}

// Test that a client can unsubscribe to a collection
func TestUnsubscribeCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		collection := resource["test.collection"]
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		creq := c.Request("subscribe.test.collection", nil)

		// Handle collection get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.collection")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.collection")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		// Get client response
		creq.GetResponse(t)

		// Call unsubscribe
		c.Request("unsubscribe.test.collection", nil).GetResponse(t)

		// Send event on collection and validate no event was sent to client
		s.Event("test.collection", "custom", event)
		c.AssertNoEvent(t, "test.collection")
	})
}

// Test that a client can unsubscribe to linked collections
func TestUnsubscribeLinkedCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		collection := resource["test.collection"]
		collectionParent := resource["test.collection.parent"]
		event := json.RawMessage(`{"foo":"bar"}`)

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

		// Get client response
		creq.GetResponse(t)

		// Call unsubscribe
		c.Request("unsubscribe.test.collection.parent", nil).GetResponse(t)

		// Send event on collection and validate no event was sent to client
		s.Event("test.collection", "custom", event)
		c.AssertNoEvent(t, "test.collection")

		// Send event on collection parent and validate no event was sent to client
		s.Event("test.collection.parent", "custom", event)
		c.AssertNoEvent(t, "test.collection.parent")
	})
}

// Test that an overlapping indirectly subscribed collection is still subscribed
// after one parent is unsubscribed
func TestUnsubscribeOnOverlappingLinkedCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		collection := resource["test.collection"]
		collectionParent := resource["test.collection.parent"]
		collectionSecondParent := resource["test.collection.secondparent"]
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		// Get collection
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

		// Get client response
		creq.GetResponse(t)

		// Get second parent collection
		creq = c.Request("subscribe.test.collection.secondparent", nil)

		// Handle parent get and access request
		mreqs = s.GetParallelRequests(t, 2)
		req = mreqs.GetRequest(t, "get.test.collection.secondparent")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collectionSecondParent + `}`))
		req = mreqs.GetRequest(t, "access.test.collection.secondparent")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Get client response
		creq.GetResponse(t)

		// Call unsubscribe
		c.Request("unsubscribe.test.collection.parent", nil).GetResponse(t)

		// Send event on collection and validate no event was sent to client
		s.Event("test.collection.parent", "custom", event)
		c.AssertNoEvent(t, "test.collection.parent")

		// Send event on collection parent and validate client event
		s.Event("test.collection.secondparent", "custom", event)
		c.GetEvent(t).Equals(t, "test.collection.secondparent.custom", event)
	})
}
