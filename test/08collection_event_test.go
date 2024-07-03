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
		RID                string // Resource ID
		EventName          string // Name of the event. Either add or remove.
		EventPayload       string // Event payload (raw JSON)
		ExpectedCollection string // Expected collection after event (raw JSON)
		ExpectedEvent      string // Expected event payload (empty means same as EventPayload)
	}{
		{"test.collection", "add", `{"idx":0,"value":"bar"}`, `["bar","foo",42,true,null]`, ""},
		{"test.collection", "add", `{"idx":1,"value":"bar"}`, `["foo","bar",42,true,null]`, ""},
		{"test.collection", "add", `{"idx":4,"value":"bar"}`, `["foo",42,true,null,"bar"]`, ""},
		{"test.collection", "add", `{"idx":0,"value":{"rid":"test.collection.soft","soft":true}}`, `[{"rid":"test.collection.soft","soft":true},"foo",42,true,null]`, ""},
		{"test.collection", "add", `{"idx":0,"value":{"data":{"foo":["bar"]}}}`, `[{"data":{"foo":["bar"]}},"foo",42,true,null]`, ""},
		{"test.collection", "add", `{"idx":0,"value":{"data":12}}`, `[12,"foo",42,true,null]`, `{"idx":0,"value":12}`},
		{"test.collection", "remove", `{"idx":0}`, `[42,true,null]`, ""},
		{"test.collection", "remove", `{"idx":1}`, `["foo",true,null]`, ""},
		{"test.collection", "remove", `{"idx":3}`, `["foo",42,true]`, ""},
		{"test.collection.soft", "remove", `{"idx":1}`, `["soft"]`, ""},
		{"test.collection.data", "remove", `{"idx":1}`, `["data",{"data":{"foo":["bar"]}},{"data":[{"foo":"bar"}]}]`, ""},
		{"test.collection.data", "remove", `{"idx":2}`, `["data",12,{"data":[{"foo":"bar"}]}]`, ""},
		{"test.collection.data", "remove", `{"idx":3}`, `["data",12,{"data":{"foo":["bar"]}}]`, ""},
	}

	for i, l := range tbl {
		for sameClient := true; sameClient; sameClient = false {
			runNamedTest(t, fmt.Sprintf("#%d with the same client being %+v", i+1, sameClient), func(s *Session) {
				var creq *ClientRequest

				c := s.Connect()
				subscribeToResource(t, s, c, l.RID)

				// Send event on collection and validate client event
				s.ResourceEvent(l.RID, l.EventName, json.RawMessage(l.EventPayload))
				expectedEvent := l.ExpectedEvent
				if expectedEvent == "" {
					expectedEvent = l.EventPayload
				}
				c.GetEvent(t).Equals(t, l.RID+"."+l.EventName, json.RawMessage(expectedEvent))

				if sameClient {
					c.Request("unsubscribe."+l.RID, nil).GetResponse(t)
					// Subscribe a second time
					creq = c.Request("subscribe."+l.RID, nil)
				} else {
					c2 := s.Connect()
					// Subscribe a second time
					creq = c2.Request("subscribe."+l.RID, nil)
				}

				// Handle collection access request
				s.GetRequest(t).AssertSubject(t, "access."+l.RID).RespondSuccess(json.RawMessage(`{"get":true}`))

				// Validate client response
				creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{"`+l.RID+`":`+l.ExpectedCollection+`}}`))
			})
		}
	}
}

// Test add event with resource reference
func TestCollectionEvent_AddEventWithResourceReference_IncludesResource(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")

		c := s.Connect()
		subscribeToTestCollection(t, s, c)

		// Send add event on collection
		s.ResourceEvent("test.collection", "add", json.RawMessage(`{"idx":3,"value":{"rid":"test.model"}}`))

		// Handle model get request
		s.GetRequest(t).AssertSubject(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate client event
		c.GetEvent(t).Equals(t, "test.collection.add", json.RawMessage(`{"idx":3,"value":{"rid":"test.model"},"models":{"test.model":`+model+`}}`))
	})
}

// Test sequential add and remove events with resource reference.
func TestCollectionEvent_SequentialAddRemoveEventsWithResourceReference_ExpectedEvent(t *testing.T) {
	tbl := [][]struct {
		Add bool
		Idx int
	}{
		{{true, 0}},
		{{true, 0}, {false, 0}},
		{{true, 0}, {false, 0}, {true, 0}},
		{{true, 0}, {true, 0}},
		{{true, 0}, {true, 1}},
		{{true, 0}, {true, 1}, {false, 0}, {false, 0}},
		{{true, 0}, {true, 1}, {false, 1}, {false, 0}, {true, 0}},
	}
	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			model := resourceData("test.model")

			c := s.Connect()
			subscribeToResource(t, s, c, "test.collection.empty")

			count := 0
			modelCached := false
			for _, step := range l {
				if step.Add {
					// Send add event on collection
					s.ResourceEvent("test.collection.empty", "add", json.RawMessage(fmt.Sprintf(`{"idx":%d,"value":{"rid":"test.model"}}`, step.Idx)))
					// First time model it added, we expect a get request
					if !modelCached {
						// Handle model get request
						s.GetRequest(t).AssertSubject(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
						modelCached = true
					}

					// Validate client event
					if count == 0 {
						// When the model is not indirectly subscribed
						c.GetEvent(t).Equals(t, "test.collection.empty.add", json.RawMessage(fmt.Sprintf(`{"idx":%d,"value":{"rid":"test.model"},"models":{"test.model":%s}}`, step.Idx, model)))
					} else {
						// Then the model already is indirectly subscribed
						c.GetEvent(t).Equals(t, "test.collection.empty.add", json.RawMessage(fmt.Sprintf(`{"idx":%d,"value":{"rid":"test.model"}}`, step.Idx)))
					}
					count++
				} else {
					// Send remove event on collection
					s.ResourceEvent("test.collection.empty", "remove", json.RawMessage(fmt.Sprintf(`{"idx":%d}`, step.Idx)))

					// Validate client event
					c.GetEvent(t).Equals(t, "test.collection.empty.remove", json.RawMessage(fmt.Sprintf(`{"idx":%d}`, step.Idx)))
					count--
				}
			}
		})
	}
}
