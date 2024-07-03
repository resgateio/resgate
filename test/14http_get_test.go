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

	encodings := []string{"json", "jsonFlat"}

	for _, enc := range encodings {
		for i, l := range tbl {
			runNamedTest(t, fmt.Sprintf("#%d (%s)", i+1, enc), func(s *Session) {
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
			}, func(c *server.Config) {
				c.APIEncoding = enc
			})
		}
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
				"test.model.soft":         `{"name":"soft","child":{"href":"/api/test/model"}}`,
				"test.model.soft.parent":  `{"name":"softparent","child":{"href":"/api/test/model/soft","model":{"name":"soft","child":{"href":"/api/test/model"}}}}`,
				"test.model.data":         `{"name":"data","primitive":12,"object":{"foo":["bar"]},"array":[{"foo":"bar"}]}`,
				"test.model.data.parent":  `{"name":"dataparent","child":{"href":"/api/test/model/data","model":{"name":"data","primitive":12,"object":{"foo":["bar"]},"array":[{"foo":"bar"}]}}}`,
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
				"test.collection.soft":         `["soft",{"href":"/api/test/collection"}]`,
				"test.collection.soft.parent":  `["softparent",{"href":"/api/test/collection/soft","collection":["soft",{"href":"/api/test/collection"}]}]`,
				"test.collection.data":         `["data",12,{"foo":["bar"]},[{"foo":"bar"}]]`,
				"test.collection.data.parent":  `["dataparent",{"href":"/api/test/collection/data","collection":["data",12,{"foo":["bar"]},[{"foo":"bar"}]]}]`,
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
				"test.model.soft":         `{"name":"soft","child":{"href":"/api/test/model"}}`,
				"test.model.soft.parent":  `{"name":"softparent","child":{"name":"soft","child":{"href":"/api/test/model"}}}`,
				"test.model.data":         `{"name":"data","primitive":12,"object":{"foo":["bar"]},"array":[{"foo":"bar"}]}`,
				"test.model.data.parent":  `{"name":"dataparent","child":{"name":"data","primitive":12,"object":{"foo":["bar"]},"array":[{"foo":"bar"}]}}`,
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
				"test.collection.soft":         `["soft",{"href":"/api/test/collection"}]`,
				"test.collection.soft.parent":  `["softparent",["soft",{"href":"/api/test/collection"}]]`,
				"test.collection.data":         `["data",12,{"foo":["bar"]},[{"foo":"bar"}]]`,
				"test.collection.data.parent":  `["dataparent",["data",12,{"foo":["bar"]},[{"foo":"bar"}]]]`,
				"test.c.a":                     `[{"href":"/api/test/c/a"}]`,
				"test.c.b":                     `[[{"href":"/api/test/c/b"}]]`,
				"test.c.d":                     `[[{"href":"/api/test/c/d"}],[{"href":"/api/test/c/d"}]]`,
				"test.c.g":                     `[[[{"href":"/api/test/c/e"},[{"href":"/api/test/c/d"}]]],[[[{"href":"/api/test/c/d"}],{"href":"/api/test/c/f"}]]]`,
				"test.c.h":                     `[[[{"href":"/api/test/c/e"},[{"href":"/api/test/c/d"}]]]]`,
			},
		},
	}

	for _, enc := range encodings {
		for _, set := range sequenceSets {
			if set.Version != versionLatest {
				continue
			}
			for i, l := range set.Table {
				runNamedTest(t, fmt.Sprintf("#%d with APIEncoding %#v", i+1, enc.APIEncoding), func(s *Session) {
					var hreq *HTTPRequest
					var req *Request

					hreqs := make(map[string]*HTTPRequest)
					reqs := make(map[string]*Request)

				TestTable:
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
							req.AssertPathPayload(t, "isHttp", true)
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
							break TestTable
						case "errorResponse":
							hreq = hreqs[ev.RID]
							hreq.GetResponse(t).AssertIsError(t)
							break TestTable
						}
					}
				}, func(c *server.Config) {
					c.APIEncoding = enc.APIEncoding
				})
			}
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
				AssertPathPayload(t, "isHttp", true).
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
				AssertPathPayload(t, "isHttp", true).
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

