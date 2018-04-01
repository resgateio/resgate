package service

import (
	"errors"
	"github.com/jirenius/resgate/resourceCache"
)

const mqWorkers = 10

func (s *Service) initMQClient() {}

// startMQClients creates a connection to the message queue.
// Service.mu is held when called
func (s *Service) startMQClient() error {
	s.Log("Connecting to message queue")
	s.cache = resourceCache.NewCache(s.mq, mqWorkers, s.logFlags)
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
		s.Log("Closing the message queue client connection...")
		s.mq.Close()
	}
	s.Log("Stopping resourceCache workers...")
	s.cache.Stop()
	s.Log("ResourceCache stopped")
}

func (s *Service) handleClosedMQ(err error) {
	if err != nil {
		s.Log("Message queue connection closed: ", err)
	} else {
		s.Log("Message queue connection closed")
	}
	s.Stop(errors.New("lost connection to message queue"))
}
