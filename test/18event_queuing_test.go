package test

import (
	"encoding/json"
	"testing"
)

// Test event while queuing
func TestEventWhileQueueing(t *testing.T) {
	model := resourceData("test.model")

	runTest(t, func(s *Session) {

		c := s.Connect()
		subscribeToTestCollection(t, s, c)

		// Send event on collection and validate client event
		s.ResourceEvent("test.collection", "add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model"}}`))

		// Handle collection get request
		req := s.
			GetRequest(t).
			AssertSubject(t, "get.test.model")

		// Send additional add event on collection
		s.ResourceEvent("test.collection", "add", json.RawMessage(`{"idx":2,"value":"newValue"}`))
		c.AssertNoEvent(t, "test.collection")

		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model"},"models":{"test.model":`+model+`}}`))
		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":2,"value":"newValue"}`))
	})
}

// Test event with added reference while queuing
func TestReferenceEventWhileQueuing(t *testing.T) {
	model := resourceData("test.model")

	runTest(t, func(s *Session) {

		c := s.Connect()
		subscribeToTestCollection(t, s, c)

		// Send event on collection and let it queue
		s.ResourceEvent("test.collection", "add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model"}}`))

		// Get model get request
		req1 := s.
			GetRequest(t).
			AssertSubject(t, "get.test.model")

		// Send two additional add events on collection
		s.ResourceEvent("test.collection", "add", json.RawMessage(`{"idx":2,"value":{"rid":"test.model2"}}`))
		s.ResourceEvent("test.collection", "add", json.RawMessage(`{"idx":3,"value":"newValue"}`))

		// Respond to model get request
		req1.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model"},"models":{"test.model":`+model+`}}`))

		// Get second model get request
		req2 := s.
			GetRequest(t).
			AssertSubject(t, "get.test.model2")

		// Assert no more events are send
		c.AssertNoEvent(t, "test.collection")

		// Respond to second model get request
		req2.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":2,"value":{"rid":"test.model2"},"models":{"test.model2":`+model+`}}`))
		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":3,"value":"newValue"}`))
	})
}

// Test event with cached reference while queuing
func TestCachedReferenceEventWhileQueuing(t *testing.T) {
	model := resourceData("test.model")
	collection := resourceData("test.collection")

	runTest(t, func(s *Session) {

		c := s.Connect()
		subscribeToTestCollectionParent(t, s, c, false)

		// Send remove event on collection and validate client event
		s.ResourceEvent("test.collection.parent", "remove", json.RawMessage(`{"idx":1}`))
		c.GetEvent(t).Equals(t, "test.collection.parent.remove", json.RawMessage(`{"idx":1}`))

		// Send event on collection and let it queue
		s.ResourceEvent("test.collection.parent", "add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model"}}`))
		// Get model get request
		req := s.
			GetRequest(t).
			AssertSubject(t, "get.test.model")

		// Send additional add event with cached reference
		s.ResourceEvent("test.collection.parent", "add", json.RawMessage(`{"idx":2,"value":{"rid":"test.collection"}}`))

		// Respond to model get request
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		c.GetEvent(t).Equals(t, "test.collection.parent.add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model"},"models":{"test.model":`+model+`}}`))
		c.GetEvent(t).Equals(t, "test.collection.parent.add", json.RawMessage(`{"idx":2,"value":{"rid":"test.collection"},"collections":{"test.collection":`+collection+`}}`))
	})
}

// Test event with cached reference while queuing
func TestUnqueueEventWithLoadedReferenceResource(t *testing.T) {
	model := resourceData("test.model")

	runTest(t, func(s *Session) {

		c := s.Connect()
		subscribeToTestModel(t, s, c)
		subscribeToTestCollection(t, s, c)

		// Send event on model with three references, and load only one
		s.ResourceEvent("test.model", "change", json.RawMessage(`{"values":{"a":{"rid":"test.model.a"},"b":{"rid":"test.model.b"},"c":{"rid":"test.model.c"}}}`))
		mreq := s.GetParallelRequests(t, 3)
		mreq.GetRequest(t, "get.test.model.a").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Send multiple events on the same resource
		s.ResourceEvent("test.collection", "add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model.b"}}`))
		s.ResourceEvent("test.collection", "add", json.RawMessage(`{"idx":2,"value":{"rid":"test.model.a"}}`))

		// Make sure test.model.a is loaded by making a roundtrip to the service
		c.AssertNoEvent(t, "test.collection")

		// Respond to the
		mreq.GetRequest(t, "get.test.model.b").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Respond to model get request
		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model.b"},"models":{"test.model.b":`+model+`}}`))
		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":2,"value":{"rid":"test.model.a"},"models":{"test.model.a":`+model+`}}`))

		// Respond to the last resource needed for the change event
		mreq.GetRequest(t, "get.test.model.c").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		c.GetEvent(t).Equals(t, "test.model.change", json.RawMessage(`{"values":{"a":{"rid":"test.model.a"},"b":{"rid":"test.model.b"},"c":{"rid":"test.model.c"}},"models":{"test.model.c":`+model+`}}`))
	})
}