func TestHTTPGet_AllowOrigin_ExpectedResponse(t *testing.T) {
	model := resourceData("test.model")
	successResponse := json.RawMessage(model)

	tbl := []struct {
		Origin                 string            // Request's Origin header. Empty means no Origin header.
		ContentType            string            // Request's Content-Type header. Empty means no Content-Type header.
		AllowOrigin            string            // AllowOrigin config
		ExpectedCode           int               // Expected response status code
		ExpectedHeaders        map[string]string // Expected response Headers
		ExpectedMissingHeaders []string          // Expected response headers not to be included
		ExpectedBody           interface{}       // Expected response body
	}{
		{"http://localhost", "", "*", http.StatusOK, map[string]string{"Access-Control-Allow-Origin": "*"}, []string{"Vary", "Access-Control-Allow-Credentials"}, successResponse},
		{"http://localhost", "", "http://localhost", http.StatusOK, map[string]string{"Access-Control-Allow-Origin": "http://localhost", "Vary": "Origin"}, []string{"Access-Control-Allow-Credentials"}, successResponse},
		{"https://resgate.io", "", "http://localhost;https://resgate.io", http.StatusOK, map[string]string{"Access-Control-Allow-Origin": "https://resgate.io", "Vary": "Origin"}, []string{"Access-Control-Allow-Credentials"}, successResponse},
		// Invalid requests
		{"http://example.com", "", "http://localhost;https://resgate.io", http.StatusForbidden, map[string]string{"Access-Control-Allow-Origin": "http://localhost", "Vary": "Origin"}, []string{"Access-Control-Allow-Credentials"}, reserr.ErrForbiddenOrigin},
		// No Origin header in request
		{"", "", "*", http.StatusOK, map[string]string{"Access-Control-Allow-Origin": "*"}, []string{"Vary"}, successResponse},
		{"", "", "http://localhost", http.StatusOK, nil, []string{"Access-Control-Allow-Origin", "Vary"}, successResponse},
	}

	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			hreq := s.HTTPRequest("GET", "/api/test/model", nil, func(req *http.Request) {
				if l.Origin != "" {
					req.Header.Set("Origin", l.Origin)
				}
				if l.ContentType != "" {
					req.Header.Set("Content-Type", l.ContentType)
				}
			})

			if l.ExpectedCode == http.StatusOK {
				// Handle model get and access request
				mreqs := s.GetParallelRequests(t, 2)
				mreqs.
					GetRequest(t, "access.test.model").
					RespondSuccess(json.RawMessage(`{"get":true}`))
				mreqs.
					GetRequest(t, "get.test.model").
					RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
			}

			// Validate http response
			hreq.GetResponse(t).
				Equals(t, l.ExpectedCode, l.ExpectedBody).
				AssertHeaders(t, l.ExpectedHeaders).
				AssertMissingHeaders(t, l.ExpectedMissingHeaders)
		}, func(cfg *server.Config) {
			cfg.AllowOrigin = &l.AllowOrigin
		})
	}
}

func TestHTTPGet_HeaderAuth_ExpectedResponse(t *testing.T) {
	model := resourceData("test.model")
	token := json.RawMessage(`{"user":"foo"}`)
	successResponse := json.RawMessage(model)

	tbl := []struct {
		AuthResponse    interface{}       // Response on auth request. requestTimeout means timeout.
		Token           interface{}       // Token to send. noToken means no token events should be sent.
		ExpectedHeaders map[string]string // Expected response Headers
	}{
		// Without token
		{requestTimeout, noToken, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{reserr.ErrNotFound, noToken, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{[]byte(`{]`), noToken, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{nil, noToken, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		// With token
		{requestTimeout, token, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{reserr.ErrNotFound, token, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{[]byte(`{]`), token, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{nil, token, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		// With nil token
		{requestTimeout, nil, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{reserr.ErrNotFound, nil, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{[]byte(`{]`), nil, map[string]string{"Access-Control-Allow-Credentials": "true"}},
		{nil, nil, map[string]string{"Access-Control-Allow-Credentials": "true"}},
	}

	for i, l := range tbl {
		l := l
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			hreq := s.HTTPRequest("GET", "/api/test/model", nil, func(req *http.Request) {
				req.Header.Set("Origin", "example.com")
			})

			req := s.GetRequest(t)
			req.AssertSubject(t, "auth.vault.method")
			req.AssertPathPayload(t, "header.Origin", []string{"example.com"})
			req.AssertPathPayload(t, "isHttp", true)
			// Send token
			expectedToken := l.Token
			if l.Token != noToken {
				cid := req.PathPayload(t, "cid").(string)
				s.ConnEvent(cid, "token", struct {
					Token interface{} `json:"token"`
				}{l.Token})
			} else {
				expectedToken = nil
			}
			// Respond to auth request
			if l.AuthResponse == requestTimeout {
				req.Timeout()
			} else if err, ok := l.AuthResponse.(*reserr.Error); ok {
				req.RespondError(err)
			} else if raw, ok := l.AuthResponse.([]byte); ok {
				req.RespondRaw(raw)
			} else {
				req.RespondSuccess(l.AuthResponse)
			}

			// Handle model get and access request
			mreqs := s.GetParallelRequests(t, 2)
			mreqs.
				GetRequest(t, "access.test.model").
				AssertPathPayload(t, "token", expectedToken).
				AssertPathPayload(t, "isHttp", true).
				RespondSuccess(json.RawMessage(`{"get":true}`))
			mreqs.
				GetRequest(t, "get.test.model").
				RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

			// Validate http response
			hreq.GetResponse(t).
				Equals(t, http.StatusOK, successResponse).
				AssertHeaders(t, l.ExpectedHeaders)
		}, func(cfg *server.Config) {
			headerAuth := "vault.method"
			cfg.HeaderAuth = &headerAuth
		})
	}
}

func TestHTTPGet_LongResourceID_ReturnsStatus414(t *testing.T) {
	longStr := generateString(10000)
	runTest(t, func(s *Session) {
		hreq := s.HTTPRequest("GET", "/api/test/"+longStr, nil)

		// Validate http response
		hreq.GetResponse(t).
			AssertError(t, reserr.ErrSubjectTooLong).
			AssertStatusCode(t, http.StatusRequestURITooLong)
	})
}

func TestHTTPGet_LongModelQuery_ReturnsModel(t *testing.T) {
	query := "q=" + generateString(10000)
	model := resourceData("test.model")

	runTest(t, func(s *Session) {
		hreq := s.HTTPRequest("GET", "/api/test/model?"+query, nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.
			GetRequest(t, "access.test.model").
			AssertPathPayload(t, "token", nil).
			AssertPathPayload(t, "query", query).
			RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.
			GetRequest(t, "get.test.model").
			AssertPathPayload(t, "query", query).
			RespondSuccess(json.RawMessage(`{"model":` + model + `,"query":"` + query + `"}`))

		// Validate http response
		hreq.GetResponse(t).Equals(t, http.StatusOK, json.RawMessage(model))
	})
}
