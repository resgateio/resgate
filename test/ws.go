package test

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/jirenius/resgate/reserr"
)

type Conn struct {
	ws   *websocket.Conn
	reqs map[uint64]*ClientRequest
	mu   sync.Mutex
}

type clientRequest struct {
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
	ID     uint64      `json:"id"`
}

type clientResponse struct {
	Result interface{}   `json:"result"`
	Error  *reserr.Error `json:"error"`
	ID     uint64        `json:"id"`
}

var clientRequestID uint64 = 0

type ClientRequest struct {
	c  *Conn
	ch chan *ClientResponse
}

type ClientResponse struct {
	Result interface{}
	Error  *reserr.Error
}

func NewConn(ws *websocket.Conn) *Conn {
	c := &Conn{
		ws:   ws,
		reqs: make(map[uint64]*ClientRequest),
	}
	go c.listen()
	return c
}

func (c *Conn) Request(method string, params interface{}) *ClientRequest {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := clientRequestID
	clientRequestID++
	err := c.ws.WriteJSON(clientRequest{
		ID:     id,
		Method: method,
		Params: params,
	})
	if err != nil {
		panic("test: error marshalling client request: " + err.Error())
	}

	req := &ClientRequest{
		c:  c,
		ch: make(chan *ClientResponse),
	}

	c.reqs[id] = req

	return req
}

func (c *Conn) Disconnect() {
	c.Disconnect()
}

func (c *Conn) listen() {
	var in []byte
	var err error

	// Loop until an error is returned when reading
	for {
		if _, in, err = c.ws.ReadMessage(); err != nil {
			break
		}

		cr := clientResponse{}
		err := json.Unmarshal(in, &cr)
		if err != nil {
			panic("test: error unmarshalling client response: " + err.Error())
		}

		c.mu.Lock()
		req, ok := c.reqs[cr.ID]
		if !ok {
			c.mu.Unlock()
			panic("test: response without matching request")
		}
		delete(c.reqs, cr.ID)
		c.mu.Unlock()

		req.ch <- &ClientResponse{
			Result: cr.Result,
			Error:  cr.Error,
		}
	}
}
