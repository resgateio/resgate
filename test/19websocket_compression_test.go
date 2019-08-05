package test

import (
	"testing"

	"github.com/resgateio/resgate/server"
)

// Test subscribing to a resource with WebSocket compression enabled
func TestWebSocketCompressionEnabled(t *testing.T) {
	runTest(t, func(s *Session) {
		c := s.Connect()
		subscribeToTestModel(t, s, c)
	}, func(c *server.Config) {
		c.WSCompression = true
	})
}
