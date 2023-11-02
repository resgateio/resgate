package metrics

import (
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	uUIDRegex = regexp.MustCompile("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
	iDRegex   = regexp.MustCompile("[0-9]+")
)

var (
	// SubcriptionsCount number of subscriptions per sanitized name
	SubcriptionsCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "resgate",
		Subsystem: "cache",
		Name:      "subscriptions",
		Help:      "Number of subscriptions per sanitized name",
	}, []string{"name"})
	// NATSConnected status of NATS connection
	NATSConnected = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "resgate",
		Subsystem: "nats",
		Name:      "connected",
		Help:      "Status of NATS connection",
	}, []string{"host"})
	// WSStablishedConnections number of stablished websocket connections
	WSStablishedConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "resgate",
		Subsystem: "ws",
		Name:      "stablished_connections",
		Help:      "Number of stablished websocket connections",
	})
)

// RegisterMetrics register all the defined metrics so they can be populated and consumed.
func RegisterMetrics() {
	prometheus.MustRegister(SubcriptionsCount)
	prometheus.MustRegister(NATSConnected)
	prometheus.MustRegister(WSStablishedConnections)
}

func SanitizedString(s string) string {
	s = uUIDRegex.ReplaceAllString(s, "{uuid}")
	s = iDRegex.ReplaceAllString(s, "{id}")
	return s
}
