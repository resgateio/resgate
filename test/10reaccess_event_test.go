package test

import (
	"encoding/json"
	"testing"
)

// Test reaccess event
func TestReaccessEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		cid := subscribeToTestModel(t, s, c)

		// Send reaccess event
		s.ResourceEvent("test.model", "reaccess", nil)

		// Validate an access request is triggered
		s.GetRequest(t).
			AssertSubject(t, "access.test.model").
			AssertPathPayload(t, "cid", cid).
			RespondSuccess(json.RawMessage(`{"get":true}`))
		c.AssertNoEvent(t, "test.model")
	})
}

// Test that a reaccess event triggers a new access call on subscribed resources
// and that the resource are still subscribed after given access
func TestReaccessEventTriggersAccessCallOnSubscribedResources(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()

		// Get linked model
		subscribeToTestModelParent(t, s, c, false)

		// Send reaccess event
		s.ResourceEvent("test.model.parent", "reaccess", nil)

		// Handle access requests with access granted
		s.GetRequest(t).AssertSubject(t, "access.test.model.parent").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate no unsubscribe events are sent to client
		c.AssertNoEvent(t, "test.model.parent")

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.custom", event)

		// Send event on model parent and validate client event
		s.ResourceEvent("test.model.parent", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.parent.custom", event)
	})
}

// Test that a reaccess event triggers a new access call on subscribed resources
// and that the resource are unsubscribed after being denied access
func TestReaccessEventTriggersUnsubscribeOnDeniedAccessCall(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)
		reasonAccessDenied := json.RawMessage(`{"reason":{"code":"system.accessDenied","message":"Access denied"}}`)

		c := s.Connect()

		// Get linked model
		subscribeToTestModelParent(t, s, c, false)

		// Send reaccess event
		s.ResourceEvent("test.model.parent", "reaccess", nil)

		// Handle access requests with access denied
		s.GetRequest(t).AssertSubject(t, "access.test.model.parent").RespondSuccess(json.RawMessage(`{"get":false}`))

		// Validate unsubscribe events are sent to client
		c.GetEvent(t).AssertEventName(t, "test.model.parent.unsubscribe").AssertData(t, reasonAccessDenied)

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")

		// Send event on model parent and validate client event
		s.ResourceEvent("test.model.parent", "custom", event)
		c.AssertNoEvent(t, "test.model.parent")
	})
}

// Test that unsubscribing a parent resource to a child that was subscribed prior
// to the parent resource will not trigger a new access call on the child.
func TestUnsubscribingParentToPresubscribedChildDoesNotTriggerAccessCall(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()

		// Get child model first
		subscribeToTestModel(t, s, c)
		// Get parent model afterwards
		subscribeToTestModelParent(t, s, c, true)

		// Call unsubscribe on parent
		c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)

		// Send event on child model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.custom", event)
	})
}

// Test that a reaccess event will queue any subsequent event for the direct
// subscribed resource, but not for the indirect.
// See issue for more information.
func TestReaccessEventQueuesEvents(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()

		// Get linked model
		subscribeToTestModelParent(t, s, c, false)

		// Send reaccess event
		s.ResourceEvent("test.model.parent", "reaccess", nil)

		// Handle access requests with access granted
		req := s.GetRequest(t).AssertSubject(t, "access.test.model.parent")

		// Send event on parent and model and validate only the model event was sent
		s.ResourceEvent("test.model.parent", "custom", event)
		s.ResourceEvent("test.model", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.custom", event)
		c.AssertNoEvent(t, "test.model.parent")

		// Respond with access
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Assert that the parent event is now sent
		c.GetEvent(t).Equals(t, "test.model.parent.custom", event)
	})
}

// Test that a reaccess event sent prior to the get response is not discarded
// and that the resource is unsubscribed if access is denied.
func TestReaccessSentBeforeGetResponseDenyingAccess(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")

		c := s.Connect()
		creq := c.Request("subscribe.test.model", nil)

		// Handle only model access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true}`))

		// Send reaccess event
		s.ResourceEvent("test.model", "reaccess", nil)

		// Then handle the get request
		mreqs.GetRequest(t, "get.test.model").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate client response
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))

		// Assert we get a new access request on model due to the reaccess
		s.GetRequest(t).AssertSubject(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":false}`))

		// Validate unsubscribe event
		c.GetEvent(t).AssertEventName(t, "test.model.unsubscribe")
	})
}

// Test that a reaccess event sent prior to the get response is not discarded
// and that the resource is still subscribed after access is re-granted.
func TestReaccessSentBeforeGetResponseGrantingAccess(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		creq := c.Request("subscribe.test.model", nil)

		// Handle only model access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true}`))

		// Send reaccess event
		s.ResourceEvent("test.model", "reaccess", nil)

		// Then handle the get request
		mreqs.GetRequest(t, "get.test.model").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate client response
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))

		// Assert we get a new access request on model due to the reaccess
		s.GetRequest(t).AssertSubject(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true}`))

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.custom", event)
	})
}

// Test that a reaccess event sent on a cyclic reference causes an access request.
func TestReaccessOnCyclicReference(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.m.a")

		c := s.Connect()
		creq := c.Request("subscribe.test.m.a", nil)

		// Handle only model access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "get.test.m.a").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		mreqs.GetRequest(t, "access.test.m.a").
			RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.m.a":`+model+`}}`))

		// Send reaccess event
		s.ResourceEvent("test.m.a", "reaccess", nil)

		// Assert we get a new access request on model due to the reaccess
		s.GetRequest(t).AssertSubject(t, "access.test.m.a").
			RespondSuccess(json.RawMessage(`{"get":true}`))
	})
}

// Test multiple reaccess events while queueing only results in one access request
func TestMultipleReaccessEventsWhileQueueing(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		creq := c.Request("subscribe.test.model", nil)

		// Handle only model access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true}`))

		// Send multiple reaccess event
		s.ResourceEvent("test.model", "reaccess", nil)
		s.ResourceEvent("test.model", "reaccess", nil)

		// Then handle the get request
		mreqs.GetRequest(t, "get.test.model").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate client response
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))

		// Assert we get a new access request on model due to the reaccess
		s.GetRequest(t).AssertSubject(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true}`))

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.custom", event)
	})
}

// Test that a reaccess event on an indirect subscription is discarded.
func TestReaccessEventOnIndirectResources(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()

		// Get linked model
		subscribeToTestModelParent(t, s, c, false)

		// Send reaccess event
		s.ResourceEvent("test.model", "reaccess", nil)

		// Send event on child model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.custom", event)
	})
}
