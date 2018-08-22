package test

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/jirenius/resgate/service"
	"github.com/posener/wstest"
)

type Session struct {
	*NATSClient
	s     *service.Service
	d     *websocket.Dialer
	conns map[*Conn]struct{}
}

func Setup() *Session {
	c := &NATSClient{}
	serv := service.NewService(c, TestConfig())

	s := &Session{
		NATSClient: c,
		d:          wstest.NewDialer(serv.GetWSHandlerFunc()),
		s:          serv,
		conns:      make(map[*Conn]struct{}),
	}

	if err := serv.Start(); err != nil {
		panic("test: failed to start server: " + err.Error())
	}
	return s
}

func (s *Session) Connect() *Conn {
	c, _, err := s.d.Dial("ws://example.org/", nil)
	if err != nil {
		panic(err)
	}
	return &Conn{
		c: c,
	}
}

func Teardown(s *Session) {
	for conn := range s.conns {
		conn.Disconnect()
	}
	st := s.s.StopChannel()
	go s.s.Stop(nil)

	select {
	case <-st:
	case <-time.After(3 * time.Second):
		panic("test: failed to stop server: timeout")
	}
}

func TestConfig() service.Config {
	var cfg service.Config
	cfg.SetDefault()
	cfg.NoHTTP = true
	return cfg
}
