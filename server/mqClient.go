package server

import (
	"github.com/resgateio/resgate/server/rescache"
)

func (s *Service) initMQClient() {
	s.cache = rescache.NewCache(s.mq, CacheWorkers, UnsubscribeDelay, s.logger)
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
	s.mq.Close()
	s.Debugf("Stopping cache workers...")
	s.cache.Stop()
	s.Debugf("Cache workers stopped")
}

func (s *Service) handleClosedMQ(err error) {
	s.Stop(err)
}
