package test

import (
	"encoding/json"
	"testing"
)

// Test handle get requests with cyclic data
func TestCyclicGetTest(t *testing.T) {
	// The following cyclic groups exist
	// a -> a

	// b -> c
	// c -> f

	// d -> e, f
	// e -> d
	// f -> d

	// Other entry points
	// g -> e, f
	// h -> e

	resources := map[string]string{
		"example.a": `{"a":{"rid":"example.a"}}`,

		"example.b": `{"c":{"rid":"example.c"}}`,
		"example.c": `{"b":{"rid":"example.b"}}`,

		"example.d": `{"e":{"rid":"example.e"},"f":{"rid":"example.f"}}`,
		"example.e": `{"d":{"rid":"example.d"}}`,
		"example.f": `{"d":{"rid":"example.d"}}`,

		"example.g": `{"e":{"rid":"example.e"},"f":{"rid":"example.f"}}`,
		"example.h": `{"e":{"rid":"example.e"}}`,
	}

	responses := map[string][]string{
		"example.a": []string{"example.a"},
		"example.b": []string{"example.b", "example.c"},
		"example.d": []string{"example.d", "example.e", "example.f"},
		"example.g": []string{"example.d", "example.e", "example.f", "example.g"},
		"example.h": []string{"example.d", "example.e", "example.f", "example.h"},
	}

	tbl := [][]struct {
		Event string
		RID   string
	}{
		{
			{"subscribe", "example.a"},
			{"access", "example.a"},
			{"get", "example.a"},
			{"response", "example.a"},
		},
		{
			{"subscribe", "example.b"},
			{"access", "example.b"},
			{"get", "example.b"},
			{"get", "example.c"},
			{"response", "example.b"},
		},
		{
			{"subscribe", "example.d"},
			{"access", "example.d"},
			{"get", "example.d"},
			{"get", "example.e"},
			{"get", "example.f"},
			{"response", "example.d"},
		},
		{
			{"subscribe", "example.g"},
			{"access", "example.g"},
			{"get", "example.g"},
			{"get", "example.e"},
			{"get", "example.f"},
			{"get", "example.d"},
			{"response", "example.g"},
		},
		{
			{"subscribe", "example.d"},
			{"access", "example.d"},
			{"get", "example.d"},
			{"subscribe", "example.h"},
			{"access", "example.h"},
			{"get", "example.e"},
			{"get", "example.h"},
			{"get", "example.f"},
			{"response", "example.d"},
			{"response", "example.h"},
		},
	}

	for _, l := range tbl {
		runTest(t, func(s *Session) {
			var creq *ClientRequest
			var req *Request

			c := s.Connect()

			creqs := make(map[string]*ClientRequest)
			reqs := make(map[string]*Request)
			sentModels := make(map[string]bool)

			for _, ev := range l {
				switch ev.Event {
				case "subscribe":
					creqs[ev.RID] = c.Request("subscribe."+ev.RID, nil)
				case "access":
					for req = reqs["access."+ev.RID]; req == nil; req = reqs["access."+ev.RID] {
						treq := s.GetRequest(t)
						reqs[treq.Subject] = treq
					}
					req.RespondSuccess(json.RawMessage(`{"get":true}`))
				case "get":
					for req = reqs["get."+ev.RID]; req == nil; req = reqs["get."+ev.RID] {
						req = s.GetRequest(t)
						reqs[req.Subject] = req
					}
					req.RespondSuccess(json.RawMessage(`{"model":` + resources[ev.RID] + `}`))
				case "response":
					creq = creqs[ev.RID]
					rids := responses[ev.RID]
					str := ""
					for _, rid := range rids {
						if sentModels[rid] {
							continue
						}
						if str != "" {
							str += ","
						}
						str += `"` + rid + `":` + resources[rid]
						sentModels[rid] = true
					}
					str = `{"models":{` + str + `}}`
					creq.GetResponse(t).AssertResult(t, json.RawMessage(str))
				}
			}
		})
	}
}
