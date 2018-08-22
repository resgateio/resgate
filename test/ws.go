package test

import "github.com/gorilla/websocket"

type Conn struct {
	c    *websocket.Conn
	reqs map[int64]*ClientRequest
}

var clientRequestID = 0

type ClientRequest struct {
	c *Conn
}

func (c *Conn) Request() *ClientRequest {
	return &ClientRequest{
		c: c,
	}
}

func (c *Conn) Disconnect() {
	c.Disconnect()
}
