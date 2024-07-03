package server

import "time"

const (
	// Version is the current version for the server.
	Version = "1.8.0"

	// ProtocolVersion is the implemented RES protocol version.
	ProtocolVersion = "1.2.3"

	// DefaultAddr is the default host for client connections.
	DefaultAddr = "0.0.0.0"

	// DefaultPort is the default port for client connections.
	DefaultPort = 8080

	// DefaultWSPath is the default path for WebSocket connections.
	DefaultWSPath = "/"

	// DefaultAPIPath is the default path to web resource.
	DefaultAPIPath = "/api"

	// DefaultAPIEncoding is the default encoding for web resources.
	DefaultAPIEncoding = "json"

	// WSTimeout is the wait time for WebSocket connections to close on shutdown.
	WSTimeout = 3 * time.Second

	// MQTimeout is the wait time for the messaging client to close on shutdown.
	MQTimeout = 3 * time.Second

	// WSConnWorkerQueueSize is the size of the queue for each connection worker.
	WSConnWorkerQueueSize = 256

	// CIDPlaceholder is the placeholder tag for the connection ID.
	CIDPlaceholder = "{cid}"

	// SubscriptionCountLimit is the subscription limit of a single connection.
	SubscriptionCountLimit = 256

	// CacheWorkers is the number of goroutines handling cached resources.
	CacheWorkers = 10

	// UnsubscribeDelay is the delay for the cache to unsubscribe and evict resources no longer used.
	UnsubscribeDelay = 5 * time.Second
)
