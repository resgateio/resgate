package server

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gorilla/websocket"
)

func (s *Service) initWSHandler() {
	var co func(r *http.Request) bool
	switch s.cfg.allowOrigin[0] {
	case "*":
		co = func(r *http.Request) bool {
			return true
		}
	default:
		origins := s.cfg.allowOrigin
		co = func(r *http.Request) bool {
			origin := r.Header["Origin"]
			if len(origin) == 0 || origin[0] == "null" {
				return true
			}
			return matchesOrigins(origins, origin[0])
		}
	}
	s.upgrader = websocket.Upgrader{
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		CheckOrigin:       co,
		EnableCompression: s.cfg.WSCompression,
	}
	s.conns = make(map[string]*wsConn)
}

// GetWSHandlerFunc returns the websocket http.Handler
// Used for testing purposes
func (s *Service) GetWSHandlerFunc() http.Handler {
	return http.HandlerFunc(s.wsHandler)
}

func (s *Service) wsHandler(w http.ResponseWriter, r *http.Request) {
	conn := s.newWSConn(r, versionLegacy)
	if conn == nil {
		return
	}

	if s.cfg.WSHeaderAuth != nil {
		// Prevent calling wsHeaderAuth if origin doesn't match. This will cause
		// CheckOrigin to be called twice, both here and during Upgrade. But it
		// will prevent unnecessary auth requests.
		if s.upgrader.CheckOrigin(r) {
			// Make auth call (if wsHeaderAuth is configured) and ignore any error.
			_ = s.wsHeaderAuth(conn)
		}
	}

	// Upgrade to gorilla websocket
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		conn.Dispose()
		s.Debugf("Failed to upgrade connection from %s: %s", r.RemoteAddr, err.Error())
		return
	}

	conn.Tracef("Connected: %s", ws.RemoteAddr())

	// Set websocket and start listening
	conn.listen(ws)
}

// wsHeaderAuth sends an auth resource request if WSHeaderAuth is set, and
// awaits the answer, returning any error. If no WSHeaderAuth is set, this is a
// no-op.
func (s *Service) wsHeaderAuth(c *wsConn) (err error) {
	if s.cfg.WSHeaderAuth == nil {
		return
	}

	done := make(chan struct{})
	c.Enqueue(func() {
		// Temporarily set as latest protocol version during the auth call.
		storedVer := c.protocolVer
		c.protocolVer = versionLatest
		c.AuthResourceNoResult(s.cfg.wsHeaderAuthRID, s.cfg.wsHeaderAuthAction, nil, func(e error) {
			c.protocolVer = storedVer
			if e != nil {
				err = e
			}
			close(done)
		})
	})
	<-done
	return
}

// stopWSHandler disconnects all ws connections.
func (s *Service) stopWSHandler() {
	s.mu.Lock()
	// Quick exit if we have no connections
	if len(s.conns) == 0 {
		s.mu.Unlock()
		return
	}
	s.Debugf("Closing %d WebSocket connection(s)...", len(s.conns))
	// Disconnecting all ws connections
	for _, conn := range s.conns {
		conn.Disconnect("Server is shutting down")
	}
	s.mu.Unlock()

	// Await for waitGroup to be done
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.wg.Wait()
	}()

	select {
	case <-done:
		s.Debugf("All connections gracefully closed")
	case <-time.After(WSTimeout):
		// Time out

		// Create string of deadlocked connections
		idStr := ""
		s.mu.Lock()
		for _, conn := range s.conns {
			if idStr != "" {
				idStr += ", "
			}
			idStr += conn.String()
		}
		s.mu.Unlock()

		// Get the stack trace
		const size = 1 << 16
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, true)]

		s.Errorf("Closing connection %s timed out:\n%s", idStr, string(buf))
	}
}
