package test

import (
	"testing"
)

// Test that the server starts and stops the server without error
func TestStart(t *testing.T) {
	runTest(t, func(s *Session) {})
}

// Test that a client can connect to the server without error
func TestConnectClient(t *testing.T) {
	runTest(t, func(s *Session) {
		s.Connect()
	})
}

// // Test that a client gets error connecting to a server that is stopped
// func TestNotConnectedClientWhenStopped(t *testing.T) {
// 	var sess *Session
// 	runTest(t, func(s *Session) {
// 		sess = s
// 	})
// 	sess.Connect()
// 	// c.AssertClosed(t) fails as the read from the websocket hijacked by wstest never returns
// }
