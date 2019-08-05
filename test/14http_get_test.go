package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/resgateio/resgate/server"
	"github.com/resgateio/resgate/server/reserr"
)

// Test invalid urls for HTTP get requests
func TestHTTPGetInvalidURLs(t *testing.T) {
	tbl := []struct {
		URL          string // Url path
		ExpectedCode int
		Expected     interface{}
	}{
		{"/wrong_prefix/test/model", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/test.model", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/test/model/", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/test//model", http.StatusNotFound, reserr.ErrNotFound},
		{"/api/test/mådel/action", http.StatusNotFound, reserr.ErrNotFound},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			hreq := s.HTTPRequest("GET", l.URL, nil)
			hresp := hreq.
				GetResponse(t).
				AssertStatusCode(t, l.ExpectedCode)

			if l.Expected != nil {
				if err, ok := l.Expected.(*reserr.Error); ok {
					hresp.AssertError(t, err)
				} else if code, ok := l.Expected.(string); ok {
					hresp.AssertErrorCode(t, code)
				} else {
					hresp.AssertBody(t, l.Expected)
				}
			}
		})
	}
}

// Test handle HTTP get requests
func TestHTTPGet(t *testing.T) {
	encodings := []struct {
		APIEncoding string
		Responses   map[string]string
	}{
		{
			"json",
			map[string]string{
				// Model responses
				"test.model":              resourceData("test.model"),
				"test.model.parent":       `{"name":"parent","child":{"href":"/api/test/model","model":` + resourceData("test.model") + `}}`,
				"test.model.grandparent":  `{"name":"grandparent","child":{"href":"/api/test/model/parent","model":{"name":"parent","child":{"href":"/api/test/model","model":` + resourceData("test.model") + `}}}}`,
				"test.model.secondparent": `{"name":"secondparent","child":{"href":"/api/test/model","model":` + resourceData("test.model") + `}}`,
				"test.model.brokenchild":  `{"name":"brokenchild","child":{"href":"/api/test/err/notFound","error":` + resourceData("test.err.notFound") + `}}`,
				"test.m.a":                `{"a":{"href":"/api/test/m/a"}}`,
				"test.m.b":                `{"c":{"href":"/api/test/m/c","model":{"b":{"href":"/api/test/m/b"}}}}`,
				"test.m.d":                `{"e":{"href":"/api/test/m/e","model":{"d":{"href":"/api/test/m/d"}}},"f":{"href":"/api/test/m/f","model":{"d":{"href":"/api/test/m/d"}}}}`,
				"test.m.g":                `{"e":{"href":"/api/test/m/e","model":{"d":{"href":"/api/test/m/d","model":{"e":{"href":"/api/test/m/e"},"f":{"href":"/api/test/m/f","model":{"d":{"href":"/api/test/m/d"}}}}}}},"f":{"href":"/api/test/m/f","model":{"d":{"href":"/api/test/m/d","model":{"e":{"href":"/api/test/m/e","model":{"d":{"href":"/api/test/m/d"}}},"f":{"href":"/api/test/m/f"}}}}}}`,
				"test.m.h":                `{"e":{"href":"/api/test/m/e","model":{"d":{"href":"/api/test/m/d","model":{"e":{"href":"/api/test/m/e"},"f":{"href":"/api/test/m/f","model":{"d":{"href":"/api/test/m/d"}}}}}}}}`,
				// Collection responses
				"test.collection":              resourceData("test.collection"),
				"test.collection.parent":       `["parent",{"href":"/api/test/collection","collection":` + resourceData("test.collection") + `}]`,
				"test.collection.grandparent":  `["grandparent",{"href":"/api/test/collection/parent","collection":["parent",{"href":"/api/test/collection","collection":` + resourceData("test.collection") + `}]}]`,
				"test.collection.secondparent": `["secondparent",{"href":"/api/test/collection","collection":` + resourceData("test.collection") + `}]`,
				"test.collection.brokenchild":  `["brokenchild",{"href":"/api/test/err/notFound","error":` + resourceData("test.err.notFound") + `}]`,
				"test.c.a":                     `[{"href":"/api/test/c/a"}]`,
				"test.c.b":                     `[{"href":"/api/test/c/c","collection":[{"href":"/api/test/c/b"}]}]`,
				"test.c.d":                     `[{"href":"/api/test/c/e","collection":[{"href":"/api/test/c/d"}]},{"href":"/api/test/c/f","collection":[{"href":"/api/test/c/d"}]}]`,
				"test.c.g":                     `[{"href":"/api/test/c/e","collection":[{"href":"/api/test/c/d","collection":[{"href":"/api/test/c/e"},{"href":"/api/test/c/f","collection":[{"href":"/api/test/c/d"}]}]}]},{"href":"/api/test/c/f","collection":[{"href":"/api/test/c/d","collection":[{"href":"/api/test/c/e","collection":[{"href":"/api/test/c/d"}]},{"href":"/api/test/c/f"}]}]}]`,
				"test.c.h":                     `[{"href":"/api/test/c/e","collection":[{"href":"/api/test/c/d","collection":[{"href":"/api/test/c/e"},{"href":"/api/test/c/f","collection":[{"href":"/api/test/c/d"}]}]}]}]`,
			},
		},
		{
			"jsonFlat",
			map[string]string{
				// Model responses
				"test.model":              resourceData("test.model"),
				"test.model.parent":       `{"name":"parent","child":` + resourceData("test.model") + `}`,
				"test.model.grandparent":  `{"name":"grandparent","child":{"name":"parent","child":` + resourceData("test.model") + `}}`,
				"test.model.secondparent": `{"name":"secondparent","child":` + resourceData("test.model") + `}`,
				"test.model.brokenchild":  `{"name":"brokenchild","child":` + resourceData("test.err.notFound") + `}`,
				"test.m.a":                `{"a":{"href":"/api/test/m/a"}}`,
				"test.m.b":                `{"c":{"b":{"href":"/api/test/m/b"}}}`,
				"test.m.d":                `{"e":{"d":{"href":"/api/test/m/d"}},"f":{"d":{"href":"/api/test/m/d"}}}`,
				"test.m.g":                `{"e":{"d":{"e":{"href":"/api/test/m/e"},"f":{"d":{"href":"/api/test/m/d"}}}},"f":{"d":{"e":{"d":{"href":"/api/test/m/d"}},"f":{"href":"/api/test/m/f"}}}}`,
				"test.m.h":                `{"e":{"d":{"e":{"href":"/api/test/m/e"},"f":{"d":{"href":"/api/test/m/d"}}}}}`,
				// Collection responses
				"test.collection":              resourceData("test.collection"),
				"test.collection.parent":       `["parent",` + resourceData("test.collection") + `]`,
				"test.collection.grandparent":  `["grandparent",["parent",` + resourceData("test.collection") + `]]`,
				"test.collection.secondparent": `["secondparent",` + resourceData("test.collection") + `]`,
				"test.collection.brokenchild":  `["brokenchild",` + resourceData("test.err.notFound") + `]`,
				"test.c.a":                     `[{"href":"/api/test/c/a"}]`,
				"test.c.b":                     `[[{"href":"/api/test/c/b"}]]`,
				"test.c.d":                     `[[{"href":"/api/test/c/d"}],[{"href":"/api/test/c/d"}]]`,
				"test.c.g":                     `[[[{"href":"/api/test/c/e"},[{"href":"/api/test/c/d"}]]],[[[{"href":"/api/test/c/d"}],{"href":"/api/test/c/f"}]]]`,
				"test.c.h":                     `[[[{"href":"/api/test/c/e"},[{"href":"/api/test/c/d"}]]]]`,
			},
		},
	}

	for _, enc := range encodings {
		for i, l := range sequenceTable {
			runNamedTest(t, fmt.Sprintf("#%d with APIEncoding %#v", i+1, enc.APIEncoding), func(s *Session) {
				var hreq *HTTPRequest
				var req *Request

				hreqs := make(map[string]*HTTPRequest)
				reqs := make(map[string]*Request)

				for _, ev := range l {
					switch ev.Event {
					case "subscribe":
						url := "/api/" + strings.Replace(ev.RID, ".", "/", -1)
						hreqs[ev.RID] = s.HTTPRequest("GET", url, nil)
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
						hreq = hreqs[ev.RID]
						hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(enc.Responses[ev.RID]))
					case "errorResponse":
						hreq = hreqs[ev.RID]
						hreq.GetResponse(t).AssertIsError(t)
					}
				}
			}, func(c *server.Config) {
				c.APIEncoding = enc.APIEncoding
			})
		}
	}
}

