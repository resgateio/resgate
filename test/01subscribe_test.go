package test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/resgateio/resgate/server"
	"github.com/resgateio/resgate/server/mq"
	"github.com/resgateio/resgate/server/reserr"
)

// Test that a get- and access-request are sent to NATS on client subscribe
func TestGetAndAccessOnSubscribe(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		c.Request("subscribe.test.model", nil)
		mreqs := s.GetParallelRequests(t, 2)

		// Validate get request
		mreqs.GetRequest(t, "get.test.model").
			AssertPayload(t, json.RawMessage(`{}`))
		// Validate access request
		mreqs.GetRequest(t, "access.test.model").
			AssertPathPayload(t, "token", json.RawMessage(`null`)).
			AssertPathMissing(t, "isHttp")
	})
}

// Test responses to client get and subscribe requests
func TestResponseOnPrimitiveModelRetrieval(t *testing.T) {
	model := resourceData("test.model")
	brokenModel := json.RawMessage(`{"foo":"bar"}`)
	brokenAccess := json.RawMessage(`{"get":"foo"}`)
	modelGetResponse := json.RawMessage(`{"model":` + model + `}`)
	fullAccess := json.RawMessage(`{"get":true}`)
	noAccess := json.RawMessage(`{"get":false}`)
	modelClientResponse := json.RawMessage(`{"models":{"test.resource":` + model + `}}`)

	// *reserr.Error implies an error response. requestTimeout implies a timeout. Otherwise success.
	tbl := []struct {
		GetResponse    interface{}
		AccessResponse interface{}
		Expected       interface{}
	}{
		{modelGetResponse, fullAccess, modelClientResponse},
		{modelGetResponse, noAccess, reserr.ErrAccessDenied},
		{modelGetResponse, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{modelGetResponse, reserr.ErrInternalError, reserr.ErrInternalError},
		{modelGetResponse, requestTimeout, mq.ErrRequestTimeout},
		{modelGetResponse, brokenAccess, reserr.CodeInternalError},
		{reserr.ErrNotFound, fullAccess, reserr.ErrNotFound},
		{reserr.ErrNotFound, noAccess, reserr.ErrAccessDenied},
		{reserr.ErrNotFound, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{reserr.ErrNotFound, reserr.ErrInternalError, reserr.ErrInternalError},
		{reserr.ErrNotFound, requestTimeout, mq.ErrRequestTimeout},
		{reserr.ErrNotFound, brokenAccess, reserr.CodeInternalError},
		{reserr.ErrInternalError, fullAccess, reserr.ErrInternalError},
		{reserr.ErrInternalError, noAccess, reserr.ErrAccessDenied},
		{reserr.ErrInternalError, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{reserr.ErrInternalError, reserr.ErrDisposing, reserr.ErrDisposing},
		{reserr.ErrInternalError, requestTimeout, mq.ErrRequestTimeout},
		{reserr.ErrInternalError, brokenAccess, reserr.CodeInternalError},
		{requestTimeout, fullAccess, mq.ErrRequestTimeout},
		{requestTimeout, noAccess, reserr.ErrAccessDenied},
		{requestTimeout, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{requestTimeout, reserr.ErrInternalError, reserr.ErrInternalError},
		{requestTimeout, requestTimeout, mq.ErrRequestTimeout},
		{requestTimeout, brokenAccess, reserr.CodeInternalError},
		{brokenModel, fullAccess, reserr.CodeInternalError},
		{brokenModel, noAccess, reserr.ErrAccessDenied},
		{brokenModel, reserr.ErrAccessDenied, reserr.ErrAccessDenied},
		{brokenModel, reserr.ErrInternalError, reserr.ErrInternalError},
		{brokenModel, requestTimeout, mq.ErrRequestTimeout},
		{brokenModel, brokenAccess, reserr.CodeInternalError},
		// Untrimmed whitespaces
		{json.RawMessage("\r\n \t{ \"model\":\t \r\n" + model + "\t} \t\n"), fullAccess, modelClientResponse},
		{modelGetResponse, json.RawMessage("\r\n \t{\"get\":\ttrue}\n"), modelClientResponse},
		// Invalid get response
		{[]byte(`{"invalid":JSON}`), fullAccess, reserr.CodeInternalError},
		{[]byte(``), fullAccess, reserr.CodeInternalError},
		{[]byte(`{"invalid":"response"}`), fullAccess, reserr.CodeInternalError},
		// Invalid access response
		{modelGetResponse, []byte(`{"invalid":JSON}`), reserr.CodeInternalError},
		{modelGetResponse, []byte(``), reserr.CodeInternalError},
		{modelGetResponse, []byte(`{"invalid":"response"}`), reserr.CodeInternalError},
		// Invalid get model or collection response
		{json.RawMessage(`{"model":["with","array","data"]}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"collection":{"with":"model data"}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"array":[1,2]}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"prop":{"action":"delete"}}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"prop":{"action":"unknown"}}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"prop":{"unknown":"property"}}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"child":{"rid":false}}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"model":{"child":{"rid":false}}}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"collection":["array",[1,2]]}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"collection":["prop",{"action":"delete"}]}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"collection":["prop",{"action":"unknown"}]}`), fullAccess, reserr.CodeInternalError},
		{json.RawMessage(`{"collection":["prop",{"unknown":"property"}]}`), fullAccess, reserr.CodeInternalError},
		// Multiple resource types
		{json.RawMessage(`{"model":{"foo":"bar"},"collection":[1,2,3]}`), fullAccess, reserr.CodeInternalError},
		// Invalid get error response
		{[]byte(`{"error":[]}`), fullAccess, reserr.CodeInternalError},
		{[]byte(`{"error":{"message":"missing code"}}`), fullAccess, ""},
		{[]byte(`{"error":{"code":12,"message":"integer code"}}`), fullAccess, reserr.CodeInternalError},
	}

	for i, l := range tbl {
		// Run both "get" and "subscribe" tests
		for _, method := range []string{"get", "subscribe"} {
			// Run both orders for "get" and "access" response
			for getFirst := true; getFirst; getFirst = false {
				s := fmt.Sprintf("#%d when using %#v", i+1, method)
				if getFirst {
					s += " with get response sent first"
				} else {
					s += " with access response sent first"
				}
				runNamedTest(t, s, func(s *Session) {
					c := s.Connect()
					var creq *ClientRequest
					switch method {
					case "get":
						creq = c.Request("get.test.resource", nil)
					case "subscribe":
						creq = c.Request("subscribe.test.resource", nil)
					}
					mreqs := s.GetParallelRequests(t, 2)

					var req *Request
					if getFirst {
						// Send get response
						req = mreqs.GetRequest(t, "get.test.resource")
						if l.GetResponse == requestTimeout {
							req.Timeout()
						} else if err, ok := l.GetResponse.(*reserr.Error); ok {
							req.RespondError(err)
						} else if raw, ok := l.GetResponse.([]byte); ok {
							req.RespondRaw(raw)
						} else {
							req.RespondSuccess(l.GetResponse)
						}
					}

					// Send access response
					req = mreqs.GetRequest(t, "access.test.resource")
					if l.AccessResponse == requestTimeout {
						req.Timeout()
					} else if err, ok := l.AccessResponse.(*reserr.Error); ok {
						req.RespondError(err)
					} else if raw, ok := l.AccessResponse.([]byte); ok {
						req.RespondRaw(raw)
					} else {
						req.RespondSuccess(l.AccessResponse)
					}

					if !getFirst {
						// Send get response
						req := mreqs.GetRequest(t, "get.test.resource")
						if l.GetResponse == nil {
							req.Timeout()
						} else if err, ok := l.GetResponse.(*reserr.Error); ok {
							req.RespondError(err)
						} else if raw, ok := l.GetResponse.([]byte); ok {
							req.RespondRaw(raw)
						} else {
							req.RespondSuccess(l.GetResponse)
						}
					}

					// Validate client response
					cresp := creq.GetResponse(t)
					if err, ok := l.Expected.(*reserr.Error); ok {
						cresp.AssertError(t, err)
					} else if code, ok := l.Expected.(string); ok {
						cresp.AssertErrorCode(t, code)
					} else {
						cresp.AssertResult(t, l.Expected)
					}
				})
			}
		}
	}
}

// Test handle subscribe requests
func TestSubscribe(t *testing.T) {
	event := json.RawMessage(`{"foo":"bar"}`)

	responses := map[string]map[string][]struct {
		RID      string
		Resource *resource
	}{
		versionLatest: {
			// Model responses
			"test.model":              {{"test.model", nil}},
			"test.model.parent":       {{"test.model.parent", nil}, {"test.model", nil}},
			"test.model.grandparent":  {{"test.model.grandparent", nil}, {"test.model.parent", nil}, {"test.model", nil}},
			"test.model.secondparent": {{"test.model.secondparent", nil}, {"test.model", nil}},
			"test.model.brokenchild":  {{"test.model.brokenchild", nil}, {"test.err.notFound", nil}},
			"test.model.soft":         {{"test.model.soft", nil}},
			"test.model.soft.parent":  {{"test.model.soft.parent", nil}, {"test.model.soft", nil}},
			"test.model.data":         {{"test.model.data", &resource{typeModel, `{"name":"data","primitive":12,"object":{"data":{"foo":["bar"]}},"array":{"data":[{"foo":"bar"}]}}`, nil}}},
			"test.model.data.parent":  {{"test.model.data.parent", nil}, {"test.model.data", &resource{typeModel, `{"name":"data","primitive":12,"object":{"data":{"foo":["bar"]}},"array":{"data":[{"foo":"bar"}]}}`, nil}}},
			// Cyclic model responses
			"test.m.a": {{"test.m.a", nil}},
			"test.m.b": {{"test.m.b", nil}, {"test.m.c", nil}},
			"test.m.d": {{"test.m.d", nil}, {"test.m.e", nil}, {"test.m.f", nil}},
			"test.m.g": {{"test.m.d", nil}, {"test.m.e", nil}, {"test.m.f", nil}, {"test.m.g", nil}},
			"test.m.h": {{"test.m.d", nil}, {"test.m.e", nil}, {"test.m.f", nil}, {"test.m.h", nil}},
			// Collection responses
			"test.collection":              {{"test.collection", nil}},
			"test.collection.parent":       {{"test.collection.parent", nil}, {"test.collection", nil}},
			"test.collection.grandparent":  {{"test.collection.grandparent", nil}, {"test.collection.parent", nil}, {"test.collection", nil}},
			"test.collection.secondparent": {{"test.collection.secondparent", nil}, {"test.collection", nil}},
			"test.collection.brokenchild":  {{"test.collection.brokenchild", nil}, {"test.err.notFound", nil}},
			"test.collection.soft":         {{"test.collection.soft", nil}},
			"test.collection.soft.parent":  {{"test.collection.soft.parent", nil}, {"test.collection.soft", nil}},
			"test.collection.data":         {{"test.collection.data", &resource{typeCollection, `["data",12,{"data":{"foo":["bar"]}},{"data":[{"foo":"bar"}]}]`, nil}}},
			"test.collection.data.parent":  {{"test.collection.data.parent", nil}, {"test.collection.data", &resource{typeCollection, `["data",12,{"data":{"foo":["bar"]}},{"data":[{"foo":"bar"}]}]`, nil}}},
			// Cyclic collection responses
			"test.c.a": {{"test.c.a", nil}},
			"test.c.b": {{"test.c.b", nil}, {"test.c.c", nil}},
			"test.c.d": {{"test.c.d", nil}, {"test.c.e", nil}, {"test.c.f", nil}},
			"test.c.g": {{"test.c.d", nil}, {"test.c.e", nil}, {"test.c.f", nil}, {"test.c.g", nil}},
			"test.c.h": {{"test.c.d", nil}, {"test.c.e", nil}, {"test.c.f", nil}, {"test.c.h", nil}},
		},
		"1.2.0": {
			// Model responses
			"test.model.soft":        {{"test.model.soft", &resource{typeModel, `{"name":"soft","child":"test.model"}`, nil}}},
			"test.model.soft.parent": {{"test.model.soft.parent", nil}, {"test.model.soft", &resource{typeModel, `{"name":"soft","child":"test.model"}`, nil}}},
			"test.model.data":        {{"test.model.data", &resource{typeModel, `{"name":"data","primitive":12,"object":"[Data]","array":"[Data]"}`, nil}}},
			"test.model.data.parent": {{"test.model.data.parent", nil}, {"test.model.data", &resource{typeModel, `{"name":"data","primitive":12,"object":"[Data]","array":"[Data]"}`, nil}}},
			// Collection responses
			"test.collection.soft":        {{"test.collection.soft", &resource{typeCollection, `["soft","test.collection"]`, nil}}},
			"test.collection.soft.parent": {{"test.collection.soft.parent", nil}, {"test.collection.soft", &resource{typeCollection, `["soft","test.collection"]`, nil}}},
			"test.collection.data":        {{"test.collection.data", &resource{typeCollection, `["data",12,"[Data]","[Data]"]`, nil}}},
			"test.collection.data.parent": {{"test.collection.data.parent", nil}, {"test.collection.data", &resource{typeCollection, `["data",12,"[Data]","[Data]"]`, nil}}},
		},
	}

	for _, set := range sequenceSets {
		for i, l := range set.Table {
			runNamedTest(t, fmt.Sprintf("#%d for client version %s", i+1, set.Version), func(s *Session) {
				var creq *ClientRequest
				var req *Request

				c := s.ConnectWithVersion(set.Version)

				creqs := make(map[string]*ClientRequest)
				reqs := make(map[string]*Request)
				sentResources := make(map[string]bool)

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
					case "accessDenied":
						for req = reqs["access."+ev.RID]; req == nil; req = reqs["access."+ev.RID] {
							treq := s.GetRequest(t)
							reqs[treq.Subject] = treq
						}
						req.RespondSuccess(json.RawMessage(`{"get":false}`))
					case "get":
						for req = reqs["get."+ev.RID]; req == nil; req = reqs["get."+ev.RID] {
							req = s.GetRequest(t)
							reqs[req.Subject] = req
						}
						rsrc := resources[ev.RID]
						switch rsrc.typ {
						case typeModel:
							req.RespondSuccess(json.RawMessage(`{"model":` + rsrc.data + `}`))
						case typeCollection:
							req.RespondSuccess(json.RawMessage(`{"collection":` + rsrc.data + `}`))
						case typeError:
							req.RespondError(rsrc.err)
						}
					case "response":
						creq = creqs[ev.RID]
						respResources := responses[set.Version][ev.RID]
						models := make(map[string]json.RawMessage)
						collections := make(map[string]json.RawMessage)
						errors := make(map[string]*reserr.Error)
						for _, rr := range respResources {
							if sentResources[rr.RID] {
								continue
							}
							var rsrc resource
							if rr.Resource == nil {
								rsrc = resources[rr.RID]
							} else {
								rsrc = *rr.Resource
							}
							switch rsrc.typ {
							case typeModel:
								models[rr.RID] = json.RawMessage(rsrc.data)
							case typeCollection:
								collections[rr.RID] = json.RawMessage(rsrc.data)
							case typeError:
								errors[rr.RID] = rsrc.err
							}
							sentResources[rr.RID] = true
						}
						m := make(map[string]interface{})
						if len(models) > 0 {
							m["models"] = models
						}
						if len(collections) > 0 {
							m["collections"] = collections
						}
						if len(errors) > 0 {
							m["errors"] = errors
						}
						creq.GetResponse(t).AssertResult(t, m)
					case "errorResponse":
						creq = creqs[ev.RID]
						creq.GetResponse(t).AssertIsError(t)
					case "event":
						s.ResourceEvent(ev.RID, "custom", event)
						c.GetEvent(t).Equals(t, ev.RID+".custom", event)
					case "noevent":
						s.ResourceEvent(ev.RID, "custom", event)
						c.AssertNoEvent(t, ev.RID)
					case "nosubscription":
						s.NoSubscriptions(t, ev.RID)
					}
				}
			})
		}
	}
}

// Test that a response with linked models is sent to the client on a client
// subscribe request when access response is delayed
func TestResponseOnLinkedModelSubscribeWithDelayedAccess(t *testing.T) {
	// Run multiple times as the bug GH-84 is caused by race conditions
	// and won't always be triggered.
	for i := 0; i < 100; i++ {
		runTest(t, func(s *Session) {
			c := s.Connect()
			subscribeToTestModelParentExt(t, s, c, false, true)
		})
	}
}

func TestSubscribe_WithCIDPlaceholder_ReplacesCID(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")
		event := json.RawMessage(`{"foo":"bar"}`)

		c := s.Connect()
		cid := getCID(t, s, c)

		creq := c.Request("subscribe.test.{cid}.model", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		req := mreqs.GetRequest(t, "access.test."+cid+".model")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		req = mreqs.GetRequest(t, "get.test."+cid+".model")
		req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		// Validate client response
		creq.GetResponse(t)

		// Send event on model and validate client did get event
		s.ResourceEvent("test."+cid+".model", "custom", event)
		c.GetEvent(t).AssertEventName(t, "test.{cid}.model.custom")
	})
}

func TestSubscribe_MultipleClientsSubscribingResource_FetchedFromCache(t *testing.T) {
	tbl := []struct {
		RID string
	}{
		{"test.model"},
		{"test.collection"},
	}
	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			rid := l.RID
			c1 := s.Connect()
			subscribeToResource(t, s, c1, rid)

			// Connect with second client
			c2 := s.Connect()
			// Send subscribe request
			c2req := c2.Request("subscribe."+rid, nil)
			s.GetRequest(t).
				AssertSubject(t, "access."+rid).
				RespondSuccess(json.RawMessage(`{"get":true}`))
			// Handle resource and validate client response
			rsrc, ok := resources[rid]
			if !ok {
				panic("no resource named " + rid)
			}
			var r string
			if rsrc.typ == typeError {
				b, _ := json.Marshal(rsrc.err)
				r = string(b)
			} else {
				r = rsrc.data
			}
			switch rsrc.typ {
			case typeModel:
				c2req.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"`+rid+`":`+r+`}}`))
			case typeCollection:
				c2req.GetResponse(t).AssertResult(t, json.RawMessage(`{"collections":{"`+rid+`":`+r+`}}`))
			default:
				panic("invalid type")
			}
		})
	}
}

func TestSubscribe_LongResourceID_ReturnsErrSubjectTooLong(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		c.Request("subscribe.test."+generateString(10000), nil).
			GetResponse(t).
			AssertError(t, reserr.ErrSubjectTooLong)
	})
}

func TestSubscribe_WithThrottle_ThrottlesRequests(t *testing.T) {
	const referenceCount = 10
	const referenceThrottle = 3
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Get subscription with a set of references
		data := "["
		for i := 1; i <= referenceCount; i++ {
			if i > 1 {
				data += ","
			}
			data += fmt.Sprintf(`{"rid":"test.model.%d"}`, i)
		}
		data += "]"

		// Send subscribe request for the collection
		creq := c.Request("subscribe.test.collection", nil)
		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		// Handle access
		req := mreqs.GetRequest(t, "access.test.collection")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.GetRequest(t, "get.test.collection").RespondSuccess(json.RawMessage(`{"collection":` + data + `}`))

		// Get throttled number of requests
		mreqs = s.GetParallelRequests(t, referenceThrottle)
		requestCount := referenceThrottle
		// Assert no other requests are sent
		for i := 1; i <= referenceCount; i++ {
			c.AssertNoNATSRequest(t, fmt.Sprintf("test.model.%d", i))
		}
		// Respond to requests one by one
		for len(mreqs) > 0 {
			r := mreqs[0]
			mreqs = mreqs[1:]
			id := r.Subject[strings.LastIndexByte(r.Subject, '.')+1:]
			r.RespondSuccess(json.RawMessage(`{"model":` + fmt.Sprintf(`{"id":%s}`, id) + `}`))
			// If we still have remaining subscriptions not yet received
			if requestCount < referenceCount {
				// For each response, a new request should be sent.
				req := s.GetRequest(t)
				mreqs = append(mreqs, req)
				requestCount++
				// Assert no other requests are sent
				for i := 1; i <= referenceCount; i++ {
					c.AssertNoNATSRequest(t, fmt.Sprintf("test.model.%d", i))
				}
			}
		}

		// Get the response to the client request
		creq.GetResponse(t)

	}, func(c *server.Config) {
		c.ReferenceThrottle = referenceThrottle
	})
}

func TestSubscribe_WithThrottleOnNestedReferences_ThrottlesRequests(t *testing.T) {
	const referenceCount = 10
	const referenceThrottle = 3
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Get subscription with a set of references
		data := "["
		for i := 1; i <= referenceCount; i++ {
			if i > 1 {
				data += ","
			}
			data += fmt.Sprintf(`{"rid":"test.model.%d"}`, i)
		}
		data += "]"

		// Send subscribe request for the collection
		creq := c.Request("subscribe.test.collection", nil)
		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		// Handle access
		req := mreqs.GetRequest(t, "access.test.collection")
		req.RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.GetRequest(t, "get.test.collection").RespondSuccess(json.RawMessage(`{"collection":` + data + `}`))

		// Get throttled number of requests
		mreqs = s.GetParallelRequests(t, referenceThrottle)
		requestCount := referenceThrottle
		// Assert no other requests are sent
		for i := 1; i <= referenceCount; i++ {
			c.AssertNoNATSRequest(t, fmt.Sprintf("test.model.%d", i))
			c.AssertNoNATSRequest(t, fmt.Sprintf("test.model.%d.child", i))
		}
		// Respond to requests one by one
		for len(mreqs) > 0 {
			r := mreqs[0]
			mreqs = mreqs[1:]
			id := r.Subject[strings.LastIndexByte(r.Subject, '.')+1:]
			if id == "child" {
				subj := r.Subject[:len(r.Subject)-len(id)-1]
				id = subj[strings.LastIndexByte(subj, '.')+1:]
				r.RespondSuccess(json.RawMessage(`{"model":` + fmt.Sprintf(`{"id":%s,"isChild":true}`, id) + `}`))
			} else {
				r.RespondSuccess(json.RawMessage(`{"model":` + fmt.Sprintf(`{"id":%s,"child":{"rid":"test.model.%s.child"}}`, id, id) + `}`))
			}
			// If we still have remaining subscriptions not yet received
			if requestCount < referenceCount*2 {
				// For each response, a new request should be sent.
				req := s.GetRequest(t)
				mreqs = append(mreqs, req)
				requestCount++
				// Assert no other requests are sent
				for i := 1; i <= referenceCount; i++ {
					c.AssertNoNATSRequest(t, fmt.Sprintf("test.model.%d", i))
					c.AssertNoNATSRequest(t, fmt.Sprintf("test.model.%d.child", i))
				}
			}
		}

		// Get the response to the client request
		creq.GetResponse(t)

	}, func(c *server.Config) {
		c.ReferenceThrottle = referenceThrottle
	})
}

// Test that two connections subscribing to the same model, both waiting for the
// response of the get request, gets the returned resource.
func TestSubscribe_MultipleSubscribersOnPendingModel_ModelSentToAllSubscribers(t *testing.T) {
	model := resourceData("test.model")

	runTest(t, func(s *Session) {
		c1 := s.Connect()
		c2 := s.Connect()
		// Subscribe with client 1
		creq1 := c1.Request("subscribe.test.model", nil)
		mreqs1 := s.GetParallelRequests(t, 2)
		// Handle access request
		mreqs1.GetRequest(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true}`))
		getreq := mreqs1.GetRequest(t, "get.test.model")

		// Subscribe with client 2
		creq2 := c2.Request("subscribe.test.model", nil)
		// Handle access request
		s.GetRequest(t).
			AssertSubject(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true}`))

		// Handle get request
		getreq.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate client 1 response and validate
		creq1.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))
		// Validate client 2 response and validate
		creq2.GetResponse(t).AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))
	})
}

// Test that two connections subscribing to the same model, both waiting for the
// response of the get request, gets the returned error.
func TestSubscribe_MultipleSubscribersOnPendingError_ErrorSentToAllSubscribers(t *testing.T) {
	runTest(t, func(s *Session) {
		c1 := s.Connect()
		c2 := s.Connect()
		// Subscribe with client 1
		creq1 := c1.Request("subscribe.test.model", nil)
		mreqs1 := s.GetParallelRequests(t, 2)
		// Handle access request
		mreqs1.GetRequest(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true}`))
		getreq := mreqs1.GetRequest(t, "get.test.model")

		// Subscribe with client 2
		creq2 := c2.Request("subscribe.test.model", nil)
		// Handle access request
		s.GetRequest(t).
			AssertSubject(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true}`))

		// Handle get request
		getreq.RespondError(reserr.ErrNotFound)

		// Validate client 1 response and validate
		creq1.GetResponse(t).AssertError(t, reserr.ErrNotFound)
		// Validate client 2 response and validate
		creq2.GetResponse(t).AssertError(t, reserr.ErrNotFound)
	})
}
