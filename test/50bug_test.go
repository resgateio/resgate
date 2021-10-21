package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/resgateio/resgate/server"
)

// Test to replicate the bug about possible client resource inconsistency.
//
// See: https://github.com/resgateio/resgate/issues/194
func TestBug_PossibleClientResourceInconsistency(t *testing.T) {
	rsrc := resource{
		typ:  typeCollection,
		data: `[1,2,3]`,
	}
	addEvent := json.RawMessage(`{"value":4,"idx":3}`)

	// Make 100 attempts to try trigger the error
	for i := 0; i < 100; i++ {
		runNamedTest(t, fmt.Sprintf("Attempt #%d/100", i+1), func(s *Session) {
			// Connect with first client to cache collection
			c1 := s.Connect()
			subscribeToCustomResource(t, s, c1, "test.collection", rsrc)

			// Connect with second client
			c2 := s.Connect()
			subscribeToTestModel(t, s, c2)

			// Send system reset and add an item
			s.SystemEvent("reset", json.RawMessage(`{"resources":["test.collection"]}`))
			// Send event on model to indirectly subscribe cached collection
			s.ResourceEvent("test.model", "change", json.RawMessage(`{"values":{"ref":{"rid":"test.collection"}}}`))
			// Respond to the reset's get request
			s.GetRequest(t).AssertSubject(t, "get.test.collection").RespondSuccess(json.RawMessage(`{"collection":[1,2,3,4]}`))

			// Handle collection get request
			ev := c2.GetEvent(t)

			// If the add event was applied before the change, we should expect an
			// add event.
			if ev.IsData(json.RawMessage(`{"values":{"ref":{"rid":"test.collection"}},"collections":{"test.collection":[1,2,3]}}`)) {
				c2.GetEvent(t).Equals(t, "test.collection.add", addEvent)
			}
			// We should expect no more events
			c2.AssertNoEvent(t, "test.collection")

			// Get the add event for the first client
			c1.GetEvent(t).Equals(t, "test.collection.add", addEvent)
		})
	}
}

// Test to replicate the bug: Deadlock on throttled access requests to same resource
//
// See: https://github.com/resgateio/resgate/issues/217
func TestBug_DeadlockOnThrottledAccessRequestsToSameResource(t *testing.T) {
	const connectionCount = 4
	const resetThrottle = 3
	rid := "test.model"
	model := resources[rid].data
	runTest(t, func(s *Session) {
		// Create a set of connections subscribing to the same resource
		conns := make([]*Conn, 0, connectionCount)
		for i := 0; i < connectionCount; i++ {
			c := s.Connect()

			creq := c.Request("subscribe."+rid, nil)
			reqCount := 1
			if i == 0 {
				reqCount = 2
			}
			// Handle access request (and model request for the first connection)
			mreqs := s.GetParallelRequests(t, reqCount)
			// Handle access
			mreqs.GetRequest(t, "access."+rid).
				RespondSuccess(json.RawMessage(`{"get":true}`))
			if i == 0 {
				// Handle get
				mreqs.GetRequest(t, "get."+rid).
					RespondSuccess(json.RawMessage(fmt.Sprintf(`{"model":%s}`, model)))
			}
			creq.GetResponse(t)

			conns = append(conns, c)
		}

		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":null,"access":["test.>"]}`))
		// Get throttled number of requests
		mreqs := s.GetParallelRequests(t, resetThrottle)
		requestCount := resetThrottle

		// Respond to requests one by one
		for len(mreqs) > 0 {
			r := mreqs[0]
			mreqs = mreqs[1:]
			r.RespondSuccess(json.RawMessage(`{"get":true}`))

			// If we still have remaining get or access requests not yet received
			if requestCount < connectionCount {
				// For each response, a new request should be sent.
				req := s.GetRequest(t)
				mreqs = append(mreqs, req)
				requestCount++
			}
		}

		// Assert no other requests are sent
		for _, c := range conns {
			c.AssertNoNATSRequest(t, rid)
		}

	}, func(c *server.Config) {
		c.ResetThrottle = resetThrottle
	})
}
