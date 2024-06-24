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

// testIndirectSentSubscribeTypes contains different ways to subscribe and
// unsubscribe to the resource test.model, directly or indirectly.
var testIndirectSentSubscribeTypes = []struct {
	Name        string
	Subscribe   func(t *testing.T, s *Session, c *Conn)
	Unsubscribe func(t *testing.T, s *Session, c *Conn)
}{
	{
		"with directly unsubscribed resource",
		func(t *testing.T, s *Session, c *Conn) {
			subscribeToTestModel(t, s, c)
		},
		func(t *testing.T, s *Session, c *Conn) {
			c.Request("unsubscribe.test.model", nil).GetResponse(t)
		},
	},
	{
		"with indirectly unsubscribed resource",
		func(t *testing.T, s *Session, c *Conn) {
			subscribeToTestModelParent(t, s, c, false)
		},
		func(t *testing.T, s *Session, c *Conn) {
			c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)
		},
	},
	{
		"with change event indirectly unsubscribed resource",
		func(t *testing.T, s *Session, c *Conn) {
			subscribeToTestModelParent(t, s, c, false)
		},
		func(t *testing.T, s *Session, c *Conn) {
			// Send change event removing the indirectly subscribed resource.
			s.ResourceEvent("test.model.parent", "change", json.RawMessage(`{"values":{"child":null}}`))
			// Consume client change event.
			c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":null}}`))
		},
	},
	{
		"with directly subscribe, indirectly subscribe with change event, directly unsubscribe, indirectly unsubscribe with change event",
		func(t *testing.T, s *Session, c *Conn) {
			subscribeToTestModel(t, s, c)
			subscribeToCustomResource(t, s, c, "test.model.parent", resource{typeModel, `{"name":"parent","child":null}`, nil})
			// Send change event adding the indirectly subscribed resource.
			s.ResourceEvent("test.model.parent", "change", json.RawMessage(`{"values":{"child":{"rid":"test.model"}}}`))
			// Consome client change event
			c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":{"rid":"test.model"}}}`))
		},
		func(t *testing.T, s *Session, c *Conn) {
			c.Request("unsubscribe.test.model", nil).GetResponse(t)
			// Send change event removing the indirectly subscribed resource.
			s.ResourceEvent("test.model.parent", "change", json.RawMessage(`{"values":{"child":null}}`))
			// Consume client change event.
			c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":null}}`))
		},
	},
	{
		"with change event indirectly subscribing and unsubscribing resource",
		func(t *testing.T, s *Session, c *Conn) {
			model := resourceData("test.model")
			subscribeToCustomResource(t, s, c, "test.model.parent", resource{typeModel, `{"name":"parent","child":null}`, nil})
			// Send change event adding the indirectly subscribed resource.
			s.ResourceEvent("test.model.parent", "change", json.RawMessage(`{"values":{"child":{"rid":"test.model"}}}`))
			// Handle get request
			s.GetRequest(t).AssertSubject(t, "get.test.model").RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
			// Consome client change event
			c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":{"rid":"test.model"}},"models":{"test.model":`+model+`}}`))
		},
		func(t *testing.T, s *Session, c *Conn) {
			// Send change event removing the indirectly subscribed resource.
			s.ResourceEvent("test.model.parent", "change", json.RawMessage(`{"values":{"child":null}}`))
			// Consume client change event.
			c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":null}}`))
		},
	},
	{
		"with directly unsubscribed resource after removing an indirect reference using change event",
		func(t *testing.T, s *Session, c *Conn) {
			subscribeToTestModel(t, s, c)
			subscribeToTestModelParent(t, s, c, true)
		},
		func(t *testing.T, s *Session, c *Conn) {
			// Send change event removing the indirectly subscribed resource.
			s.ResourceEvent("test.model.parent", "change", json.RawMessage(`{"values":{"child":null}}`))
			// Consume client change event.
			c.GetEvent(t).Equals(t, "test.model.parent.change", json.RawMessage(`{"values":{"child":null}}`))
			c.Request("unsubscribe.test.model", nil).GetResponse(t)
		},
	},
}

