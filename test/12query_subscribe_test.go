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
		mreqs.GetRequest(t, "get.test.model").AssertPathPayload(t, "query", "q=foo&f=bar").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		mreqs.GetRequest(t, "access.test.model").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate client response and validate
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model?q=foo&f=bar":`+model+`}}`))

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.GetEvent(t).Equals(t, "test.model?q=foo&f=bar.custom", event)
	})
}
