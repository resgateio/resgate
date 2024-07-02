package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/bsm/openmetrics"
	"github.com/bsm/openmetrics/omhttp"
	"github.com/resgateio/resgate/server/metrics"
)

const MetricsPattern = "/metrics"

func (s *Service) initMetricsServer() {
	if s.cfg.MetricsPort == 0 {
		return
	}
	s.metrics = &metrics.MetricSet{}
}

// MetricsHandler returns any metrics HTTP handler for testing purposes.
func (s *Service) MetricsHandler() http.Handler {
	return s.metricsh
}

// startMetricsServer initializes the server and starts a goroutine with a prometheus metrics server
func (s *Service) startMetricsServer() {
	if s.cfg.MetricsPort == 0 {
		return
	}

	reg := openmetrics.NewConsistentRegistry(func() time.Time { return time.Now() })
	ms := s.metrics
	ms.Register(reg, Version, ProtocolVersion)

	h := omhttp.NewHandler(reg)
	s.metricsh = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms.Scrape()
		h.ServeHTTP(w, r)
	})

	// For testing
	if s.cfg.NoHTTP {
		return
	}

	mux := http.NewServeMux()
	mux.Handle(MetricsPattern, s.metricsh)

	hln, err := net.Listen("tcp", s.cfg.metricsNetAddr)
	if err != nil {
		s.Logf("Metrics server can't listen on %s: %s", s.cfg.metricsNetAddr, err)
		return
	}

	metricsServer := &http.Server{
		Handler: mux,
	}
	s.m = metricsServer

	s.Logf("Metrics endpoint listening on %s://%s%s", s.cfg.scheme, s.cfg.metricsNetAddr, MetricsPattern)

	go func() {
		var err error
		if s.cfg.TLS {
			err = s.m.ServeTLS(hln, s.cfg.TLSCert, s.cfg.TLSKey)
		} else {
			err = s.m.Serve(hln)
		}

		if err != nil {
			s.Stop(err)
		}
	}()

}

// stopMetricsServer stops the Metrics server
func (s *Service) stopMetricsServer() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.m == nil {
		return
	}

	s.Debugf("Stopping Metrics server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.m.Shutdown(ctx)
	s.m = nil

	if ctx.Err() == context.DeadlineExceeded {
		s.Errorf("Metrics server forcefully stopped after timeout")
	} else {
		s.Debugf("Metrics server gracefully stopped")
	}
}
