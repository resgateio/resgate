package server

import (
	"errors"

	"github.com/jirenius/resgate/server/rescache"
)

const mqWorkers = 10

func (s *Service) initMQClient() {
	s.cache = rescache.NewCache(s.mq, mqWorkers, s.logger)
}

// startMQClients creates a connection to the message queue.
// Service.mu is held when called
func (s *Service) startMQClient() error {
	s.Logf("Connecting to message queue")
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
	if !s.mq.IsClosed() {
		s.Logf("Closing the message queue client connection...")
		s.mq.Close()
	}
	s.Logf("Stopping rescache workers...")
	s.cache.Stop()
	s.Logf("rescache stopped")
}

func (s *Service) handleClosedMQ(err error) {
	if err != nil {
		s.Logf("Message queue connection closed: ", err)
	} else {
		s.Logf("Message queue connection closed")
	}
	s.Stop(errors.New("lost connection to message queue"))
}
