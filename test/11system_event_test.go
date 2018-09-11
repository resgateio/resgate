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
		model := resource["test.model"]

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
		collection := resource["test.collection"]

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
