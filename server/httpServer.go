package server

import (
	"context"
	"net/http"
	"strings"
	"time"
)

func (s *Service) initHTTPServer() {
}

// startHTTPServer initializes the server and starts a goroutine with a http
// server Service.mu is held when called.
func (s *Service) startHTTPServer() {
	if s.cfg.NoHTTP {
		return
	}

	s.Logf("Listening on %s://%s", s.cfg.scheme, s.cfg.netAddr)
	h := &http.Server{Addr: s.cfg.netAddr, Handler: s}
	s.h = h

	go func() {
		var err error
		if s.cfg.TLS {
			err = h.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
		} else {
			err = h.ListenAndServe()
		}

		if err != nil {
			s.Stop(err)
		}
	}()
}

// stopHTTPServer stops the http server
func (s *Service) stopHTTPServer() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.h == nil {
		return
	}

	s.Debugf("Stopping HTTP server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.h.Shutdown(ctx)
	s.h = nil

	if ctx.Err() == context.DeadlineExceeded {
		s.Errorf("HTTP server forcefully stopped after timeout")
	} else {
		s.Debugf("HTTP server gracefully stopped")
	}
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Global OPTIONS handling taken from http.ServeMux
	if r.RequestURI == "*" {
		if r.ProtoAtLeast(1, 1) {
			w.Header().Set("Connection", "close")
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch {
	case r.URL.Path == s.cfg.WSPath:
		s.wsHandler(w, r)
	case strings.HasPrefix(r.URL.Path, s.cfg.APIPath):
		s.apiHandler(w, r)
	default:
		notFoundHandler(w, s.enc)
	}
}
