package test

import (
	"encoding/json"
	"testing"
)

// Test responses to query model subscribe requests
func TestSendingQueryToNonQueryModel(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resource["test.model"]
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()

		// Send subscribe request
		creq := c.Request("subscribe.test.model?q=foo&f=bar", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.
			GetRequest(t, "get.test.model").
			AssertPathPayload(t, "query", "q=foo&f=bar").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		mreqs.GetRequest(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response and validate
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model?q=foo&f=bar":`+model+`}}`))

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.GetEvent(t).Equals(t, "test.model?q=foo&f=bar.custom", event)
		c.AssertNoNATSRequest(t, "test.model")
	})
}

// Test subscribing to query model
func TestSubscribingToQueryModel(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send event on non-query model and validate no event is sent to client
		s.ResourceEvent("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")
		c.AssertNoNATSRequest(t, "test.model")
	})
}

// Test subscribing to query model
func TestSubscribingToQueryModelWithNormalization(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar&ignore=true", "q=foo&f=bar")

		// Send event on non-query model and validate no event is sent to client
		s.ResourceEvent("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")
		c.AssertNoNATSRequest(t, "test.model")
	})
}

// Test subscribing to query model
func TestQueryModelIsFetchedFromCache(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resource["test.model"]
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()

		// Send subscribe request
		creq := c.Request("subscribe.test.model?q=foo&f=bar", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.
			GetRequest(t, "get.test.model").
			RespondSuccess(json.RawMessage(`{"model":` + model + `,"query":"q=foo&f=bar"}`))
		mreqs.GetRequest(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response and validate
		creq.GetResponse(t)

		c2 := s.Connect()
		// Subscribe a second time
		creq2 := c2.Request("subscribe.test.model?q=foo&f=bar", nil)
		// Handle model get and access request
		mreqs2 := s.GetParallelRequests(t, 1)
		mreqs2.GetRequest(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response and validate
		creq2.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model?q=foo&f=bar":`+model+`}}`))

		// Send event on non-query model and validate no event is sent to client
		s.ResourceEvent("test.model", "custom", event)
		c2.AssertNoEvent(t, "test.model")
		c2.AssertNoNATSRequest(t, "test.model")
	})
}

// Test subscribing to query model
func TestQueryModelIsFetchedFromCacheAfterQueryNormalization(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resource["test.model"]
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()

		// Send subscribe request
		creq := c.Request("subscribe.test.model?q=foo&f=bar&ignore=true", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.
			GetRequest(t, "get.test.model").
			RespondSuccess(json.RawMessage(`{"model":` + model + `,"query":"f=bar&q=foo"}`))
		mreqs.GetRequest(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response and validate
		creq.GetResponse(t)

		c2 := s.Connect()
		// Subscribe to the query resource a second time using the normalized query
		creq2 := c2.Request("subscribe.test.model?f=bar&q=foo", nil)
		// Handle model get and access request
		mreqs2 := s.GetParallelRequests(t, 1)
		mreqs2.GetRequest(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response and validate
		creq2.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model?f=bar&q=foo":`+model+`}}`))

		// Send event on non-query model and validate no event is sent to client
		s.ResourceEvent("test.model", "custom", event)
		c2.AssertNoEvent(t, "test.model")
		c2.AssertNoNATSRequest(t, "test.model")

		c3 := s.Connect()
		// Subscribe to the query resource a third time using the non-normalized query
		creq3 := c3.Request("subscribe.test.model?q=foo&f=bar&ignore=true", nil)
		// Handle model get and access request
		mreqs3 := s.GetParallelRequests(t, 1)
		mreqs3.GetRequest(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response and validate
		creq3.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model?q=foo&f=bar&ignore=true":`+model+`}}`))

		// Send event on non-query model and validate no event is sent to client
		s.ResourceEvent("test.model", "custom", event)
		c3.AssertNoEvent(t, "test.model")
		c3.AssertNoNATSRequest(t, "test.model")
	})
}

// Test subscribing to two different queries on the same model triggers
// get and access requests for each.
func TestDifferentQueriesTriggersGetAndAccessRequests(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")
		subscribeToTestQueryModel(t, s, c, "q=foo&f=baz", "q=foo&f=baz")
	})
}
