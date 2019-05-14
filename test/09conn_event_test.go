package test

import (
	"encoding/json"
	"testing"

	"github.com/resgateio/resgate/server/reserr"
)

// Test connection event
func TestConnectionEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		token := `{"user":"foo"}`

		c := s.Connect()

		cid := getCID(t, s, c)

		// Send token event
		s.ConnEvent(cid, "token", json.RawMessage(`{"token":`+token+`}`))
	})
}

// Test token is sent on access call
func TestTokenOnAccessCall(t *testing.T) {
	token := `{"user":"foo"}`
	model := resourceData("test.model")

	runTest(t, func(s *Session) {
		c := s.Connect()

		cid := getCID(t, s, c)

		// Send token event
		s.ConnEvent(cid, "token", json.RawMessage(`{"token":`+token+`}`))

		// Subscribe to model
		creq := c.Request("subscribe.test.model", nil)
		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model").
			AssertPathPayload(t, "token", json.RawMessage(token)).
			RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.GetRequest(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		// Validate client response
		creq.GetResponse(t)

		// Call to another resource
		creq = c.Request("call.test.callmodel.method", nil)
		s.GetRequest(t).
			AssertSubject(t, "access.test.callmodel").
			AssertPathPayload(t, "token", json.RawMessage(token)).
			RespondSuccess(json.RawMessage(`{"get":false}`))
		creq.GetResponse(t)
	})
}

// Test last token is sent on access call
func TestLastTokenOnAccessCall(t *testing.T) {
	token1 := `{"user":"foo","token":1}`
	token2 := `{"user":"bar","token":2}`

	runTest(t, func(s *Session) {
		c := s.Connect()

		cid := getCID(t, s, c)

		// Send token event
		s.ConnEvent(cid, "token", json.RawMessage(`{"token":`+token1+`}`))

		// Call to resource
		creq := c.Request("call.test.model1.method", nil)
		s.
			GetRequest(t).
			AssertSubject(t, "access.test.model1").
			AssertPathPayload(t, "token", json.RawMessage(token1)).
			RespondSuccess(json.RawMessage(`{"get":false}`))
		creq.GetResponse(t)

		// Send token event
		s.ConnEvent(cid, "token", json.RawMessage(`{"token":`+token2+`}`))

		// Call to resource
		creq = c.Request("call.test.model2.method", nil)
		s.
			GetRequest(t).
			AssertSubject(t, "access.test.model2").
			AssertPathPayload(t, "token", json.RawMessage(token2)).
			RespondSuccess(json.RawMessage(`{"get":false}`))
		creq.GetResponse(t)

		// Send token event
		s.ConnEvent(cid, "token", nil)

		// Call to resource
		creq = c.Request("call.test.model3.method", nil)
		s.
			GetRequest(t).
			AssertSubject(t, "access.test.model3").
			AssertPathPayload(t, "token", nil).
			RespondSuccess(json.RawMessage(`{"get":false}`))
		creq.GetResponse(t)
	})
}

// Test that a token event triggers a re-access call on subscribed resources
// and that the resource are still subscribed after given access
func TestTokenEventTriggersAccessCallOnSubscribedResources(t *testing.T) {
	runTest(t, func(s *Session) {
		token := `{"user":"foo"}`
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()

		cid := getCID(t, s, c)

		// Send token event
		s.ConnEvent(cid, "token", json.RawMessage(`{"token":`+token+`}`))

		// Get linked model
		subscribeToTestModelParent(t, s, c, false)

		// Get collection
		subscribeToTestCollection(t, s, c)

		// Change token
		s.ConnEvent(cid, "token", json.RawMessage(`{"token":`+token+`}`))

		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model.parent").RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.GetRequest(t, "access.test.collection").RespondSuccess(json.RawMessage(`{"get":true}`))

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

// Test that a token event triggers a re-access call on subscribed resources
// and that the resource are unsubscribed after being deniedn access
func TestTokenEventTriggersUnsubscribeOnDeniedAccessCall(t *testing.T) {
	runTest(t, func(s *Session) {
		token := `{"user":"foo"}`
		event := json.RawMessage(`{"foo":"bar"}`)
		reasonAccessDenied := json.RawMessage(`{"reason":{"code":"system.accessDenied","message":"Access denied"}}`)

		c := s.Connect()

		cid := getCID(t, s, c)

		// Send token event
		s.ConnEvent(cid, "token", json.RawMessage(`{"token":`+token+`}`))

		// Get linked model
		subscribeToTestModelParent(t, s, c, false)

		// Get collection
		subscribeToTestCollection(t, s, c)

		// Change token
		s.ConnEvent(cid, "token", json.RawMessage(`{"token":`+token+`}`))

		// Handle access requests with access denied
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model.parent").RespondSuccess(json.RawMessage(`{"get":false}`))
		mreqs.GetRequest(t, "access.test.collection").RespondError(reserr.ErrAccessDenied)

		// Validate unsubscribe events are sent to client
		evs := c.GetParallelEvents(t, 2)
		evs.GetEvent(t, "test.model.parent.unsubscribe").AssertData(t, reasonAccessDenied)
		evs.GetEvent(t, "test.collection.unsubscribe").AssertData(t, reasonAccessDenied)

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")

		// Send event on model parent and validate client event
		s.ResourceEvent("test.model.parent", "custom", event)
		c.AssertNoEvent(t, "test.model.parent")

		// Send event on collection and validate client event
		s.ResourceEvent("test.collection", "custom", event)
		c.AssertNoEvent(t, "test.collection")
	})
}
