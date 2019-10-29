package test

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Test add and remove events on subscribed resource
func TestAddAndRemoveEventOnSubscribedResource(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestCollection(t, s, c)

		// Send add event on collection and validate client event
		s.ResourceEvent("test.collection", "add", json.RawMessage(`{"idx":3,"value":"bar"}`))
		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":3,"value":"bar"}`))

		// Send remove event on collection and validate client event
		s.ResourceEvent("test.collection", "remove", json.RawMessage(`{"idx":2}`))
		c.GetEvent(t).Equals(t, "test.collection.remove", json.RawMessage(`{"idx":2}`))
	})
}

// Test add and remove event effects on cached collection
func TestAddRemoveEventsOnCachedCollection(t *testing.T) {
	tbl := []struct {
		EventName          string // Name of the event. Either add or remove.
		EventPayload       string // Event payload (raw JSON)
		ExpectedCollection string // Expected collection after event (raw JSON)
	}{
		{"add", `{"idx":0,"value":"bar"}`, `["bar","foo",42,true,null]`},
		{"add", `{"idx":1,"value":"bar"}`, `["foo","bar",42,true,null]`},
		{"add", `{"idx":4,"value":"bar"}`, `["foo",42,true,null,"bar"]`},
		{"remove", `{"idx":0}`, `[42,true,null]`},
		{"remove", `{"idx":1}`, `["foo",true,null]`},
		{"remove", `{"idx":3}`, `["foo",42,true]`},
	}

	for i, l := range tbl {
		for sameClient := true; sameClient; sameClient = false {
			runNamedTest(t, fmt.Sprintf("#%d with the same client being %+v", i+1, sameClient), func(s *Session) {
				var creq *ClientRequest

				c := s.Connect()
				subscribeToTestCollection(t, s, c)

				// Send event on collection and validate client event
				s.ResourceEvent("test.collection", l.EventName, json.RawMessage(l.EventPayload))
				c.GetEvent(t).Equals(t, "test.collection."+l.EventName, json.RawMessage(l.EventPayload))

				if sameClient {
					c.Request("unsubscribe.test.collection", nil).GetResponse(t)
					// Subscribe a second time
					creq = c.Request("subscribe.test.collection", nil)
				} else {
					c2 := s.Connect()
					// Subscribe a second time
					creq = c2.Request("subscribe.test.collection", nil)
				}

				// Handle collection access request
				s.GetRequest(t).AssertSubject(t, "access.test.collection").RespondSuccess(json.RawMessage(`{"get":true}`))

				// Validate client response
				creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+l.ExpectedCollection+`}}`))
			})
		}
	}
}

// Test add event with new resource reference
func TestAddEventWithNewResourceReference(t *testing.T) {
	model := resourceData("test.model")

	runTest(t, func(s *Session) {

		c := s.Connect()
		subscribeToTestCollection(t, s, c)

		// Send event on collection and validate client event
		s.ResourceEvent("test.collection", "add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model"}}`))

		// Handle collection get request
		s.
			GetRequest(t).
			AssertSubject(t, "get.test.model").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model"},"models":{"test.model":`+model+`}}`))

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.model.custom", common.CustomEvent())
	})
}

// Test remove event with removed resource reference
func TestRemoveEventWithRemovedResourceReference(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestCollectionParent(t, s, c, false)

		// Send event on collection and validate client event
		s.ResourceEvent("test.collection", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.collection.custom", common.CustomEvent())

		// Send event on collection and validate client event
		s.ResourceEvent("test.collection.parent", "remove", json.RawMessage(`{"idx":1}`))
		c.GetEvent(t).Equals(t, "test.collection.parent.remove", json.RawMessage(`{"idx":1}`))

		// Send event on collection and validate client event is not sent to client
		s.ResourceEvent("test.collection", "custom", common.CustomEvent())
		c.AssertNoEvent(t, "test.collection")
	})
}
