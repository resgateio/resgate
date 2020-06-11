package test

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Test change event on subscribed resource
func TestChangeEventOnSubscribedResource(t *testing.T) {
	runTest(t, func(s *Session) {

		c := s.Connect()
		subscribeToTestModel(t, s, c)

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
		c.GetEvent(t).Equals(t, "test.model.change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
	})
}

// Test that change events sent prior to a get response is discarded
func TestChangeEventPriorToGetResponseIsDiscarded(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")

		c := s.Connect()

		// Send subscribe request
		creq := c.Request("subscribe.test.model", nil)
		// Wait for get and access request
		mreqs := s.GetParallelRequests(t, 2)
		// Send change event
		s.ResourceEvent("test.model", "change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
		// Respond to get and access request
		mreqs.GetRequest(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		mreqs.GetRequest(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))
		// Validate client response and validate
		creq.GetResponse(t)

		// Send event on model and validate client event
		c.AssertNoEvent(t, "test.model")
	})
}

// Test change event effect on cached model
func TestChangeEventOnCachedModel(t *testing.T) {
	tbl := []struct {
		RID                 string // RID of resource to subscribe to
		ChangeEvent         string // Change event to send (raw JSON)
		ExpectedChangeEvent string // Expected event sent to client (raw JSON. Empty means none)
		ExpectedModel       string // Expected model after event (raw JSON)
		ExpectedErrors      int
	}{
		{"test.model", `{"values":{"string":"bar","int":-12}}`, `{"values":{"string":"bar","int":-12}}`, `{"string":"bar","int":-12,"bool":true,"null":null}`, 0},
		{"test.model", `{"values":{"string":"bar"}}`, `{"values":{"string":"bar"}}`, `{"string":"bar","int":42,"bool":true,"null":null}`, 0},
		{"test.model", `{"values":{"int":-12}}`, `{"values":{"int":-12}}`, `{"string":"foo","int":-12,"bool":true,"null":null}`, 0},
		{"test.model", `{"values":{"new":false}}`, `{"values":{"new":false}}`, `{"string":"foo","int":42,"bool":true,"null":null,"new":false}`, 0},
		{"test.model", `{"values":{"int":{"action":"delete"}}}`, `{"values":{"int":{"action":"delete"}}}`, `{"string":"foo","bool":true,"null":null}`, 0},
		{"test.model", `{"values":{"soft":{"rid":"test.model.soft","soft":true}}}`, `{"values":{"soft":{"rid":"test.model.soft","soft":true}}}`, `{"string":"foo","int":42,"bool":true,"null":null,"soft":{"rid":"test.model.soft","soft":true}}`, 0},
		{"test.model.soft", `{"values":{"child":null}}`, `{"values":{"child":null}}`, `{"name":"soft","child":null}`, 0},
		{"test.model.soft", `{"values":{"child":{"action":"delete"}}}`, `{"values":{"child":{"action":"delete"}}}`, `{"name":"soft"}`, 0},
		{"test.model.data", `{"values":{"primitive":{"data":13}}}`, `{"values":{"primitive":13}}`, `{"name":"data","primitive":13,"object":{"data":{"foo":["bar"]}},"array":{"data":[{"foo":"bar"}]}}`, 0},
		{"test.model.data", `{"values":{"object":{"data":{"foo":["baz"]}}}}`, `{"values":{"object":{"data":{"foo":["baz"]}}}}`, `{"name":"data","primitive":12,"object":{"data":{"foo":["baz"]}},"array":{"data":[{"foo":"bar"}]}}`, 0},
		{"test.model.data", `{"values":{"array":{"data":[{"foo":"baz"}]}}}`, `{"values":{"array":{"data":[{"foo":"baz"}]}}}`, `{"name":"data","primitive":12,"object":{"data":{"foo":["bar"]}},"array":{"data":[{"foo":"baz"}]}}`, 0},

		// Unchanged values
		{"test.model", `{"values":{}}`, "", `{"string":"foo","int":42,"bool":true,"null":null}`, 0},
		{"test.model", `{"values":{"string":"foo"}}`, "", `{"string":"foo","int":42,"bool":true,"null":null}`, 0},
		{"test.model", `{"values":{"string":"foo","int":42}}`, "", `{"string":"foo","int":42,"bool":true,"null":null}`, 0},
		{"test.model", `{"values":{"invalid":{"action":"delete"}}}`, "", `{"string":"foo","int":42,"bool":true,"null":null}`, 0},
		{"test.model", `{"values":{"null":null,"string":"bar"}}`, `{"values":{"string":"bar"}}`, `{"string":"bar","int":42,"bool":true,"null":null}`, 0},
		{"test.model.soft", `{"values":{"child":{"rid":"test.model","soft":true}}}`, "", `{"name":"soft","child":{"rid":"test.model","soft":true}}`, 0},
		{"test.model.data", `{"values":{}}`, "", `{"name":"data","primitive":12,"object":{"data":{"foo":["bar"]}},"array":{"data":[{"foo":"bar"}]}}`, 0},
		{"test.model.data", `{"values":{"primitive":12,"object":{"data":{"foo":["bar"]}},"array":{"data":[{"foo":"bar"}]}}}`, "", `{"name":"data","primitive":12,"object":{"data":{"foo":["bar"]}},"array":{"data":[{"foo":"bar"}]}}`, 0},
		{"test.model", `{"values":{"null":{"data":null}}}`, "", `{"string":"foo","int":42,"bool":true,"null":null}`, 0},

		// Model change event v1.0 legacy behavior
		{"test.model", `{"string":"bar","int":-12}`, `{"values":{"string":"bar","int":-12}}`, `{"string":"bar","int":-12,"bool":true,"null":null}`, 1},
		{"test.model", `{"string":"bar"}`, `{"values":{"string":"bar"}}`, `{"string":"bar","int":42,"bool":true,"null":null}`, 1},
	}

	for i, l := range tbl {
		for sameClient := true; sameClient; sameClient = false {
			runNamedTest(t, fmt.Sprintf("#%d with the same client being %+v", i+1, sameClient), func(s *Session) {
				var creq *ClientRequest

				c := s.Connect()
				subscribeToResource(t, s, c, l.RID)

				// Send event on model and validate client event
				s.ResourceEvent(l.RID, "change", json.RawMessage(l.ChangeEvent))
				if l.ExpectedChangeEvent == "" {
					c.AssertNoEvent(t, l.RID+".change")
				} else {
					c.GetEvent(t).Equals(t, l.RID+".change", json.RawMessage(l.ExpectedChangeEvent))
				}

				if sameClient {
					c.Request("unsubscribe."+l.RID, nil).GetResponse(t)
					// Subscribe a second time
					creq = c.Request("subscribe."+l.RID, nil)
				} else {
					c2 := s.Connect()
					// Subscribe a second time
					creq = c2.Request("subscribe."+l.RID, nil)
				}

				// Handle model access request
				s.GetRequest(t).AssertSubject(t, "access."+l.RID).RespondSuccess(json.RawMessage(`{"get":true}`))

				// Validate client response
				creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"`+l.RID+`":`+l.ExpectedModel+`}}`))
				s.AssertErrorsLogged(t, l.ExpectedErrors)
			})
		}
	}
}

// Test change event with new resource reference
func TestChangeEventWithNewResourceReference(t *testing.T) {
	collection := resourceData("test.collection")

	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModel(t, s, c)

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "change", json.RawMessage(`{"values":{"ref":{"rid":"test.collection"}}}`))

		// Handle collection get request
		s.
			GetRequest(t).
			AssertSubject(t, "get.test.collection").
			RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		c.GetEvent(t).Equals(t, "test.model.change", json.RawMessage(`{"values":{"ref":{"rid":"test.collection"}},"collections":{"test.collection":`+collection+`}}`))

		// Send event on collection and validate client event
		s.ResourceEvent("test.collection", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.collection.custom", common.CustomEvent())
	})
}

// Test change event with removed resource reference
func TestChangeEventWithRemovedResourceReference(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModelParent(t, s, c, false)

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.model.custom", common.CustomEvent())

		// Send event on model and validate client event
		s.ResourceEvent("test.model.parent", "change", json.RawMessage(`{"values":{"child":null}}`))
		c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":null}}`))

		// Send event on collection and validate client event is not sent to client
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.AssertNoEvent(t, "test.model")
	})
}

