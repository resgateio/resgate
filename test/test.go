package test

import (
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

	s.Connect()
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
	s.Close()
}

func TestConfig() service.Config {
	var cfg service.Config
	cfg.SetDefault()
	return cfg
}
