package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/resgateio/resgate/server/reserr"
)

// Test change event effect on cached model
func TestLegacy120ChangeEvent_OnCachedModel(t *testing.T) {
	tbl := []struct {
		RID                 string // RID of resource to subscribe to
		ChangeEvent         string // Change event to send (raw JSON)
		ExpectedChangeEvent string // Expected event sent to client (raw JSON. Empty means none)
		ExpectedModel       string // Expected model after event (raw JSON)
		ExpectedErrors      int
	}{
		{"test.model", `{"values":{"soft":{"rid":"test.model.soft","soft":true}}}`, `{"values":{"soft":"test.model.soft"}}`, `{"string":"foo","int":42,"bool":true,"null":null,"soft":"test.model.soft"}`, 0},
		{"test.model.soft", `{"values":{"child":null}}`, `{"values":{"child":null}}`, `{"name":"soft","child":null}`, 0},
		{"test.model.soft", `{"values":{"child":{"action":"delete"}}}`, `{"values":{"child":{"action":"delete"}}}`, `{"name":"soft"}`, 0},

		// Unchanged values
		{"test.model.soft", `{"values":{"child":{"rid":"test.model","soft":true}}}`, "", `{"name":"soft","child":"test.model"}`, 0},
	}

	for i, l := range tbl {
		for sameClient := true; sameClient; sameClient = false {
			runNamedTest(t, fmt.Sprintf("#%d with the same client being %+v", i+1, sameClient), func(s *Session) {
				var creq *ClientRequest

				rid := l.RID
				c := s.ConnectWithVersion("1.2.0")

				// Subscribe to resource
				r := resources[rid].data
				// Send subscribe request
				creq = c.Request("subscribe."+rid, nil)
				// Handle model get and access request
				mreqs := s.GetParallelRequests(t, 2)
				req := mreqs.GetRequest(t, "access."+rid)
				req.RespondSuccess(json.RawMessage(`{"get":true}`))
				mreqs.GetRequest(t, "get."+rid).RespondSuccess(json.RawMessage(`{"model":` + r + `}`))
				creq.GetResponse(t)

				// Send event on model and validate client event
				s.ResourceEvent(rid, "change", json.RawMessage(l.ChangeEvent))
				if l.ExpectedChangeEvent == "" {
					c.AssertNoEvent(t, rid+".change")
				} else {
					c.GetEvent(t).Equals(t, rid+".change", json.RawMessage(l.ExpectedChangeEvent))
				}

				if sameClient {
					c.Request("unsubscribe."+rid, nil).GetResponse(t)
					// Subscribe a second time
					creq = c.Request("subscribe."+rid, nil)
				} else {
					c2 := s.Connect()
					// Subscribe a second time
					creq = c2.Request("subscribe."+rid, nil)
				}

				// Handle model access request
				s.GetRequest(t).AssertSubject(t, "access."+rid).RespondSuccess(json.RawMessage(`{"get":true}`))

				// Validate client response
				creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"`+rid+`":`+l.ExpectedModel+`}}`))
				s.AssertErrorsLogged(t, l.ExpectedErrors)
			})
		}
	}
}

// Test change event with new resource reference
func TestLegacy120ChangeEvent_WithSoftReferenceReplacedByResourceReference_SubscribesReference(t *testing.T) {
	model := resourceData("test.model")

	runTest(t, func(s *Session) {
		c := s.ConnectWithVersion("1.2.0")
		// Subscribe to resource
		rid := "test.model.soft"
		r := resources[rid].data
		// Send subscribe request
		creq := c.Request("subscribe."+rid, nil)
		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access."+rid)
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.GetRequest(t, "get."+rid).RespondSuccess(json.RawMessage(`{"model":` + r + `}`))
		creq.GetResponse(t)

		// Send event on model and validate client event
		s.ResourceEvent(rid, "change", json.RawMessage(`{"values":{"child":{"rid":"test.model","soft":false}}}`))

		// Handle model get request
		s.
			GetRequest(t).
			AssertSubject(t, "get.test.model").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		c.GetEvent(t).Equals(t, rid+".change", json.RawMessage(`{"values":{"child":{"rid":"test.model","soft":false}},"models":{"test.model":`+model+`}}`))

		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.model.custom", common.CustomEvent())
	})
}

// Test add and remove event effects on cached collection
func TestLegacy120AddRemoveEvents_OnCachedCollection(t *testing.T) {
	tbl := []struct {
		RID                string // Resource ID
		EventName          string // Name of the event. Either add or remove.
		EventPayload       string // Event payload (raw JSON)
		ClientEventPayload string // Event payload as sent to client(raw JSON)
		ExpectedCollection string // Expected collection after event (raw JSON)
	}{
		{"test.collection", "add", `{"idx":0,"value":{"rid":"test.collection.soft","soft":true}}`, `{"idx":0,"value":"test.collection.soft"}`, `["test.collection.soft","foo",42,true,null]`},
		{"test.collection.soft", "remove", `{"idx":1}`, `{"idx":1}`, `["soft"]`},
	}

	for i, l := range tbl {
		for sameClient := true; sameClient; sameClient = false {
			runNamedTest(t, fmt.Sprintf("#%d with the same client being %+v", i+1, sameClient), func(s *Session) {
				var creq *ClientRequest

				c := s.ConnectWithVersion("1.2.0")
				// Subscribe to resource
				rid := l.RID
				r := resources[rid].data
				// Send subscribe request
				creq = c.Request("subscribe."+rid, nil)
				// Handle model get and access request
				mreqs := s.GetParallelRequests(t, 2)
				req := mreqs.GetRequest(t, "access."+rid)
				req.RespondSuccess(json.RawMessage(`{"get":true}`))
				mreqs.GetRequest(t, "get."+rid).RespondSuccess(json.RawMessage(`{"collection":` + r + `}`))
				creq.GetResponse(t)

				// Send event on collection and validate client event
				s.ResourceEvent(rid, l.EventName, json.RawMessage(l.EventPayload))
				c.GetEvent(t).Equals(t, rid+"."+l.EventName, json.RawMessage(l.ClientEventPayload))

				if sameClient {
					c.Request("unsubscribe."+rid, nil).GetResponse(t)
					// Subscribe a second time
					creq = c.Request("subscribe."+rid, nil)
				} else {
					c2 := s.Connect()
					// Subscribe a second time
					creq = c2.Request("subscribe."+rid, nil)
				}

				// Handle collection access request
				s.GetRequest(t).AssertSubject(t, "access."+l.RID).RespondSuccess(json.RawMessage(`{"get":true}`))

				// Validate client response
				creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{"`+l.RID+`":`+l.ExpectedCollection+`}}`))
			})
		}
	}
}

func TestLegacy120Subscribe_StaticResource_ReturnsErrorUnsupportedFeature(t *testing.T) {
	runTest(t, func(s *Session) {
		static := resourceData("test.static")
		c := s.ConnectWithVersion("1.2.0")

		creq := c.Request("subscribe.test.static", nil)

		// Handle static get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test.static")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test.static")
		req.RespondSuccess(json.RawMessage(`{"static":` + static + `}`))
		// Validate client response
		creq.GetResponse(t).AssertError(t, reserr.ErrUnsupportedFeature)

		// Send event on model and validate client did get event
		creq.c.AssertNoEvent(t, "test.static")
	})
}
