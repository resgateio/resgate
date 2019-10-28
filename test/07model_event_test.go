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
		ChangeEvent         string // Change event to send (raw JSON)
		ExpectedChangeEvent string // Expected event sent to client (raw JSON. Empty means none)
		ExpectedModel       string // Expected model after event (raw JSON)
		ExpectedErrors      int
	}{
		{`{"values":{"string":"bar","int":-12}}`, `{"values":{"string":"bar","int":-12}}`, `{"string":"bar","int":-12,"bool":true,"null":null}`, 0},
		{`{"values":{"string":"bar"}}`, `{"values":{"string":"bar"}}`, `{"string":"bar","int":42,"bool":true,"null":null}`, 0},
		{`{"values":{"int":-12}}`, `{"values":{"int":-12}}`, `{"string":"foo","int":-12,"bool":true,"null":null}`, 0},
		{`{"values":{"new":false}}`, `{"values":{"new":false}}`, `{"string":"foo","int":42,"bool":true,"null":null,"new":false}`, 0},
		{`{"values":{"int":{"action":"delete"}}}`, `{"values":{"int":{"action":"delete"}}}`, `{"string":"foo","bool":true,"null":null}`, 0},

		// Unchanged values
		{`{"values":{}}`, "", `{"string":"foo","int":42,"bool":true,"null":null}`, 0},
		{`{"values":{"string":"foo"}}`, "", `{"string":"foo","int":42,"bool":true,"null":null}`, 0},
		{`{"values":{"string":"foo","int":42}}`, "", `{"string":"foo","int":42,"bool":true,"null":null}`, 0},
		{`{"values":{"invalid":{"action":"delete"}}}`, "", `{"string":"foo","int":42,"bool":true,"null":null}`, 0},
		{`{"values":{"null":null,"string":"bar"}}`, `{"values":{"string":"bar"}}`, `{"string":"bar","int":42,"bool":true,"null":null}`, 0},

		// Model change event v1.0 legacy behavior
		{`{"string":"bar","int":-12}`, `{"values":{"string":"bar","int":-12}}`, `{"string":"bar","int":-12,"bool":true,"null":null}`, 1},
		{`{"string":"bar"}`, `{"values":{"string":"bar"}}`, `{"string":"bar","int":42,"bool":true,"null":null}`, 1},
	}

	for i, l := range tbl {
		for sameClient := true; sameClient; sameClient = false {
			runNamedTest(t, fmt.Sprintf("#%d with the same client being %+v", i+1, sameClient), func(s *Session) {
				var creq *ClientRequest

				c := s.Connect()
				subscribeToTestModel(t, s, c)

				// Send event on model and validate client event
				s.ResourceEvent("test.model", "change", json.RawMessage(l.ChangeEvent))
				if l.ExpectedChangeEvent == "" {
					c.AssertNoEvent(t, "test.model.change")
				} else {
					c.GetEvent(t).Equals(t, "test.model.change", json.RawMessage(l.ExpectedChangeEvent))
				}

				if sameClient {
					c.Request("unsubscribe.test.model", nil).GetResponse(t)
					// Subscribe a second time
					creq = c.Request("subscribe.test.model", nil)
				} else {
					c2 := s.Connect()
					// Subscribe a second time
					creq = c2.Request("subscribe.test.model", nil)
				}

				// Handle model access request
				s.GetRequest(t).AssertSubject(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))

				// Validate client response
				creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+l.ExpectedModel+`}}`))
				s.AssertErrorsLogged(t, l.ExpectedErrors)
			})
		}
	}
}

// Test change event with new resource reference
func TestChangeEventWithNewResourceReference(t *testing.T) {
	collection := resourceData("test.collection")
	customEvent := json.RawMessage(`{"foo":"bar"}`)

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
		s.ResourceEvent("test.collection", "custom", customEvent)
		c.GetEvent(t).Equals(t, "test.collection.custom", customEvent)
	})
}

// Test change event with removed resource reference
func TestChangeEventWithRemovedResourceReference(t *testing.T) {
	customEvent := json.RawMessage(`{"foo":"bar"}`)

	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModelParent(t, s, c, false)

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", customEvent)
		c.GetEvent(t).Equals(t, "test.model.custom", customEvent)

		// Send event on model and validate client event
		s.ResourceEvent("test.model.parent", "change", json.RawMessage(`{"values":{"child":null}}`))
		c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":null}}`))

		// Send event on collection and validate client event is not sent to client
		s.ResourceEvent("test.model", "custom", customEvent)
		c.AssertNoEvent(t, "test.model")
	})
}

// Test change event with new resource reference
func TestChangeEventWithChangedResourceReference(t *testing.T) {
	collection := resourceData("test.collection")
	customEvent := json.RawMessage(`{"foo":"bar"}`)

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
		s.ResourceEvent("test.collection", "custom", customEvent)
		c.GetEvent(t).Equals(t, "test.collection.custom", customEvent)

		// Send event on model and validate no event is sent to client
		s.ResourceEvent("test.model", "custom", customEvent)
		c.AssertNoEvent(t, "test.model")
	})
}
