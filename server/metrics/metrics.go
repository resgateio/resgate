package metrics

import (
	"runtime"

	"github.com/bsm/openmetrics"
)

type MetricSet struct {
	m runtime.MemStats
	// Memstats
	MemSysBytes openmetrics.Gauge
	// WebSocket connectionws
	WSConnections     openmetrics.Gauge
	WSConnectionCount openmetrics.Counter
	// WebSocket requests
	WSRequestsGet         openmetrics.Counter
	WSRequestsSubscribe   openmetrics.Counter
	WSRequestsUnsubscribe openmetrics.Counter
	WSRequestsCall        openmetrics.Counter
	WSRequestsAuth        openmetrics.Counter
	// Cache
	CacheResources     openmetrics.Gauge
	CacheSubscriptions openmetrics.Gauge
	// HTTP requests
	HTTPRequests     openmetrics.CounterFamily
	HTTPRequestsGet  openmetrics.Counter
	HTTPRequestsPost openmetrics.Counter
}

// Scrape updates the metric set with info on current mem usage.
func (m *MetricSet) Scrape() {
	runtime.ReadMemStats(&m.m)
	m.MemSysBytes.Set(float64(m.m.Sys))
}

func (m *MetricSet) Register(reg *openmetrics.Registry, version string, protocolVersion string) {
	// Go info
	reg.Info(openmetrics.Desc{
		Name:   "go",
		Help:   "Information about the Go environment.",
		Labels: []string{"version"},
	}).With(runtime.Version())

	// Resgate info
	reg.Info(openmetrics.Desc{
		Name:   "resgate",
		Help:   "Information about resgate.",
		Labels: []string{"version", "protocol"},
	}).With(version, protocolVersion)

	// Memory stats
	m.MemSysBytes = reg.Gauge(openmetrics.Desc{
		Name: "go_memstats_sys_bytes",
		Help: "Number of bytes obtained from system.",
	}).With()
	m.MemSysBytes.Set(0)

	// WebSocket connections
	m.WSConnections = reg.Gauge(openmetrics.Desc{
		Name: "resgate_ws_current_connections",
		Help: "Current established WebSocket connections.",
	}).With()
	m.WSConnections.Set(0)
	m.WSConnectionCount = reg.Counter(openmetrics.Desc{
		Name: "resgate_ws_connections",
		Help: "Total established WebSocket connections.",
	}).With()

	// WebSocket requests
	wsRequests := reg.Counter(openmetrics.Desc{
		Name:   "resgate_ws_requests",
		Help:   "Total WebSocket client requests.",
		Labels: []string{"method"},
	})
	m.WSRequestsGet = wsRequests.With("get")
	m.WSRequestsSubscribe = wsRequests.With("subscribe")
	m.WSRequestsUnsubscribe = wsRequests.With("unsubscribe")
	m.WSRequestsCall = wsRequests.With("call")
	m.WSRequestsAuth = wsRequests.With("auth")

	// HTTP requests
	m.HTTPRequests = reg.Counter(openmetrics.Desc{
		Name:   "resgate_http_requests",
		Help:   "Total HTTP client requests.",
		Labels: []string{"method"},
	})
	m.HTTPRequestsGet = m.HTTPRequests.With("GET")
	m.HTTPRequestsPost = m.HTTPRequests.With("POST")

	// Cache
	m.CacheResources = reg.Gauge(openmetrics.Desc{
		Name: "resgate_cache_resources",
		Help: "Current number of resources stored in the cache.",
	}).With()
	m.CacheResources.Set(0)
	m.CacheSubscriptions = reg.Gauge(openmetrics.Desc{
		Name: "resgate_cache_subscriptions",
		Help: "Current number of subscriptions on cached resources.",
	}).With()
	m.CacheSubscriptions.Set(0)
}
