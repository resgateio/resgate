package test

import (
	"encoding/json"
	"testing"
)

// Test change event on subscribed resource
func TestChangeEventOnSubscribedResource(t *testing.T) {
	model := resource["test.model"]

	runTest(t, func(s *Session) {

		c := s.Connect()
		creq := c.Request("subscribe.test.model", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.model")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.model")
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))

		// Send event on model and validate client event
		s.Event("test.model", "change", json.RawMessage(`{"string":"bar","int":-12}`))
		c.GetEvent(t).Equals(t, "test.model.change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
	})
}

// Test change event effect on cached model
func TestChangeEventOnCachedModel(t *testing.T) {
	model := `{"string":"foo","int":42}`

	tbl := []struct {
		ChangeEvent   string // Change event to send (raw JSON)
		ExpectedModel string // Expected model after event (raw JSON)
	}{
		{`{"string":"bar","int":-12}`, `{"string":"bar","int":-12}`},
		{`{"string":"bar"}`, `{"string":"bar","int":42}`},
		{`{"int":-12}`, `{"string":"foo","int":-12}`},
		{`{}`, `{"string":"foo","int":42}`},
		{`{"bool":true}`, `{"string":"foo","int":42,"bool":true}`},
		{`{"int":{"action":"delete"}}`, `{"string":"foo"}`},
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
				creq := c.Request("subscribe.test.model", nil)

				// Handle model get and access request
				mreqs := s.GetParallelRequests(t, 2)
				req := mreqs.GetRequest(t, "access.test.model")
				req.RespondSuccess(json.RawMessage(`{"get":true}`))
				req = mreqs.GetRequest(t, "get.test.model")
				req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

				// Validate client response
				cresp := creq.GetResponse(t)
				cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))

				// Send event on model and validate client event
				s.Event("test.model", "change", json.RawMessage(l.ChangeEvent))
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
				req = s.GetRequest(t).AssertSubject(t, "access.test.model")
				req.RespondSuccess(json.RawMessage(`{"get":true}`))

				// Validate client response
				cresp = creq.GetResponse(t)
				cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model":`+l.ExpectedModel+`}}`))

				panicked = false
			})
		}
	}
}

// Test change event with new resource reference
func TestChangeEventWithNewResourceReference(t *testing.T) {
	model := resource["test.model"]
	collection := resource["test.collection"]
	customEvent := json.RawMessage(`{"foo":"bar"}`)

	runTest(t, func(s *Session) {

		c := s.Connect()
		creq := c.Request("subscribe.test.model", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.model")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.model")
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))

		// Send event on model and validate client event
		s.Event("test.model", "change", json.RawMessage(`{"ref":{"rid":"test.collection"}}`))

		// Handle collection get request
		s.
			GetRequest(t).
			AssertSubject(t, "get.test.collection").
			RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		c.GetEvent(t).Equals(t, "test.model.change", json.RawMessage(`{"values":{"ref":{"rid":"test.collection"}},"collections":{"test.collection":`+collection+`}}`))

		// Send event on collection and validate client event
		s.Event("test.collection", "custom", customEvent)
		c.GetEvent(t).Equals(t, "test.collection.custom", customEvent)
	})
}

// Test change event with removed resource reference
func TestChangeEventWithRemovedResourceReference(t *testing.T) {
	model := resource["test.model"]
	modelParent := resource["test.model.parent"]
	customEvent := json.RawMessage(`{"foo":"bar"}`)

	runTest(t, func(s *Session) {
		c := s.Connect()
		creq := c.Request("subscribe.test.model.parent", nil)

		// Handle parent get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "get.test.model.parent")
		req.RespondSuccess(json.RawMessage(`{"model":` + modelParent + `}`))
		req = mreqs.GetRequest(t, "access.test.model.parent")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))

		// Handle child get request
		mreqs = s.GetParallelRequests(t, 1)
		req = mreqs.GetRequest(t, "get.test.model")
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Get client response
		cresp := creq.GetResponse(t)
		cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`,"test.model.parent":`+modelParent+`}}`))

		// Send event on model and validate client event
		s.Event("test.model", "custom", customEvent)
		c.GetEvent(t).Equals(t, "test.model.custom", customEvent)

		// Send event on model and validate client event
		s.Event("test.model.parent", "change", json.RawMessage(`{"child":null}`))
		c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":null}}`))

		// Send event on collection and validate client event is not sent to client
		s.Event("test.model", "custom", customEvent)
		c.AssertNoEvent(t, "test.model")
	})
}
