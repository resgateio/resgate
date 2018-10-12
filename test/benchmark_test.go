package test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func BenchmarkCallRequestWithNilParamsOnSubscribedModel(b *testing.B) {
	s := setup()
	c := s.Connect()

	model := resource["test.model"]
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
	s := setup()
	c := s.Connect()

	model := resource["test.model"]
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

func BenchmarkCustomEventSubscribers1(b *testing.B) {
	benchmarkCustomEvent(b, 1)
}

func BenchmarkCustomEventSubscribers10(b *testing.B) {
	benchmarkCustomEvent(b, 10)
}

func BenchmarkCustomEventSubscribers100(b *testing.B) {
	benchmarkCustomEvent(b, 100)
}

func BenchmarkCustomEventSubscribers1000(b *testing.B) {
	benchmarkCustomEvent(b, 1000)
}

func BenchmarkCustomEventSubscribers10000(b *testing.B) {
	benchmarkCustomEvent(b, 10000)
}

func benchmarkCustomEvent(b *testing.B, subscribers int) {
	var s *Session
	panicked := true
	defer func() {
		if panicked {
			fmt.Printf("Trace log:\n%s", s.l)
		}
	}()

	s = setup()
	cs := make([]*Conn, subscribers)
	evs := make(chan *ClientEvent, subscribers+10)
	model := resource["test.model"]
	// customEvent := json.RawMessage(`{"foo":"bar"}`)

	for i := 0; i < subscribers; i++ {
		c := s.ConnectWithChannel(evs)
		cs[i] = c
		// Subscribe to resource
		creq := c.Request("subscribe.test.model", nil)
		if i == 0 {
			// Handle model get and access request
			mreqs := s.GetParallelRequests(nil, 2)
			req := mreqs.GetRequest(nil, "access.test.model")
			req.RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))
			req = mreqs.GetRequest(nil, "get.test.model")
			req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		} else {
			s.
				GetRequest(nil).
				AssertSubject(nil, "access.test.model").
				RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))
		}
		// Get client response
		creq.GetResponse(nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Send event on model and validate client event
		s.ResourceEvent("test.model", "custom", nil)
		// Wait for all the events
		for i := 0; i < subscribers; i++ {
			select {
			case ev := <-evs:
				ev.AssertEventName(nil, "test.model.custom")
			case <-time.After(timeoutSeconds * time.Second):
				panic("expected a client event but found none")
			}
		}
	}

	teardown(s)
	panicked = false
}
