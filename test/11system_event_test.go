package test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/resgateio/resgate/server"
	"github.com/resgateio/resgate/server/reserr"
)

// Test system reset event
func TestSystemResetEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		// Send token event
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))
	})
}

// Test that a system.reset event triggers get requests on matching model
func TestSystemResetTriggersGetRequestOnModel(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")

		c := s.Connect()

		// Get model
		subscribeToTestModel(t, s, c)

		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))

		// Validate a get request is sent
		s.GetRequest(t).AssertSubject(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate no events are sent to client
		c.AssertNoEvent(t, "test.model")
	})
}

// Test that a system.reset event triggers get requests on matching collection
func TestSystemResetTriggersGetRequestOnCollection(t *testing.T) {
	runTest(t, func(s *Session) {
		collection := resourceData("test.collection")

		c := s.Connect()

		// Get collection
		subscribeToTestCollection(t, s, c)

		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))

		// Validate a get request is sent
		s.GetRequest(t).AssertSubject(t, "get.test.collection").RespondSuccess(json.RawMessage(`{"collection":` + collection + `}`))

		// Validate no events are sent to client
		c.AssertNoEvent(t, "test.collection")
	})
}

func TestSystemReset_WithUpdatedResource_GeneratesEvents(t *testing.T) {
	type event struct {
		Event   string
		Payload string
	}
	tbl := []struct {
		RID            string
		ResetResponse  string
		ExpectedEvents []event
	}{
		{"test.model", `{"model":{"string":"foo","int":42,"bool":true,"null":null}}`, []event{}},
		{"test.model", `{"model":{"string":"bar","int":42,"bool":true}}`, []event{
			{"change", `{"values":{"string":"bar","null":{"action":"delete"}}}`},
		}},
		{"test.model", `{"model":{"string":"foo","int":42,"bool":true,"null":null,"child":{"rid":"test.model","soft":true}}}`, []event{
			{"change", `{"values":{"child":{"rid":"test.model","soft":true}}}`},
		}},
		{"test.model.soft", `{"model":{"name":"soft","child":null}}`, []event{
			{"change", `{"values":{"child":null}}`},
		}},
		{"test.collection", `{"collection":["foo",42,true,null]}`, []event{}},
		{"test.collection", `{"collection":[42,"new",true,null]}`, []event{
			{"remove", `{"idx":0}`},
			{"add", `{"idx":1,"value":"new"}`},
		}},
		{"test.collection", `{"collection":["foo",42,true,null,{"rid":"test.model","soft":true}]}`, []event{
			{"add", `{"idx":4,"value":{"rid":"test.model","soft":true}}`},
		}},
		{"test.collection.soft", `{"collection":["soft"]}`, []event{
			{"remove", `{"idx":1}`},
		}},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			c := s.Connect()

			// Get collection
			subscribeToResource(t, s, c, l.RID)

			// Send system reset
			s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))

			// Validate a get request is sent
			s.GetRequest(t).AssertSubject(t, "get."+l.RID).RespondSuccess(json.RawMessage(l.ResetResponse))

			for _, ev := range l.ExpectedEvents {
				// Validate no events are sent to client
				c.GetEvent(t).AssertEventName(t, l.RID+"."+ev.Event).AssertData(t, json.RawMessage(ev.Payload))
			}
			c.AssertNoEvent(t, l.RID)
		})
	}
}

