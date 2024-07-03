// Tests for metrics
package test

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/resgateio/resgate/server"
)

func TestMetrics_DefaultResponse_ContainsExpectedValues(t *testing.T) {
	runTest(t, func(s *Session) {
		r := s.MetricsHTTPRequest()
		AssertResponseHeaders(t, r, map[string]string{"Content-Type": "application/openmetrics-text; version=1.0.0; charset=utf-8"})
		AssertResponseContainsMetrics(t, r, []string{
			`# TYPE go info`,
			fmt.Sprintf(`go_info{version="%s"} 1`, runtime.Version()),
			`# TYPE resgate info`,
			fmt.Sprintf(`resgate_info{version="%s",protocol="%s"} 1`, server.Version, server.ProtocolVersion),
			`# TYPE go_memstats_sys_bytes gauge`,
			`# TYPE resgate_ws_current_connections gauge`,
			`resgate_ws_current_connections 0`,
			`# TYPE resgate_ws_connections counter`,
			`resgate_ws_connections_total 0`,
			`# TYPE resgate_ws_requests counter`,
			`resgate_ws_requests_total{method="get"} 0`,
			`resgate_ws_requests_total{method="unsubscribe"} 0`,
			`resgate_ws_requests_total{method="auth"} 0`,
			`resgate_ws_requests_total{method="subscribe"} 0`,
			`resgate_ws_requests_total{method="call"} 0`,
			`# TYPE resgate_http_requests counter`,
			`resgate_http_requests_total{method="POST"} 0`,
			`resgate_http_requests_total{method="GET"} 0`,
			`# TYPE resgate_cache_resources gauge`,
			`resgate_cache_resources 0`,
			`# TYPE resgate_cache_subscriptions gauge`,
			`resgate_cache_subscriptions 0`,
			`# EOF`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_WSCurrentConnections_IncreasesOnConnect(t *testing.T) {
	runTest(t, func(s *Session) {
		s.Connect()
		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_ws_current_connections 1`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_WSCurrentConnections_DecreasesOnDisconnect(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		c.Disconnect()
		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_ws_current_connections 0`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_WSCurrentConnectionsMultiple_IncreasesOnEachConnect(t *testing.T) {
	runTest(t, func(s *Session) {
		c1 := s.Connect()
		s.Connect()
		c1.Disconnect()
		s.Connect()
		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_ws_current_connections 2`,
			`resgate_ws_connections_total 3`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_WSConnections_IncreasesOnConnect(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		c.Disconnect()
		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_ws_connections_total 1`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_WSConnectionsMultiple_IncreasesOnEachConnect(t *testing.T) {
	runTest(t, func(s *Session) {
		c1 := s.Connect()
		s.Connect()
		c1.Disconnect()
		s.Connect()
		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_ws_connections_total 3`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_WSRequestsGet_IncreasesCounter(t *testing.T) {
	runTest(t, func(s *Session) {
		model := resourceData("test.model")

		c := s.Connect()
		creq := c.Request("get.test.model", nil)

		// Handle model get and access request
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.GetRequest(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.GetRequest(t, "get.test.model").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

		// Validate client response
		creq.GetResponse(t)

		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_ws_requests_total{method="get"} 1`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_WSRequestsUnsubscribe_IncreasesCounter(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Send subscribe request
		subscribeToTestModel(t, s, c)
		c.Request("unsubscribe.test.model", nil).GetResponse(t)

		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_ws_requests_total{method="unsubscribe"} 1`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_WSRequestsAuth_IncreasesCounter(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Send auth request
		creq := c.Request("auth.test.method", nil)
		s.GetRequest(t).RespondSuccess(nil)
		creq.GetResponse(t)

		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_ws_requests_total{method="auth"} 1`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_WSRequestsSubscribe_IncreasesCounter(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Send subscribe request
		subscribeToTestModel(t, s, c)

		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_ws_requests_total{method="subscribe"} 1`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_WSRequestsCall_IncreasesCounter(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Send call request
		creq := c.Request("call.test.model.method", nil)
		s.GetRequest(t).
			AssertSubject(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true,"call":"*"}`))
		s.GetRequest(t).
			AssertSubject(t, "call.test.model.method").
			RespondSuccess(nil)
		creq.GetResponse(t)

		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_ws_requests_total{method="call"} 1`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_WSRequestsNew_IncreasesCallCounter(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		// Send call request
		creq := c.Request("new.test.model", nil)
		s.GetRequest(t).
			AssertSubject(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true,"call":""}`))
		creq.GetResponse(t)

		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_ws_requests_total{method="call"} 1`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_HTTPRequestsPost_IncreasesCounter(t *testing.T) {
	runTest(t, func(s *Session) {
		hreq := s.HTTPRequest("POST", "/api/test/model/method", nil)
		// Handle query model access request
		s.GetRequest(t).
			AssertSubject(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"call":"method"}`))
		// Handle query model call request
		s.GetRequest(t).
			AssertSubject(t, "call.test.model.method").
			RespondSuccess(nil)
		// Validate http response
		hreq.GetResponse(t)

		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_http_requests_total{method="POST"} 1`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_HTTPRequestsGet_IncreasesCounter(t *testing.T) {
	model := resourceData("test.model")

	runTest(t, func(s *Session) {
		// Get model
		hreq := s.HTTPRequest("GET", "/api/test/model", nil)
		mreqs := s.GetParallelRequests(t, 2)
		mreqs.
			GetRequest(t, "access.test.model").
			RespondSuccess(json.RawMessage(`{"get":true}`))
		mreqs.
			GetRequest(t, "get.test.model").
			RespondSuccess(json.RawMessage(`{"model":` + model + `}`))
		hreq.GetResponse(t)

		AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
			`resgate_http_requests_total{method="GET"} 1`,
		})
	}, func(cfg *server.Config) {
		cfg.MetricsPort = 8090
	})
}

func TestMetrics_HTTPRequestsOther_IncreasesCounter(t *testing.T) {
	methods := []string{"PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		methodLower := strings.ToLower(method)
		runNamedTest(t, method, func(s *Session) {
			hreq := s.HTTPRequest(method, "/api/test/model", nil)
			// Handle query model access request
			s.GetRequest(t).
				AssertSubject(t, "access.test.model").
				RespondSuccess(json.RawMessage(`{"call":"` + methodLower + `"}`))
			// Handle query model call request
			s.GetRequest(t).
				AssertSubject(t, "call.test.model."+methodLower).
				RespondSuccess(nil)
			// Validate http response
			hreq.GetResponse(t)

			AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
				fmt.Sprintf(`resgate_http_requests_total{method="%s"} 1`, method),
			})
		}, func(cfg *server.Config) {
			cfg.MetricsPort = 8090
			switch method {
			case "PUT":
				cfg.PUTMethod = &methodLower
			case "PATCH":
				cfg.PATCHMethod = &methodLower
			case "DELETE":
				cfg.DELETEMethod = &methodLower
			}
		})
	}
}

func TestMetrics_CacheResources_ExpectedGaugeValues(t *testing.T) {
	table := []struct {
		Name                  string
		Actions               func(t *testing.T, s *Session, c *Conn)
		Config                func(cfg *server.Config)
		ExpectedResources     int
		ExpectedSubscriptions int
	}{
		{
			Name: "Simple model",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModel(t, s, c)
			},
			ExpectedResources:     1,
			ExpectedSubscriptions: 1,
		},
		{
			Name: "Simple collection",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestCollection(t, s, c)
			},
			ExpectedResources:     1,
			ExpectedSubscriptions: 1,
		},
		{
			Name: "Parent model",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModelParent(t, s, c, false)
			},
			ExpectedResources:     2,
			ExpectedSubscriptions: 2,
		},
		{
			Name: "Parent collection",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestCollectionParent(t, s, c, false)
			},
			ExpectedResources:     2,
			ExpectedSubscriptions: 2,
		},
		{
			Name: "Overlapping model subscription",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModel(t, s, c)
				subscribeToTestModelParent(t, s, c, true)
			},
			ExpectedResources:     2,
			ExpectedSubscriptions: 2,
		},
		{
			Name: "simple model from two connections",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModel(t, s, c)
				c2 := s.Connect()
				subscribeToCachedResource(t, s, c2, "test.model")
			},
			ExpectedResources:     1,
			ExpectedSubscriptions: 2,
		},
		{
			Name: "overlapping model subscription from two connections",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModelParent(t, s, c, false)
				c2 := s.Connect()
				subscribeToCachedResource(t, s, c2, "test.model")
			},
			ExpectedResources:     2,
			ExpectedSubscriptions: 3,
		},
		{
			Name: "unsubscribe simple model",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModel(t, s, c)
				c.Request("unsubscribe.test.model", nil).GetResponse(t)
				c.AssertNoEvent(t, "test.model")
			},
			ExpectedResources:     1,
			ExpectedSubscriptions: 0,
		},
		{
			Name: "unsubscribe parent model",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModelParent(t, s, c, false)
				c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)
				c.AssertNoEvent(t, "test.model.parent")
			},
			ExpectedResources:     2,
			ExpectedSubscriptions: 0,
		},
		{
			Name: "unsubscribe overlapping child model",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModel(t, s, c)
				subscribeToTestModelParent(t, s, c, true)
				c.Request("unsubscribe.test.model", nil).GetResponse(t)
				c.AssertNoEvent(t, "test.model")
			},
			ExpectedResources:     2,
			ExpectedSubscriptions: 2,
		},
		{
			Name: "unsubscribe overlapping parent model",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModel(t, s, c)
				subscribeToTestModelParent(t, s, c, true)
				c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)
				c.AssertNoEvent(t, "test.model.parent")
			},
			ExpectedResources:     2,
			ExpectedSubscriptions: 1,
		},

		{
			Name: "unsubscribe simple model and wait for cache unsubscribe",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModel(t, s, c)
				c.Request("unsubscribe.test.model", nil).GetResponse(t)
				s.AssertUnsubscribe("test.model")
				c.AssertNoEvent(t, "test.model")
			},
			Config: func(cfg *server.Config) {
				cfg.NoUnsubscribeDelay = true
			},
			ExpectedResources:     0,
			ExpectedSubscriptions: 0,
		},
		{
			Name: "unsubscribe parent model and wait for cache unsubscribe",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModelParent(t, s, c, false)
				c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)
				s.AwaitUnsubscribe()
				s.AwaitUnsubscribe()
				c.AssertNoEvent(t, "test.model.parent")
			},
			Config: func(cfg *server.Config) {
				cfg.NoUnsubscribeDelay = true
			},
			ExpectedResources:     0,
			ExpectedSubscriptions: 0,
		},
		{
			Name: "unsubscribe overlapping parent model and wait for cache unsubscribe",
			Actions: func(t *testing.T, s *Session, c *Conn) {
				subscribeToTestModel(t, s, c)
				subscribeToTestModelParent(t, s, c, true)
				c.Request("unsubscribe.test.model.parent", nil).GetResponse(t)
				c.AssertNoEvent(t, "test.model.parent")
				s.AssertUnsubscribe("test.model.parent")
			},
			Config: func(cfg *server.Config) {
				cfg.NoUnsubscribeDelay = true
			},
			ExpectedResources:     1,
			ExpectedSubscriptions: 1,
		},
	}
	for _, l := range table {
		runNamedTest(t, l.Name, func(s *Session) {
			c := s.Connect()
			l.Actions(t, s, c)

			AssertResponseContainsMetrics(t, s.MetricsHTTPRequest(), []string{
				fmt.Sprintf(`resgate_cache_resources %d`, l.ExpectedResources),
				fmt.Sprintf(`resgate_cache_subscriptions %d`, l.ExpectedSubscriptions),
			})
		}, func(cfg *server.Config) {
			cfg.MetricsPort = 8090
			if l.Config != nil {
				l.Config(cfg)
			}
		})
	}
}
