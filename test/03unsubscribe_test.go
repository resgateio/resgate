package test

import (
	"encoding/json"
	"testing"

	"github.com/jirenius/resgate/reserr"
)

// Test that a client can unsubscribe to a model
func TestUnsubscribeModel(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		subscribeToTestModel(t, s, c)

		// Call unsubscribe
		c.Request("unsubscribe.test.model", nil).GetResponse(t)

		// Send event on model and validate no event was sent to client
		s.ResourceEvent("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")
	})
}

// Test unsubscribing without subscription
func TestUnsubscribeWithoutSubscription(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Call unsubscribe
		c.Request("unsubscribe.test.model", nil).GetResponse(t).AssertError(t, reserr.ErrNoSubscription)
	})
}

// Test that a client can unsubscribe to linked models
func TestUnsubscribeLinkedModel(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		subscribeToTestModelParent(t, s, c, false)

		// Call unsubscribe
		c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)

		// Send event on model and validate no event was sent to client
		s.ResourceEvent("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")

		// Send event on model parent and validate no event was sent to client
		s.ResourceEvent("test.model.parent", "custom", event)
		c.AssertNoEvent(t, "test.model.parent")
	})
}

// Test that an overlapping indirectly subscribed model is still subscribed
// after one parent is unsubscribed
func TestUnsubscribeOnOverlappingLinkedModel(t *testing.T) {
	runTest(t, func(s *Session) {
		modelSecondParent := resource["test.model.secondparent"]
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		subscribeToTestModelParent(t, s, c, false)

		// Get second parent model
		creq := c.Request("subscribe.test.model.secondparent", nil)

		// Handle parent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "get.test.model.secondparent").RespondSuccess(json.RawMessage(`{"model":` + modelSecondParent + `}`))
		mreqs.GetRequest(t, "access.test.model.secondparent").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Get client response
		creq.GetResponse(t)

		// Call unsubscribe
		c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)

		// Send event on model and validate no event was sent to client
		s.ResourceEvent("test.model.parent", "custom", event)
		c.AssertNoEvent(t, "test.model.parent")

		// Send event on model parent and validate client event
		s.ResourceEvent("test.model.secondparent", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.secondparent.custom", event)
	})
}

// Test that a client can unsubscribe to a collection
func TestUnsubscribeCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		subscribeToTestCollection(t, s, c)

		// Call unsubscribe
		c.Request("unsubscribe.test.collection", nil).GetResponse(t)

		// Send event on collection and validate no event was sent to client
		s.ResourceEvent("test.collection", "custom", event)
		c.AssertNoEvent(t, "test.collection")
	})
}

// Test that a client can unsubscribe to linked collections
func TestUnsubscribeLinkedCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		subscribeToTestCollectionParent(t, s, c, false)

		// Call unsubscribe
		c.Request("unsubscribe.test.collection.parent", nil).GetResponse(t)

		// Send event on collection and validate no event was sent to client
		s.ResourceEvent("test.collection", "custom", event)
		c.AssertNoEvent(t, "test.collection")

		// Send event on collection parent and validate no event was sent to client
		s.ResourceEvent("test.collection.parent", "custom", event)
		c.AssertNoEvent(t, "test.collection.parent")
	})
}

// Test that an overlapping indirectly subscribed collection is still subscribed
// after one parent is unsubscribed
func TestUnsubscribeOnOverlappingLinkedCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		collectionSecondParent := resource["test.collection.secondparent"]
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		subscribeToTestCollectionParent(t, s, c, false)

		// Get second parent collection
		creq := c.Request("subscribe.test.collection.secondparent", nil)

		// Handle parent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "get.test.collection.secondparent").RespondSuccess(json.RawMessage(`{"collection":` + collectionSecondParent + `}`))
		mreqs.GetRequest(t, "access.test.collection.secondparent").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Get client response
		creq.GetResponse(t)

		// Call unsubscribe
		c.Request("unsubscribe.test.collection.parent", nil).GetResponse(t)

		// Send event on collection and validate no event was sent to client
		s.ResourceEvent("test.collection.parent", "custom", event)
		c.AssertNoEvent(t, "test.collection.parent")

		// Send event on collection parent and validate client event
		s.ResourceEvent("test.collection.secondparent", "custom", event)
		c.GetEvent(t).Equals(t, "test.collection.secondparent.custom", event)
	})
}
