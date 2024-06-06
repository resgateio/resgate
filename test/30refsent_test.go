// Tests for the cases when a directly or indirectly subscribed resource has
// been sent to the client, and later is getting unsubscribe at the same time
// that another indirect reference is added but send to the client after the
// unsubscription.
//
// See: https://github.com/resgateio/resgate/issues/241
package test

import (
	"encoding/json"
	"testing"
)

func TestRefSent_SubscriptionReferenceDirectlyUnsubscribedBeforeReady_ContainsResource(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")
		modelDelayed := `{"foo":"modelDelayed"}`
		modelParent := `{"name":"parent","child":{"rid":"test.model"},"delayed":{"rid":"test.model.delayed"}}`

		c := s.Connect()
		subscribeToTestModel(t, s, c)

		// Get parent model
		creq := c.Request("subscribe.test.model.parent", nil)

		// Handle parent get and access request.
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model.parent").RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.GetRequest(t, "get.test.model.parent").RespondSuccess(json.RawMessage(`{"model":` + modelParent + `}`))
		// Delay response to get request of referenced test.model.delayed
		mreqsecond := s.GetRequest(t)

		// Call unsubscribe on (first) parent
		c.Request("unsubscribe.test.model", nil).GetResponse(t)

		// Repond to parent get request
		mreqsecond.RespondSuccess(json.RawMessage(`{"model":` + modelDelayed + `}`))

		// Get client response, which should include test.model.
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`,"test.model.parent":`+modelParent+`,"test.model.delayed":`+modelDelayed+`}}`))
	})
}

func TestRefSent_SubscriptionReferenceIndirectlyUnsubscribedBeforeReady_ContainsResource(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")
		modelDelayed := `{"foo":"modelDelayed"}`
		modelSecondParent := `{"name":"secondparent","child":{"rid":"test.model"},"delayed":{"rid":"test.model.delayed"}}`

		c := s.Connect()
		subscribeToTestModelParent(t, s, c, false)

		// Get secondparent model
		creq := c.Request("subscribe.test.model.secondparent", nil)

		// Handle secondparent get and access request.
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model.secondparent").RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.GetRequest(t, "get.test.model.secondparent").RespondSuccess(json.RawMessage(`{"model":` + modelSecondParent + `}`))
		// Delay response to get request of referenced test.model.delayed
		mreqsecond := s.GetRequest(t)

		// Call unsubscribe on (first) parent
		c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)

		// Repond to secondparent get request
		mreqsecond.RespondSuccess(json.RawMessage(`{"model":` + modelDelayed + `}`))

		// Get client response, which should include test.model.
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`,"test.model.secondparent":`+modelSecondParent+`,"test.model.delayed":`+modelDelayed+`}}`))
	})
}

func TestRefSent_ChangeEventReferenceDirectlyUnsubscribedBeforeReady_ContainsResource(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")
		modelDelayed := `{"foo":"modelDelayed"}`
		modelParent := `{"name":"parent","child":{"rid":"test.model"},"delayed":{"rid":"test.model.delayed"}}`
		modelGrandParent := `{"name":"grandparent","parent":null}`

		c := s.Connect()
		subscribeToTestModel(t, s, c)

		// Get grandparent model
		creq := c.Request("subscribe.test.model.grandparent", nil)

		// Handle parent get and access request.
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model.grandparent").RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.GetRequest(t, "get.test.model.grandparent").RespondSuccess(json.RawMessage(`{"model":` + modelGrandParent + `}`))

		// Get client response.
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model.grandparent":`+modelGrandParent+`}}`))

		// Send change event, adding parent model reference to grandparent.
		s.ResourceEvent("test.model.grandparent", "change", json.RawMessage(`{"values":{"parent":{"rid":"test.model.parent"}}}`))

		// Handle parent get request
		s.GetRequest(t).AssertSubject(t, "get.test.model.parent").RespondSuccess(json.RawMessage(`{"model":` + modelParent + `}`))

		// Delay response to get request of referenced test.model.delayed
		mreqsecond := s.GetRequest(t)

		// Call unsubscribe on (first) parent
		c.Request("unsubscribe.test.model", nil).GetResponse(t)

		// Repond to parent get request
		mreqsecond.RespondSuccess(json.RawMessage(`{"model":` + modelDelayed + `}`))

		// Get change event, which should include test.model
		c.GetEvent(t).Equals(t, "test.model.grandparent.change", json.RawMessage(`{"values":{"parent":{"rid":"test.model.parent"}},"models":{"test.model":`+model+`,"test.model.parent":`+modelParent+`,"test.model.delayed":`+modelDelayed+`}}`))

		// Send event on parent and validate client event
		s.ResourceEvent("test.model.parent", "custom", common.CustomEvent())
		c.GetEvent(t).Equals(t, "test.model.parent.custom", common.CustomEvent())
	})
}
