package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/resgateio/resgate/server"
	"github.com/resgateio/resgate/server/reserr"
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
		modelSecondParent := resourceData("test.model.secondparent")
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
		collectionSecondParent := resourceData("test.collection.secondparent")
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

func TestUnsubscribe_FollowedByResourceResponse_IncludesResource(t *testing.T) {
	for useCount := true; useCount; useCount = false {
		runNamedTest(t, fmt.Sprintf("with useCount set to %+v", useCount), func(s *Session) {
			c := s.Connect()
			model := resourceData("test.model")

			// Send subscribe request
			creq := c.Request("subscribe.test.model", nil)
			// Handle model get and access request
			mreqs := s.GetParallelRequests(t, 2)
			mreqs.GetRequest(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
			req := mreqs.GetRequest(t, "access.test.model")
			req.RespondSuccess(json.RawMessage(`{"get":true}`))

			// Validate client response and validate
			creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))

			// Send client request
			creq = c.Request("call.test.getModel", nil)
			req = s.GetRequest(t)
			req.AssertSubject(t, "access.test")
			req.RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))
			// Get call request
			req = s.GetRequest(t)
			req.AssertSubject(t, "call.test.getModel")
			req.RespondResource("test.model")
			// Validate client response
			cresp := creq.GetResponse(t)
			cresp.AssertResult(t, json.RawMessage(`{"rid":"test.model"}`))

			// Call unsubscribe
			if useCount {
				c.Request("unsubscribe.test.model", json.RawMessage(`{"count":2}`)).GetResponse(t)
			} else {
				c.Request("unsubscribe.test.model", json.RawMessage(`{}`)).GetResponse(t)
				c.Request("unsubscribe.test.model", nil).GetResponse(t)
			}

			// Send client request
			creq = c.Request("call.test.getModel", nil)
			req = s.GetRequest(t)
			req.AssertSubject(t, "access.test")
			req.RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))
			// Get call request
			req = s.GetRequest(t)
			req.AssertSubject(t, "call.test.getModel")
			req.RespondResource("test.model")
			// Access request
			req = s.GetRequest(t)
			req.AssertSubject(t, "access.test.model")
			req.RespondSuccess(json.RawMessage(`{"get":true}`))
			// Validate client response
			cresp = creq.GetResponse(t)
			cresp.AssertResult(t, json.RawMessage(`{"rid":"test.model","models":{"test.model":`+model+`}}`))
		})
	}
}

func TestUnsubscribe_WithCount_UnsubscribesModel(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		subscribeToTestModel(t, s, c)

		// Call unsubscribe
		c.Request("unsubscribe.test.model", json.RawMessage(`{"count":1}`)).GetResponse(t)

		// Send event on model and validate no event was sent to client
		s.ResourceEvent("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")
	})
}

func TestUnsubscribe_WithInvalidPayload_DoesNotUnsubscribesModel(t *testing.T) {
	tbl := []struct {
		Payload   interface{}
		ErrorCode string
	}{
		{json.RawMessage(`[]`), "system.invalidParams"},
		{json.RawMessage(`{"count":"foo"}`), "system.invalidParams"},
		{json.RawMessage(`{"count":true}`), "system.invalidParams"},
		{json.RawMessage(`{"count":0}`), "system.invalidParams"},
		{json.RawMessage(`{"count":-1}`), "system.invalidParams"},
		{json.RawMessage(`{"count":2}`), "system.noSubscription"},
	}

	event := json.RawMessage(`{"foo":"bar"}`)

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			c := s.Connect()
			subscribeToTestModel(t, s, c)

			// Call unsubscribe
			c.Request("unsubscribe.test.model", l.Payload).
				GetResponse(t).
				AssertErrorCode(t, l.ErrorCode)

			// Send event on model and validate it is still subscribed
			s.ResourceEvent("test.model", "custom", event)
			c.GetEvent(t).AssertEventName(t, "test.model.custom")
		})
	}
}

// Test that a model that no client subscribes to gets unsubscribed and removed
// from the cache.
func TestUnsubscribe_Model_UnsubcribesFromCache(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModel(t, s, c)

		// Call unsubscribe
		c.Request("unsubscribe.test.model", nil).GetResponse(t)
		s.AssertUnsubscribe("test.model")
	}, func(cfg *server.Config) {
		cfg.NoUnsubscribeDelay = true
	})
}

// Test that a linked model resources that no client subscribes to gets
// unsubscribed and removed from the cache.
func TestUnsubscribe_LinkedModel_UnsubscribesFromCache(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModelParent(t, s, c, false)

		// Call unsubscribe
		c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)
		s.AssertUnsubscribe("test.model", "test.model.parent")
	}, func(cfg *server.Config) {
		cfg.NoUnsubscribeDelay = true
	})
}
