package service

import (
	"errors"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/jirenius/resgate/mq"
	"github.com/jirenius/resgate/resourceCache"
)

// Service is a RES gateway implementation
type Service struct {
	cfg      Config
	logger   *log.Logger
	mu       sync.Mutex
	stopping bool
	stop     chan error
	logFlags int

	mq    mq.Client
	cache *resourceCache.Cache

	// httpServer
	mux *http.ServeMux
	h   *http.Server

	// wsListener/wsConn
	seq   uint64             // Sequential counter for wsConn Ids
	conns map[string]*wsConn // Connections by wsConn Id's
	wg    sync.WaitGroup     // Wait for all connections to be disconnected
}

// NewService creates a new Service
func NewService(mq mq.Client, cfg Config) *Service {
	logFlags := log.LstdFlags
	if debug {
		logFlags = log.Ltime
	}
	s := &Service{
		cfg:      cfg,
		mq:       mq,
		logFlags: logFlags,
		logger:   log.New(os.Stdout, "[Main] ", logFlags),
	}

	s.cfg.prepare()
	s.initHTTPServer()
	s.initWsListener()
	s.initHTTPListener()
	s.initMQClient()
	return s
}

var debug = false

// SetDebug enables debug logging
func SetDebug(enabled bool) {
	debug = enabled
}

// Log writes a log message
func (s *Service) Log(v ...interface{}) {
	s.logger.Print(v...)
}

// Logf writes a formatted log message
func (s *Service) Logf(format string, v ...interface{}) {
	s.logger.Printf(format, v...)
}

// Start connects the Service to the nats server
func (s *Service) Start() (err error) {
	err = s.start()
	if err != nil {
		s.Stop(err)
	}
	return
}

// Start connects the Service to the nats server
func (s *Service) start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stop != nil {
		return nil
	}
	if s.stopping {
		return errors.New("Service is stopping")
	}

	s.Log("Starting service")
	s.stop = make(chan error, 1)

	if err := s.startMQClient(); err != nil {
		s.Logf("Failed to connect to message queue: %s", err)
		return err
	}

	s.startHTTPServer()

	return nil
}

// Stop closes the connection to the nats server
func (s *Service) Stop(err error) {
	s.mu.Lock()
	if s.stop == nil || s.stopping {
		s.mu.Unlock()
		return
	}
	s.stopping = true
	s.mu.Unlock()

	s.Log("Stopping service...")

	s.stopWsListener()
	s.stopHTTPServer()
	s.stopMQClient()

	s.mu.Lock()
	s.stop <- err
	close(s.stop)
	s.stop = nil
	s.stopping = false
	s.Log("Service stopped")
	s.mu.Unlock()
}

// StopChannel returns a channel that will pass a value
// when the service has stopped.
func (s *Service) StopChannel() <-chan error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stop
}
