package server

import (
	"time"

	"github.com/resgateio/resgate/server/rescache"
)

func (s *Service) initMQClient() {
	unsubdelay := UnsubscribeDelay
	if s.cfg.NoUnsubscribeDelay {
		unsubdelay = 0
	}
	s.cache = rescache.NewCache(s.mq, CacheWorkers, s.cfg.ResetThrottle, unsubdelay, s.logger, s.metrics)
}

// startMQClients creates a connection to the messaging system.
// Service.mu is held when called
func (s *Service) startMQClient() error {
	if err := s.mq.Connect(); err != nil {
		return err
	}

	if err := s.cache.Start(); err != nil {
		return err
	}

	s.mq.SetClosedHandler(s.handleClosedMQ)
	return nil
}

// stopMQClient closes the connection to the nats server
func (s *Service) stopMQClient() {
	s.Debugf("Closing messaging client...")
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.mq.Close()
	}()

	select {
	case <-done:
		s.Debugf("Messaging client gracefully closed")
	case <-time.After(MQTimeout):
		s.Errorf("Closing messaging client timed out. Continuing shutdown.")
	}

	s.Debugf("Stopping cache workers...")
	s.cache.Stop()
	s.Debugf("Cache workers stopped")
}

func (s *Service) handleClosedMQ(err error) {
	s.Stop(err)
}
