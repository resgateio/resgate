package server

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gorilla/websocket"
	"github.com/resgateio/resgate/server/codec"
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

	var h http.Header
	if s.cfg.WSHeaderAuth != nil {
		// Prevent calling wsHeaderAuth if origin doesn't match. This will cause
		// CheckOrigin to be called twice, both here and during Upgrade. But it
		// will prevent unnecessary auth requests.
		if s.upgrader.CheckOrigin(r) {
			// Make auth call (if wsHeaderAuth is configured) and if we have a
			// meta in the response, handle it.
			refRID, meta, err := s.wsHeaderAuth(conn)
			if meta != nil {
				if meta.IsDirectResponseStatus() {
					conn.Dispose()
					httpStatusResponse(w, s.enc, *meta.Status, meta.Header, RIDToPath(refRID, s.cfg.APIPath), err)
					return
				}
				if meta.Header != nil {
					h = make(http.Header, len(meta.Header))
					codec.MergeHeader(h, meta.Header)
				}
			}
		}
	}

	// Upgrade to gorilla websocket
	ws, err := s.upgrader.Upgrade(w, r, h)
	if err != nil {
		conn.Dispose()
		s.Debugf("Failed to upgrade connection from %s: %s", r.RemoteAddr, err.Error())
		return
	}

	conn.Tracef("Connected: %s", ws.RemoteAddr())

	// Metrics
	if s.metrics != nil {
		s.metrics.WSConnectionCount.Add(1)
		s.metrics.WSConnections.Add(1)
	}

	// Set websocket and start listening
	conn.listen(ws)

	// Metrics
	if s.metrics != nil {
		s.metrics.WSConnections.Add(-1)
	}

	if s.onWSClose != nil {
		s.onWSClose(ws)
	}
}

// wsHeaderAuth sends an auth resource request if WSHeaderAuth is set, and
// awaits the answer, returning any error. If no WSHeaderAuth is set, this is a
// no-op.
func (s *Service) wsHeaderAuth(c *wsConn) (refRID string, meta *codec.Meta, err error) {
	done := make(chan struct{})
	c.Enqueue(func() {
		// Temporarily set as latest protocol version during the auth call.
		storedVer := c.protocolVer
		c.protocolVer = versionLatest
		c.AuthResourceNoResult(s.cfg.wsHeaderAuthRID, s.cfg.wsHeaderAuthAction, nil, func(ref string, e error, m *codec.Meta) {
			c.protocolVer = storedVer
			// Validate the status of the meta object.
			if !m.IsValidStatus() {
				s.Errorf("Invalid WebSocket meta status: %d", *m.Status)
				m.Status = nil
			}
			refRID = ref
			meta = m
			err = e
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