// Test change event with new resource reference
func TestChangeEventWithChangedResourceReference(t *testing.T) {
	collection := resourceData("test.collection")

	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModelParent(t, s, c, false)

		// Send change event on model parent
		s.ResourceEvent("test.model.parent", "change", json.RawMessage(`{"values":{"child":{"rid":"test.collection"}}}`))

		// Handle collection get request
		s.
			GetRequest(t).
			AssertSubject(t, "get.test.collection").
			RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":{"rid":"test.collection"}},"collections":{"test.collection":`+collection+`}}`))

		// Send event on collection and validate client event
		s.ResourceEvent("test.collection", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.collection.custom", common.CustomEvent())

		// Send event on model and validate no event is sent to client
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.AssertNoEvent(t, "test.model")
	})
}

// Test change event with removed resource reference
func TestChangeEvent_WithResourceReferenceReplacedBySoftReference_UnsubscribesReference(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModelParent(t, s, c, false)

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.model.custom", common.CustomEvent())

		// Send event on model and validate client event
		s.ResourceEvent("test.model.parent", "change", json.RawMessage(`{"values":{"child":{"rid":"test.model","soft":true}}}`))
		c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":{"rid":"test.model","soft":true}}}`))

		// Send event on collection and validate client event is not sent to client
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.AssertNoEvent(t, "test.model")
	})
}

// Test change event with new resource reference
func TestChangeEvent_WithSoftReferenceReplacedByResourceReference_SubscribesReference(t *testing.T) {
	model := resourceData("test.model")

	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToResource(t, s, c, "test.model.soft")

		// Send event on model and validate client event
		s.ResourceEvent("test.model.soft", "change", json.RawMessage(`{"values":{"child":{"rid":"test.model","soft":false}}}`))

		// Handle model get request
		s.
			GetRequest(t).
			AssertSubject(t, "get.test.model").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		c.GetEvent(t).Equals(t, "test.model.soft.change", json.RawMessage(`{"values":{"child":{"rid":"test.model","soft":false}},"models":{"test.model":`+model+`}}`))

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.model.custom", common.CustomEvent())
	})
}