func TestIndirectSent_SubscriptionReference_ContainsResource(t *testing.T) {
	for _, sentSubscribeType := range testIndirectSentSubscribeTypes {
		runNamedTest(t, sentSubscribeType.Name, func(s *Session) {

			model := resourceData("test.model")
			modelDelayed := `{"name":"delayed"}`
			modelDelayedParent := `{"name":"delayedparent","child":{"rid":"test.model"},"delayed":{"rid":"test.model.delayed"}}`

			c := s.Connect()
			// Subscribe to test.model
			sentSubscribeType.Subscribe(t, s, c)

			// Get parent model
			creq := c.Request("subscribe.test.model.delayedparent", nil)

			// Handle parent get and access request.
			mreqs := s.GetParallelRequests(t, 2)
			mreqs.GetRequest(t, "access.test.model.delayedparent").RespondSuccess(json.RawMessage(`{"get":true}`))
			mreqs.GetRequest(t, "get.test.model.delayedparent").RespondSuccess(json.RawMessage(`{"model":` + modelDelayedParent + `}`))
			// Delay response to get request of referenced test.model.delayed
			mreqsecond := s.GetRequest(t)

			// Call unsubscribe on test.model
			sentSubscribeType.Unsubscribe(t, s, c)

			// Repond to parent get request
			mreqsecond.RespondSuccess(json.RawMessage(`{"model":` + modelDelayed + `}`))

			// Get client response, which should include test.model.
			creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`,"test.model.delayedparent":`+modelDelayedParent+`,"test.model.delayed":`+modelDelayed+`}}`))
		})
	}
}

func TestIndirectSent_ChangeEventReference_ContainsResource(t *testing.T) {
	for _, sentSubscribeType := range testIndirectSentSubscribeTypes {
		runNamedTest(t, sentSubscribeType.Name, func(s *Session) {
			model := resourceData("test.model")
			modelDelayed := `{"name":"delayed"}`
			modelDelayedParent := `{"name":"delayedparent","child":{"rid":"test.model"},"delayed":{"rid":"test.model.delayed"}}`
			modelDelayedGrandParent := `{"name":"delayedgrandparent","parent":null}`

			c := s.Connect()
			// Subscribe to test.model
			sentSubscribeType.Subscribe(t, s, c)

			// Get grandparent model
			creq := c.Request("subscribe.test.model.delayedgrandparent", nil)

			// Handle parent get and access request.
			mreqs := s.GetParallelRequests(t, 2)
			mreqs.GetRequest(t, "access.test.model.delayedgrandparent").RespondSuccess(json.RawMessage(`{"get":true}`))
			mreqs.GetRequest(t, "get.test.model.delayedgrandparent").RespondSuccess(json.RawMessage(`{"model":` + modelDelayedGrandParent + `}`))

			// Get client response.
			creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model.delayedgrandparent":`+modelDelayedGrandParent+`}}`))

			// Send change event, adding parent model reference to delayedgrandparent.
			s.ResourceEvent("test.model.delayedgrandparent", "change", json.RawMessage(`{"values":{"parent":{"rid":"test.model.delayedparent"}}}`))

			// Handle parent get request
			s.GetRequest(t).AssertSubject(t, "get.test.model.delayedparent").RespondSuccess(json.RawMessage(`{"model":` + modelDelayedParent + `}`))

			// Delay response to get request of referenced test.model.delayed
			mreqsecond := s.GetRequest(t)

			// Call unsubscribe on test.model
			sentSubscribeType.Unsubscribe(t, s, c)

			// Repond to parent get request
			mreqsecond.RespondSuccess(json.RawMessage(`{"model":` + modelDelayed + `}`))

			// Get change event, which should include test.model
			c.GetEvent(t).Equals(t, "test.model.delayedgrandparent.change", json.RawMessage(`{"values":{"parent":{"rid":"test.model.delayedparent"}},"models":{"test.model":`+model+`,"test.model.delayedparent":`+modelDelayedParent+`,"test.model.delayed":`+modelDelayed+`}}`))

			// Send event on parent and validate client event
			s.ResourceEvent("test.model.delayedparent", "custom", common.CustomEvent())
			c.GetEvent(t).Equals(t, "test.model.delayedparent.custom", common.CustomEvent())
		})
	}
}