// Test that a system.reset event triggers a re-access call on subscribed resources
// and that the resource are still subscribed after given access
func TestSystemAccessEventTriggersAccessCallOnSubscribedResources(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()

		// Get linked model
		subscribeToTestModelParent(t, s, c, false)

		// Get collection
		subscribeToTestCollection(t, s, c)

		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"access":["test.model.>"]}`))

		// Handle access requests with access denied
		s.GetRequest(t).AssertSubject(t, "access.test.model.parent").RespondSuccess(json.RawMessage(`{"get":true}`))

		// Validate no unsubscribe events are sent to client
		c.AssertNoEvent(t, "test.model.parent")

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

// Test that a reaccess event triggers a re-access call on subscribed resources
// and that the resource are unsubscribed after being deniedn access
func TestSystemResetEventTriggersUnsubscribeOnDeniedAccessCall(t *testing.T) {
	runTest(t, func(s *Session) {
		event := json.RawMessage(`{"foo":"bar"}`)
		reasonAccessDenied := json.RawMessage(`{"reason":{"code":"system.accessDenied","message":"Access denied"}}`)

		c := s.Connect()

		// Get linked model
		subscribeToTestModelParent(t, s, c, false)

		// Get collection
		subscribeToTestCollection(t, s, c)

		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"access":["test.model.>"]}`))

		// Handle access requests with access denied
		s.GetRequest(t).AssertSubject(t, "access.test.model.parent").RespondSuccess(json.RawMessage(`{"get":false}`))

		// Validate unsubscribe events are sent to client
		c.GetEvent(t).AssertEventName(t, "test.model.parent.unsubscribe").AssertData(t, reasonAccessDenied)

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", event)
		c.AssertNoEvent(t, "test.model")

		// Send event on model parent and validate client event
		s.ResourceEvent("test.model.parent", "custom", event)
		c.AssertNoEvent(t, "test.model.parent")

		// Send event on collection and validate client event
		s.ResourceEvent("test.collection", "custom", event)
		c.GetEvent(t).Equals(t, "test.collection.custom", event)
	})
}

// Test that a system.reset event triggers get requests on query model
func TestSystemResetTriggersGetRequestOnQueryModel(t *testing.T) {
	tbl := []struct {
		Query      string
		Normalized string
	}{
		{"foo=bar", "foo=bar"},
		{"a=b&foo=bar", "foo=bar"},
		{"", "foo=bar"},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			model := resourceData("test.model")

			c := s.Connect()

			// Get model
			subscribeToTestQueryModel(t, s, c, l.Query, l.Normalized)

			// Send system reset
			s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))

			// Validate a get request is sent
			s.GetRequest(t).
				AssertSubject(t, "get.test.model").
				AssertPathPayload(t, "query", l.Normalized).
				RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

			// Validate no more requests are sent to NATS
			c.AssertNoNATSRequest(t, "test.model")

			// Validate no events are sent to client
			c.AssertNoEvent(t, "test.model")
		})
	}
}

func TestSystemReset_NotFoundResponseOnModel_GeneratesDeleteEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Get model
		subscribeToTestModel(t, s, c)
		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))
		// Respond to get request with system.notFound error
		s.GetRequest(t).AssertSubject(t, "get.test.model").RespondError(reserr.ErrNotFound)
		// Validate delete event is sent to client
		c.GetEvent(t).AssertEventName(t, "test.model.delete").AssertData(t, nil)
		// Validate subsequent events are not sent to client
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.AssertNoEvent(t, "test.model")
	})
}

func TestSystemReset_NotFoundResponseOnCollection_GeneratesDeleteEvent(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Get model
		subscribeToTestCollection(t, s, c)
		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))
		// Respond to get request with system.notFound error
		s.GetRequest(t).AssertSubject(t, "get.test.collection").RespondError(reserr.ErrNotFound)
		// Validate delete event is sent to client
		c.GetEvent(t).AssertEventName(t, "test.collection.delete").AssertData(t, nil)
		// Validate subsequent events are not sent to client
		s.ResourceEvent("test.collection", "custom", common.CustomEvent())
		c.AssertNoEvent(t, "test.collection")
	})
}

func TestSystemReset_InternalErrorResponseOnModel_LogsError(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Get model
		subscribeToTestModel(t, s, c)
		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))
		// Respond to get request with system.notFound error
		s.GetRequest(t).AssertSubject(t, "get.test.model").RespondError(reserr.ErrInternalError)
		// Validate no delete event is sent to client
		c.AssertNoEvent(t, "test.model")
		// Validate subsequent events are sent to client
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.model.custom", common.CustomEvent())
		// Assert error is logged
		s.AssertErrorsLogged(t, 1)
	})
}

