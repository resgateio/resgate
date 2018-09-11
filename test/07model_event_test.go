package test

import (
	"encoding/json"
	"testing"
)

// Test change event on subscribed resource
func TestChangeEventOnSubscribedResource(t *testing.T) {
	runTest(t, func(s *Session) {

		c := s.Connect()
		subscribeToTestModel(t, s, c)

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "change", json.RawMessage(`{"string":"bar","int":-12}`))
		c.GetEvent(t).Equals(t, "test.model.change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
	})
}

// Test change event effect on cached model
func TestChangeEventOnCachedModel(t *testing.T) {
	tbl := []struct {
		ChangeEvent   string // Change event to send (raw JSON)
		ExpectedModel string // Expected model after event (raw JSON)
	}{
		{`{"string":"bar","int":-12}`, `{"string":"bar","int":-12,"bool":true,"null":null}`},
		{`{"string":"bar"}`, `{"string":"bar","int":42,"bool":true,"null":null}`},
		{`{"int":-12}`, `{"string":"foo","int":-12,"bool":true,"null":null}`},
		{`{}`, `{"string":"foo","int":42,"bool":true,"null":null}`},
		{`{"new":false}`, `{"string":"foo","int":42,"bool":true,"null":null,"new":false}`},
		{`{"int":{"action":"delete"}}`, `{"string":"foo","bool":true,"null":null}`},
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

				var creq *ClientRequest

				c := s.Connect()
				subscribeToTestModel(t, s, c)

				// Send event on model and validate client event
				s.ResourceEvent("test.model", "change", json.RawMessage(l.ChangeEvent))
				c.GetEvent(t).Equals(t, "test.model.change", json.RawMessage(`{"values":`+l.ChangeEvent+`}`))

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

				panicked = false
			})
		}
	}
}

// Test change event with new resource reference
func TestChangeEventWithNewResourceReference(t *testing.T) {
	collection := resource["test.collection"]
	customEvent := json.RawMessage(`{"foo":"bar"}`)

	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModel(t, s, c)

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "change", json.RawMessage(`{"ref":{"rid":"test.collection"}}`))

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
		s.ResourceEvent("test.model.parent", "change", json.RawMessage(`{"child":null}`))
		c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":null}}`))

		// Send event on collection and validate client event is not sent to client
		s.ResourceEvent("test.model", "custom", customEvent)
		c.AssertNoEvent(t, "test.model")
	})
}