func TestIndirectSent_AddEventReference_ContainsResource(t *testing.T) {
	for _, sentSubscribeType := range testIndirectSentSubscribeTypes {
		runNamedTest(t, sentSubscribeType.Name, func(s *Session) {
			model := resourceData("test.model")
			modelDelayed := `{"name":"delayed"}`
			modelDelayedParent := `{"name":"delayedparent","child":{"rid":"test.model"},"delayed":{"rid":"test.model.delayed"}}`
			collectionDelayedGrandParent := `["delayedgrandparent"]`

			c := s.Connect()
			// Subscribe to test.model
			sentSubscribeType.Subscribe(t, s, c)

			// Get grandparent model
			creq := c.Request("subscribe.test.collection.delayedgrandparent", nil)

			// Handle parent get and access request.
			mreqs := s.GetParallelRequests(t, 2)
			mreqs.GetRequest(t, "access.test.collection.delayedgrandparent").RespondSuccess(json.RawMessage(`{"get":true}`))
			mreqs.GetRequest(t, "get.test.collection.delayedgrandparent").RespondSuccess(json.RawMessage(`{"collection":` + collectionDelayedGrandParent + `}`))

			// Get client response.
			creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{"test.collection.delayedgrandparent":`+collectionDelayedGrandParent+`}}`))

			// Send add event, adding parent model reference to delayedgrandparent.
			s.ResourceEvent("test.collection.delayedgrandparent", "add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model.delayedparent"}}`))

			// Handle parent get request
			s.GetRequest(t).AssertSubject(t, "get.test.model.delayedparent").RespondSuccess(json.RawMessage(`{"model":` + modelDelayedParent + `}`))

			// Delay response to get request of referenced test.model.delayed
			mreqsecond := s.GetRequest(t)

			// Call unsubscribe on test.model
			sentSubscribeType.Unsubscribe(t, s, c)

			// Repond to parent get request
			mreqsecond.RespondSuccess(json.RawMessage(`{"model":` + modelDelayed + `}`))

			// Get add event, which should include test.model
			c.GetEvent(t).Equals(t, "test.collection.delayedgrandparent.add", json.RawMessage(`{"idx":1,"value":{"rid":"test.model.delayedparent"},"models":{"test.model":`+model+`,"test.model.delayedparent":`+modelDelayedParent+`,"test.model.delayed":`+modelDelayed+`}}`))

			// Send event on parent and validate client event
			s.ResourceEvent("test.model.delayedparent", "custom", common.CustomEvent())
			c.GetEvent(t).Equals(t, "test.model.delayedparent.custom", common.CustomEvent())
		})
	}
}

func TestIndirectSent_SubscriptionReferenceToNestedResource_ContainsResource(t *testing.T) {
	runTest(t, func(s *Session) {

		model := resourceData("test.model")
		modelParent := resourceData("test.model.parent")
		modelDelayed := `{"name":"delayed"}`
		modelDelayedGrandParent := `{"name":"delayedGrandparent","child":{"rid":"test.model.parent"},"delayed":{"rid":"test.model.delayed"}}`

		c := s.Connect()
		// Subscribe to test.model.parent, and indirectly to test.model.
		subscribeToTestModelParent(t, s, c, false)

		// Get grant parent model.
		creq := c.Request("subscribe.test.model.delayedGrandparent", nil)

		// Handle parent get and access request.
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model.delayedGrandparent").RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.GetRequest(t, "get.test.model.delayedGrandparent").RespondSuccess(json.RawMessage(`{"model":` + modelDelayedGrandParent + `}`))
		// Delay response to get request of referenced test.model.delayed
		mreqsecond := s.GetRequest(t)

		// Call unsubscribe on test.model.parent
		c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)

		// Repond to parent get request
		mreqsecond.RespondSuccess(json.RawMessage(`{"model":` + modelDelayed + `}`))

		// Get client response, which should include test.model.parent and test.model.
		creq.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`,"test.model.parent":`+modelParent+`,"test.model.delayedGrandparent":`+modelDelayedGrandParent+`,"test.model.delayed":`+modelDelayed+`}}`))
	})
}