// Test getting a primitive query model with a HTTP GET request
func TestHTTPGetOnPrimitiveQueryModel(t *testing.T) {
	encodings := []string{"json", "jsonFlat"}

	for _, enc := range encodings {
		runNamedTest(t, enc, func(s *Session) {
			model := resourceData("test.model")

			hreq := s.HTTPRequest("GET", "/api/test/model?q=foo&f=bar", nil)

			// Handle model get and access request
			mreqs := s.GetParallelRequests(t, 2)
			mreqs.
				GetRequest(t, "access.test.model").
				AssertPathPayload(t, "token", nil).
				AssertPathPayload(t, "query", "q=foo&f=bar").
				RespondSuccess(json.RawMessage(`{"get":true}`))
			mreqs.
				GetRequest(t, "get.test.model").
				AssertPathPayload(t, "query", "q=foo&f=bar").
				RespondSuccess(json.RawMessage(`{"model":` + model + `,"query":"q=foo&f=bar"}`))

			// Validate http response
			hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(model))
		}, func(c *server.Config) {
			c.APIEncoding = enc
		})
	}
}

// Test getting a primitive query collection with a HTTP GET request
func TestHTTPGetOnPrimitiveQueryCollection(t *testing.T) {
	encodings := []string{"json", "jsonFlat"}

	for _, enc := range encodings {
		runNamedTest(t, enc, func(s *Session) {
			collection := resourceData("test.collection")

			hreq := s.HTTPRequest("GET", "/api/test/collection?q=foo&f=bar", nil)

			// Handle collection get and access request
			mreqs := s.GetParallelRequests(t, 2)
			mreqs.
				GetRequest(t, "access.test.collection").
				AssertPathPayload(t, "token", nil).
				AssertPathPayload(t, "query", "q=foo&f=bar").
				RespondSuccess(json.RawMessage(`{"get":true}`))
			mreqs.
				GetRequest(t, "get.test.collection").
				AssertPathPayload(t, "query", "q=foo&f=bar").
				RespondSuccess(json.RawMessage(`{"collection":` + collection + `,"query":"q=foo&f=bar"}`))

			// Validate http response
			hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(collection))
		}, func(c *server.Config) {
			c.APIEncoding = enc
		})
	}
}
