package service

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var wsDisconnectTimeout = 3 * time.Second

func (s *Service) initWsListener() {
	s.conns = make(map[string]*wsConn)
	s.mux.HandleFunc(s.cfg.WSPath, s.wsHandler)
}

// GetWSHandlerFunc returns the websocket http.Handler
// Used for testing purposes
func (s *Service) GetWSHandlerFunc() http.Handler {
	return http.HandlerFunc(s.wsHandler)
}

func (s *Service) wsHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow exact matching path
	if r.URL.Path != s.cfg.WSPath {
		s.httpHandler(w, r)
		return
	}
	// Upgrade to gorilla websocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Debugf("Failed to upgrade connection from %s: %s", r.RemoteAddr, err.Error())
		return
	}

	conn := s.newWSConn(ws, r)
	if conn == nil {
		return
	}
	conn.listen()
}

// stopWsListener disconnects all ws connections.
func (s *Service) stopWsListener() {
	s.mu.Lock()
	// Quick exit if we have no connections
	if len(s.conns) == 0 {
		s.mu.Unlock()
		return
	}
	s.Logf("Disconnecting all ws connections...")
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
		s.Logf("All ws connections gracefully closed")
	case <-time.After(wsDisconnectTimeout):
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

		s.Logf("Disconnecting %s timed out:\n%s", idStr, string(buf))
	}
}
