package test

import (
	"encoding/json"
	"testing"
)

// Test change event on subscribed resource
func TestAddAndRemoveEventOnSubscribedResource(t *testing.T) {
	collection := resource["test.collection"]

	runTest(t, func(s *Session) {

		c := s.Connect()
		creq := c.Request("subscribe.test.collection", nil)

		// Handle collection get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.collection")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.collection")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+collection+`}}`))

		// Send add event on collection and validate client event
		s.Event("test.collection", "add", json.RawMessage(`{"idx":3,"value":"bar"}`))
		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":3,"value":"bar"}`))

		// Send remove event on collection and validate client event
		s.Event("test.collection", "remove", json.RawMessage(`{"idx":2}`))
		c.GetEvent(t).Equals(t, "test.collection.remove", json.RawMessage(`{"idx":2}`))
	})
}

// Test add and remove event effects on cached collection
func TestAddRemoveEventsOnCachedCollection(t *testing.T) {
	collection := `["foo",42,true,null]`

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
			runTest(t, func(s *Session) {
				panicked := true
				defer func() {
					if panicked {
						t.Logf("Error in test %d with same client being %+v", i, sameClient)
					}
				}()

				c := s.Connect()
				creq := c.Request("subscribe.test.collection", nil)

				// Handle collection get and access request
				mreqs := s.GetParallelRequests(t, 2)
				req := mreqs.GetRequest(t, "access.test.collection")
				req.RespondSuccess(json.RawMessage(`{"get":true}`))
				req = mreqs.GetRequest(t, "get.test.collection")
				req.RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

				// Validate client response
				cresp := creq.GetResponse(t)
				cresp.AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+collection+`}}`))

				// Send event on collection and validate client event
				s.Event("test.collection", l.EventName, json.RawMessage(l.EventPayload))
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
				req = s.GetRequest(t).AssertSubject(t, "access.test.collection")
				req.RespondSuccess(json.RawMessage(`{"get":true}`))

				// Validate client response
				cresp = creq.GetResponse(t)
				cresp.AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+l.ExpectedCollection+`}}`))

				panicked = false
			})
		}
	}
}

// Test change event with new resource reference
func TestAddEventWithNewResourceReference(t *testing.T) {
	model := resource["test.model"]
	collection := resource["test.collection"]
	customEvent := json.RawMessage(`{"foo":"bar"}`)

	runTest(t, func(s *Session) {

		c := s.Connect()
		creq := c.Request("subscribe.test.collection", nil)

		// Handle collection get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.collection")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.collection")
		req.RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+collection+`}}`))

		// Send event on collection and validate client event
		s.Event("test.collection", "add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model"}}`))

		// Handle collection get request
		s.
			GetRequest(t).
			AssertSubject(t, "get.test.model").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model"},"models":{"test.model":`+model+`}}`))

		// Send event on model and validate client event
		s.Event("test.model", "custom", customEvent)
		c.GetEvent(t).Equals(t, "test.model.custom", customEvent)
	})
}

// Test change event with removed resource reference
func TestRemoveEventWithRemovedResourceReference(t *testing.T) {
	collection := resource["test.collection"]
	collectionParent := resource["test.collection.parent"]
	customEvent := json.RawMessage(`{"foo":"bar"}`)

	runTest(t, func(s *Session) {
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
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"collections":{"test.collection":`+collection+`,"test.collection.parent":`+collectionParent+`}}`))

		// Send event on collection and validate client event
		s.Event("test.collection", "custom", customEvent)
		c.GetEvent(t).Equals(t, "test.collection.custom", customEvent)

		// Send event on collection and validate client event
		s.Event("test.collection.parent", "remove", json.RawMessage(`{"idx":1}`))
		c.GetEvent(t).Equals(t, "test.collection.parent.remove", json.RawMessage(`{"idx":1}`))

		// Send event on collection and validate client event is not sent to client
		s.Event("test.collection", "custom", customEvent)
		c.AssertNoEvent(t, "test.collection")
	})
}
