package test

import (
	"encoding/json"
	"testing"
)

// Test query event
func TestModelQueryEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		s.
			GetRequest(t).
			Equals(t, "_EVENT_01_", json.RawMessage(`{"query":"q=foo&f=bar"}`)).
			RespondSuccess(json.RawMessage(`{"events":[]}`))
	})
}

// Test query event with omitted events array
func TestModelQueryEventWithOmittedEventsToQueryRequest(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		s.
			GetRequest(t).
			Equals(t, "_EVENT_01_", json.RawMessage(`{"query":"q=foo&f=bar"}`)).
			RespondSuccess(json.RawMessage(`{}`))

		c.AssertNoEvent(t, "test.model")
	})
}

// Test query events on multiple queries on the same model triggers
func TestModelQueryEventOnMultipleQueries(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Subscribe with different queries to the same model
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")
		subscribeToTestQueryModel(t, s, c, "q=foo&f=baz", "q=foo&f=baz")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests for the two model queries
		req1 := s.GetRequest(t).AssertSubject(t, "_EVENT_01_")
		req2 := s.GetRequest(t).AssertSubject(t, "_EVENT_01_")
		// Determine which order the query requests came and validate
		if req1.PathPayload(t, "query").(string) == "q=foo&f=bar" {
			req1.AssertPathPayload(t, "query", "q=foo&f=bar")
			req2.AssertPathPayload(t, "query", "q=foo&f=baz")
		} else {
			req1.AssertPathPayload(t, "query", "q=foo&f=baz")
			req2.AssertPathPayload(t, "query", "q=foo&f=bar")
		}
		// Send query response without events
		req1.RespondSuccess(json.RawMessage(`{}`))
		req2.RespondSuccess(json.RawMessage(`{}`))

		// Validate no events was sent to the client
		c.AssertNoEvent(t, "test.model")
	})
}

// Test that query event triggers a single query request for multiple queries with
// identical normalization
func TestModelQueryEventOnSameNormalizedQueryResultsInSingleQueryRequest(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Subscribe with different queries to the same model
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "f=bar&q=foo")
		subscribeToTestQueryModel(t, s, c, "f=bar&q=foo&fake=1", "f=bar&q=foo")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests and respond
		s.
			GetRequest(t).
			AssertPathPayload(t, "query", "f=bar&q=foo").
			RespondSuccess(json.RawMessage(`{}`))

		// Validate no events was sent to the client
		c.AssertNoEvent(t, "test.model")
		c.AssertNoNATSRequest(t, "test.model")
	})
}

// Test query event resulting in change event
func TestModelQueryEventResultingInChangeEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with a single change event
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"string":"bar","int":-12}}]}`))

		// Validate change event was sent to client
		c.GetEvent(t).Equals(t, "test.model?q=foo&f=bar.change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
	})
}

// Test that two different model queries with the same normalized query both gets
// a client change event
func TestDifferentModelQueriesWithSameNormalizedQueryEachGetsClientChangeEvents(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Subscribe with different queries to the same model
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "f=bar&q=foo")
		subscribeToTestQueryModel(t, s, c, "f=bar&q=foo&fake=1", "f=bar&q=foo")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Get query requests and respond
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"string":"bar","int":-12}}]}`))

		// Validate client change event was sent on both query models
		evs := c.GetParallelEvents(t, 2)
		evs.GetEvent(t, "test.model?q=foo&f=bar.change").AssertData(t, json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
		evs.GetEvent(t, "test.model?f=bar&q=foo&fake=1.change").AssertData(t, json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
		c.AssertNoNATSRequest(t, "test.model")
	})
}

// Test query event resulting in change event
func TestModelQueryEventResponseUpdatesTheCache(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestQueryModel(t, s, c, "q=foo&f=bar", "q=foo&f=bar")

		// Send query event
		s.ResourceEvent("test.model", "query", json.RawMessage(`{"subject":"_EVENT_01_"}`))
		// Respond to query request with a single change event
		s.GetRequest(t).RespondSuccess(json.RawMessage(`{"events":[{"event":"change","data":{"string":"bar","int":-12}}]}`))
		// Get change event send to the client
		c.GetEvent(t).Equals(t, "test.model?q=foo&f=bar.change", json.RawMessage(`{"values":{"string":"bar","int":-12}}`))
	})
}