func TestSystemReset_InternalErrorResponseOnCollection_LogsError(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Get collection
		subscribeToTestCollection(t, s, c)
		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))
		// Respond to get request with system.notFound error
		s.GetRequest(t).AssertSubject(t, "get.test.collection").RespondError(reserr.ErrInternalError)
		// Validate no delete event is sent to client
		c.AssertNoEvent(t, "test.collection")
		// Validate subsequent events are sent to client
		s.ResourceEvent("test.collection", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.collection.custom", common.CustomEvent())
		// Assert error is logged
		s.AssertErrorsLogged(t, 1)
	})
}

func TestSystemReset_MismatchingResourceTypeResponseOnModel_LogsError(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Get model
		subscribeToTestModel(t, s, c)
		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))
		// Respond to get request with mismatching type
		s.GetRequest(t).AssertSubject(t, "get.test.model").RespondSuccess(json.RawMessage(`{"collection":["foo",42,true,null]}`))
		// Validate no delete event is sent to client
		c.AssertNoEvent(t, "test.model")
		// Validate subsequent events are sent to client
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.model.custom", common.CustomEvent())
		// Assert error is logged
		s.AssertErrorsLogged(t, 1)
	})
}

func TestSystemReset_MismatchingResourceTypeResponseOnCollection_LogsError(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Get collection
		subscribeToTestCollection(t, s, c)
		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))
		// Respond to get request with mismatching type
		s.GetRequest(t).AssertSubject(t, "get.test.collection").RespondSuccess(json.RawMessage(`{"model":{"string":"foo","int":42,"bool":true,"null":null}}`))
		// Validate no delete event is sent to client
		c.AssertNoEvent(t, "test.collection")
		// Validate subsequent events are sent to client
		s.ResourceEvent("test.collection", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.collection.custom", common.CustomEvent())
		// Assert error is logged
		s.AssertErrorsLogged(t, 1)
	})
}

func TestSystemReset_WithThrottle_ThrottlesRequests(t *testing.T) {
	const subscriptionCount = 5
	const resetThrottle = 3
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Get subscriptions
		for i := 1; i <= subscriptionCount; i++ {
			subscribeToCustomResource(t, s, c, fmt.Sprintf("test.model.%d", i), resource{
				typ:  typeModel,
				data: fmt.Sprintf(`{"id":%d}`, i),
			})
		}
		// Send system reset
		s.SystemEvent("reset", json.RawMessage(`{"resources":["test.>"]}`))
		// Get throttled number of requests
		mreqs := s.GetParallelRequests(t, resetThrottle)
		requestCount := resetThrottle
		// Assert no other requests are sent
		for i := 1; i <= subscriptionCount; i++ {
			c.AssertNoNATSRequest(t, fmt.Sprintf("test.model.%d", i))
		}
		// Respond to requests one by one
		for len(mreqs) > 0 {
			r := mreqs[0]
			mreqs = mreqs[1:]
			id := r.Subject[strings.LastIndexByte(r.Subject, '.')+1:]
			r.RespondSuccess(json.RawMessage(`{"model":` + fmt.Sprintf(`{"id":%s}`, id) + `}`))
			// If we still have remaining subscriptions not yet received
			if requestCount < subscriptionCount {
				// For each response, a new request should be sent.
				req := s.GetRequest(t)
				mreqs = append(mreqs, req)
				requestCount++
				// Assert no other requests are sent
				for i := 1; i <= subscriptionCount; i++ {
					c.AssertNoNATSRequest(t, fmt.Sprintf("test.model.%d", i))
				}
			}
		}

	}, func(c *server.Config) {
		c.ResetThrottle = resetThrottle
	})

}
