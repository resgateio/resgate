package test

import (
	"encoding/json"
	"testing"
)

func BenchmarkCallRequestWithNilParamsOnSubscribedModel(b *testing.B) {
	s := setup(nil)
	c := s.Connect()

	model := resourceData("test.model")
	// Subscribe to resource
	creq := c.Request("subscribe.test.model", nil)
	// Handle model get and access request
	mreqs := s.GetParallelRequests(nil, 2)
	req := mreqs.GetRequest(nil, "access.test.model")
	req.RespondSuccess(`{"get":true,"call":"*"}`)
	req = mreqs.GetRequest(nil, "get.test.model")
	req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

	// Get client response
	creq.GetResponse(nil)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Send client call request
		creq = c.Request("call.test.model.method", nil)
		// Get call request
		s.GetRequest(nil).RespondSuccess(`{"foo":"bar"}`)
		creq.GetResponse(nil)
	}

	teardown(s)
}

func BenchmarkCallRequestWithNilParamsOnSubscribedModelParallel(b *testing.B) {
	s := setup(nil)
	c := s.Connect()

	model := resourceData("test.model")
	// Subscribe to resource
	creq := c.Request("subscribe.test.model", nil)
	// Handle model get and access request
	mreqs := s.GetParallelRequests(nil, 2)
	req := mreqs.GetRequest(nil, "access.test.model")
	req.RespondSuccess(`{"get":true,"call":"*"}`)
	req = mreqs.GetRequest(nil, "get.test.model")
	req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

	// Get client response
	creq.GetResponse(nil)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Send client call request
			cpreq := c.Request("call.test.model.method", nil)
			// Get call request
			s.GetRequest(nil).RespondSuccess(`{"foo":"bar"}`)
			cpreq.GetResponse(nil)
		}
	})

	teardown(s)
}
