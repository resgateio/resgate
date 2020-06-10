package test

import (
	"encoding/json"
	"testing"
)

func TestDeleteEvent_OnModel_SentToClient(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModel(t, s, c)

		// Send delete event
		s.ResourceEvent("test.model", "delete", nil)

		// Validate the delete event is sent to client
		c.GetEvent(t).Equals(t, "test.model.delete", nil)
	})
}

func TestDeleteEvent_OnCollection_SentToClient(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestCollection(t, s, c)

		// Send delete event
		s.ResourceEvent("test.collection", "delete", nil)

		// Validate the delete event is sent to client
		c.GetEvent(t).Equals(t, "test.collection.delete", nil)
	})
}

func TestDeleteEvent_OnStatic_SentToClient(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToResource(t, s, c, "test.static")

		// Send delete event
		s.ResourceEvent("test.static", "delete", nil)

		// Validate the delete event is sent to client
		c.GetEvent(t).Equals(t, "test.static.delete", nil)
	})
}

func TestDeleteEvent_AndCustomEventOnModel_CustomEventNotSentToClient(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModel(t, s, c)
		// Send delete event
		s.ResourceEvent("test.model", "delete", nil)
		c.GetEvent(t).Equals(t, "test.model.delete", nil)
		// Send custom event on model and validate no event
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.AssertNoEvent(t, "test.model")
	})
}

func TestDeleteEvent_AndCustomEventOnCollection_CustomEventNotSentToClient(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestCollection(t, s, c)
		// Send delete event
		s.ResourceEvent("test.collection", "delete", nil)
		c.GetEvent(t).Equals(t, "test.collection.delete", nil)
		// Send custom event on collection and validate no event
		s.ResourceEvent("test.collection", "custom", common.CustomEvent())
		c.AssertNoEvent(t, "test.collection")
	})
}

func TestDeleteEvent_AndCustomEventOnStatic_CustomEventNotSentToClient(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToResource(t, s, c, "test.static")
		// Send delete event
		s.ResourceEvent("test.static", "delete", nil)
		c.GetEvent(t).Equals(t, "test.static.delete", nil)
		// Send custom event on static and validate no event
		s.ResourceEvent("test.static", "custom", common.CustomEvent())
		c.AssertNoEvent(t, "test.static")
	})
}

func TestDeleteEvent_PriorToGetResponse_IsDiscarded(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")
		c := s.Connect()

		// Send subscribe request
		creq := c.Request("subscribe.test.model", nil)
		// Wait for get and access request
		mreqs := s.GetParallelRequests(t, 2)
		// Send delete event
		s.ResourceEvent("test.model", "delete", nil)
		// Respond to get and access request
		mreqs.GetRequest(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		mreqs.GetRequest(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))
		// Validate client response and validate
		creq.GetResponse(t)
		c.AssertNoEvent(t, "test.model")
		// Send event on model and validate it is sent
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.model.custom", common.CustomEvent())
	})
}

func TestDeleteEvent_FollowedBySubscribe_IsNotCached(t *testing.T) {
	runTest(t, func(s *Session) {
		c1 := s.Connect()
		c2 := s.Connect()

		// Subscribe with first client
		subscribeToTestModel(t, s, c1)
		// Send delete event
		s.ResourceEvent("test.model", "delete", nil)
		// Validate the delete event is sent to client
		c1.GetEvent(t).Equals(t, "test.model.delete", nil)

		// Subscribe with second client
		subscribeToTestModel(t, s, c2)
		// Send custom event
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c1.AssertNoEvent(t, "test.model")
		c2.GetEvent(t).Equals(t, "test.model.custom", common.CustomEvent())
	})
}

func TestDeleteEvent_FollowedByResubscribe_IsNotCached(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()

		// Subscribe with first client
		subscribeToTestModel(t, s, c)
		// Send delete event
		s.ResourceEvent("test.model", "delete", nil)
		// Validate the delete event is sent to client
		c.GetEvent(t).Equals(t, "test.model.delete", nil)
		// Send custom event and assert event not sent to client
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.AssertNoEvent(t, "test.model")
		// Resubscribe
		creq := c.Request("unsubscribe.test.model", nil)
		creq.GetResponse(t)
		subscribeToTestModel(t, s, c)
		// Send custom event and assert event is sent to client
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.model.custom", common.CustomEvent())
	})
}

func TestDeleteEvent_OnModelQueuedForEviction_DoesNothing(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModel(t, s, c)

		// Unsubscribe to resource
		c.Request("unsubscribe.test.model", nil).GetResponse(t)

		// Send delete event
		s.ResourceEvent("test.model", "delete", nil)

		// Validate the delete event is sent to client
		c.AssertNoEvent(t, "test.model")
	})
}

func TestDeleteEvent_OnCollectionQueuedForEviction_DoesNothing(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestCollection(t, s, c)

		// Unsubscribe to resource
		c.Request("unsubscribe.test.collection", nil).GetResponse(t)

		// Send delete event
		s.ResourceEvent("test.collection", "delete", nil)

		// Validate the delete event is sent to client
		c.AssertNoEvent(t, "test.collection")
	})
}

func TestDeleteEvent_OnStaticQueuedForEviction_DoesNothing(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToResource(t, s, c, "test.static")

		// Unsubscribe to resource
		c.Request("unsubscribe.test.static", nil).GetResponse(t)

		// Send delete event
		s.ResourceEvent("test.static", "delete", nil)

		// Validate the delete event is sent to client
		c.AssertNoEvent(t, "test.static")
	})
}
