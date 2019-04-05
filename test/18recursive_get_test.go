package test

import (
	"encoding/json"
	"testing"
)

// Test handle get requests with recursive data
func TestRecursiveGetTest(t *testing.T) {
	var resources = map[string]string{
		"example.parent":   `{"childA":{"rid":"example.a"},"childB":{"rid":"example.b"}}`,
		"example.a":        `{"siblings":{"rid":"example.siblings"}}`,
		"example.b":        `{"siblings":{"rid":"example.siblings"}}`,
		"example.siblings": `[{"rid":"example.a"},{"rid":"example.b"}]`,
	}

	runTest(t, func(s *Session) {
		c := s.Connect()
		creq := c.Request("get.example.parent", nil)

		// Handle model a get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.example.parent")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.example.parent")
		req.RespondSuccess(json.RawMessage(`{"model":` + resources["example.parent"] + `}`))

		// Handle siblings get requests
		mreqs = s.GetParallelRequests(t, 2)
		req = mreqs.GetRequest(t, "get.example.a")
		req.RespondSuccess(json.RawMessage(`{"model":` + resources["example.a"] + `}`))
		req = mreqs.GetRequest(t, "get.example.b")
		req.RespondSuccess(json.RawMessage(`{"model":` + resources["example.b"] + `}`))

		// Handle siblings get requests
		mreqs = s.GetParallelRequests(t, 1)
		req = mreqs.GetRequest(t, "get.example.siblings")
		req.RespondSuccess(json.RawMessage(`{"collection":` + resources["example.siblings"] + `}`))

		// Validate client response
		creq.GetResponse(t)
	})
}
