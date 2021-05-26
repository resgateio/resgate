package test

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Test to replicate the bug about possible client resource inconcistency.
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
