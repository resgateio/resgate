package test

import (
	"encoding/json"
	"testing"
)

// Test system reset event
func TestSystemResetEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		// Send token event
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))
	})
}

// Test that a system.reset event triggers get requests on matching model
func TestSystemResetTriggersGetRequestOnModel(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")

		c := s.Connect()

		// Get model
		subscribeToTestModel(t, s, c)

		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))

		// Validate a get request is sent
		s.GetRequest(t).AssertSubject(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate no events are sent to client
		c.AssertNoEvent(t, "test.model")
	})
}

// Test that a system.reset event triggers get requests on matching collection
func TestSystemResetTriggersGetRequestOnCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		collection := resourceData("test.collection")

		c := s.Connect()

		// Get collection
		subscribeToTestCollection(t, s, c)

		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))

		// Validate a get request is sent
		s.GetRequest(t).AssertSubject(t, "get.test.collection").RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		// Validate no events are sent to client
		c.AssertNoEvent(t, "test.collection")
	})
}

// Test that a system.reset event on modified model generates change event
func TestSystemResetGeneratesChangeEventOnModel(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()

		// Get model
		subscribeToTestModel(t, s, c)

		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))

		// Validate a get request is sent
		s.GetRequest(t).AssertSubject(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":{"string":"bar","int":42,"bool":true}}`))

		// Validate no events are sent to client
		c.GetEvent(t).AssertEventName(t, "test.model.change").AssertData(t, json.RawMessage(`{"values":{"string":"bar","null":{"action":"delete"}}}`))
	})
}

// Test that a system.reset event on modified collection generates add and remove events
func TestSystemResetGeneratesAddRemoveEventsOnCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()

		// Get collection
		subscribeToTestCollection(t, s, c)

		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))

		// Validate a get request is sent
		s.GetRequest(t).AssertSubject(t, "get.test.collection").RespondSuccess(json.RawMessage(`{"collection":[42,"new",true,null]}`))

		// Validate no events are sent to client
		c.GetEvent(t).AssertEventName(t, "test.collection.remove").AssertData(t, json.RawMessage(`{"idx":0}`))
		c.GetEvent(t).AssertEventName(t, "test.collection.add").AssertData(t, json.RawMessage(`{"idx":1,"value":"new"}`))
	})
}

// Test that a system.reset event triggers a re-access call on subscribed resources
// and that the resource are still subscribed after given access
func TestSystemAccessEventTriggersAccessCallOnSubscribedResources(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()

		// Get linked model
		subscribeToTestModelParent(t, s, c, false)

		// Get collection
		subscribeToTestCollection(t, s, c)

		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"access":["test.model.>"]}`))

		// Handle access requests with access denied
		s.GetRequest(t).AssertSubject(t, "access.test.model.parent").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate no unsubscribe events are sent to client
		c.AssertNoEvent(t, "test.model.parent")

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.custom", event)

		// Send event on model parent and validate client event
		s.ResourceEvent("test.model.parent", "custom", event)
		c.GetEvent(t).Equals(t, "test.model.parent.custom", event)

		// Send event on collection and validate client event
		s.ResourceEvent("test.collection", "custom", event)
		c.GetEvent(t).Equals(t, "test.collection.custom", event)
	})
}

// Test that a reaccess event triggers a re-access call on subscribed resources
// and that the resource are unsubscribed after being deniedn access
func TestSystemResetEventTriggersUnsubscribeOnDeniedAccessCall(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)
		reasonAccessDenied := json.RawMessage(`{"reason":{"code":"system.accessDenied","message":"Access denied"}}`)

		c := s.Connect()

		// Get linked model
		subscribeToTestModelParent(t, s, c, false)

		// Get collection
		subscribeToTestCollection(t, s, c)

		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"access":["test.model.>"]}`))

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

		// Send event on collection and validate client event
		s.ResourceEvent("test.collection", "custom", event)
		c.GetEvent(t).Equals(t, "test.collection.custom", event)
	})
}
