package test

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime/pprof"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jirenius/resgate/reserr"
)

type Conn struct {
	s    *Session
	d    *websocket.Dialer
	ws   *websocket.Conn
	reqs map[uint64]*ClientRequest
	evs  chan *ClientEvent
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
	Event  *string       `json:"event"`
	Data   interface{}   `json:"data"`
}

var clientRequestID uint64 = 0

type ClientRequest struct {
	Method string
	Params interface{}
	c      *Conn
	ch     chan *ClientResponse
}

type ClientResponse struct {
	Result interface{}
	Error  *reserr.Error
}

type ClientEvent struct {
	Event string
	Data  interface{}
}

// ParallelEvents holds multiple events in undetermined order
type ParallelEvents []*ClientEvent

func NewConn(s *Session, d *websocket.Dialer, ws *websocket.Conn) *Conn {
	c := &Conn{
		s:    s,
		d:    d,
		ws:   ws,
		reqs: make(map[uint64]*ClientRequest),
		evs:  make(chan *ClientEvent, 256),
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
		Method: method,
		Params: params,
		c:      c,
		ch:     make(chan *ClientResponse, 1),
	}

	c.reqs[id] = req

	return req
}

func (c *Conn) Disconnect() {
	c.Disconnect()
}

func (c *Conn) GetEvent(t *testing.T) *ClientEvent {
	select {
	case ev := <-c.evs:
		return ev
	case <-time.After(timeoutSeconds * time.Second):
		t.Fatal("expected a client event but found none")
	}
	return nil
}

// GetParallelEvents gets n number of events where the order is uncertain.
func (c *Conn) GetParallelEvents(t *testing.T, n int) ParallelEvents {
	pev := make(ParallelEvents, n)
	for i := 0; i < n; i++ {
		pev[i] = c.GetEvent(t)
	}
	return pev
}

// AssertNoEvent assert that no events are queued
func (c *Conn) AssertNoEvent(t *testing.T, rid string) {
	// Quick check if an event already exists
	select {
	case ev := <-c.evs:
		t.Fatalf("expected no client event, but found %#v", ev.Event)
	default:
	}

	// Flush out events by sending an auth on the resource
	// We use auth as it requires no access check, but will
	// be processed by the same goroutine as events.
	creq := c.Request("auth."+rid+".foo", nil)
	req := c.s.GetRequest(t)
	req.AssertSubject(t, "auth."+rid+".foo")
	req.RespondSuccess(nil)
	creq.GetResponse(t)

	// Check if an event has arrived meanwhile
	select {
	case ev := <-c.evs:
		t.Fatalf("expected no client event, but found %#v", ev.Event)
	default:
	}
}

// AssertNoNATSRequest assert that no request are queued on NATS
func (c *Conn) AssertNoNATSRequest(t *testing.T, rid string) {
	// Flush out requests by sending an auth on the resource
	// and validate it is the request next in queue.
	creq := c.Request("auth."+rid+".foo", nil)
	req := c.s.GetRequest(t)
	if req.Subject != "auth."+rid+".foo" {
		t.Fatalf("expected no NATS request, but found %#v", req.Subject)
	}
	req.RespondSuccess(nil)
	creq.GetResponse(t)
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
		// Check if it is an event
		if cr.Event != nil {
			c.evs <- &ClientEvent{
				Event: *cr.Event,
				Data:  cr.Data,
			}
			c.mu.Unlock()
		} else {
			req, ok := c.reqs[cr.ID]
			if !ok {
				c.mu.Unlock()
				panic("test: response without matching request")
			}
			delete(c.reqs, cr.ID)
			c.mu.Unlock()
			select {
			case req.ch <- &ClientResponse{
				Result: cr.Result,
				Error:  cr.Error,
			}:
			default:
				panic("test: failed to write client response")
			}
		}
	}
}

// GetResponse awaits for a response and returns it.
// Fails if a response hasn't arrived within 1 second.
func (cr *ClientRequest) GetResponse(t *testing.T) *ClientResponse {
	select {
	case resp := <-cr.ch:
		return resp
	case <-time.After(timeoutSeconds * time.Second):
		if t == nil {
			pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
			panic(fmt.Sprintf("expected a response to client request %#v, but found none", cr.Method))
		} else {
			t.Fatalf("expected a response to client request %#v, but found none", cr.Method)
		}
	}
	return nil
}

// AssertResult asserts that the response has the expected result
func (cr *ClientResponse) AssertResult(t *testing.T, result interface{}) *ClientResponse {
	// Assert it is not an error
	if cr.Error != nil {
		t.Fatalf("expected successful response, but got error:\n%s: %s", cr.Error.Code, cr.Error.Message)
	}

	var err error
	rj, err := json.Marshal(result)
	if err != nil {
		panic("test: error marshalling assertion result: " + err.Error())
	}

	var r interface{}
	err = json.Unmarshal(rj, &r)
	if err != nil {
		panic("test: error unmarshalling assertion result: " + err.Error())
	}

	if !reflect.DeepEqual(r, cr.Result) {
		crj, err := json.Marshal(cr.Result)
		if err != nil {
			panic("test: error marshalling response result: " + err.Error())
		}
		t.Fatalf("expected response result to be:\n%s\nbut got:\n%s", rj, crj)
	}
	return cr
}

// AssertError asserts that the response has the expected error
func (cr *ClientResponse) AssertError(t *testing.T, err *reserr.Error) *ClientResponse {
	// Assert it is an error
	if cr.Error == nil {
		var err error
		rj, err := json.Marshal(cr.Result)
		if err != nil {
			panic("test: error marshalling response result: " + err.Error())
		}
		t.Fatalf("expected error response, but got result:\n%s", rj)
	}

	if !reflect.DeepEqual(err, cr.Error) {
		ej, err := json.Marshal(err)
		if err != nil {
			panic("test: error marshalling assertion error: " + err.Error())
		}
		cej, err := json.Marshal(cr.Error)
		if err != nil {
			panic("test: error marshalling response error: " + err.Error())
		}
		t.Fatalf("expected response result to be:\n%s\nbut got:\n%s", ej, cej)
	}
	return cr
}

// AssertErrorCode asserts that the response has the expected error code
func (cr *ClientResponse) AssertErrorCode(t *testing.T, code string) *ClientResponse {
	// Assert it is an error
	if cr.Error == nil {
		var err error
		rj, err := json.Marshal(cr.Result)
		if err != nil {
			panic("test: error marshalling response result: " + err.Error())
		}
		t.Fatalf("expected error response, but got result:\n%s", rj)
	}

	if cr.Error.Code != code {
		t.Fatalf("expected response error code to be:\n%#v\nbut got:\n%#v", code, cr.Error.Code)
	}
	return cr
}

// GetEvent returns a event based on event name.
func (pr ParallelEvents) GetEvent(t *testing.T, event string) *ClientEvent {
	for _, r := range pr {
		if r.Event == event {
			return r
		}
	}

	t.Fatalf("expected parallel events to contain %#v, but found none", event)
	return nil
}

// Equals asserts that the event has the expected event name and payload
func (ev *ClientEvent) Equals(t *testing.T, event string, data interface{}) *ClientEvent {
	ev.AssertEventName(t, event)
	ev.AssertData(t, data)
	return ev
}

// AssertEventName asserts that the event has the expected event name
func (ev *ClientEvent) AssertEventName(t *testing.T, event string) *ClientEvent {
	if ev.Event != event {
		t.Fatalf("expected event to be %#v, but got %#v", event, ev.Event)
	}
	return ev
}

// AssertData asserts that the event has the expected data
func (ev *ClientEvent) AssertData(t *testing.T, data interface{}) *ClientEvent {
	var err error
	dj, err := json.Marshal(data)
	if err != nil {
		panic("test: error marshalling assertion data: " + err.Error())
	}

	var p interface{}
	err = json.Unmarshal(dj, &p)
	if err != nil {
		panic("test: error unmarshalling assertion data: " + err.Error())
	}

	if !reflect.DeepEqual(p, ev.Data) {
		evdj, err := json.Marshal(ev.Data)
		if err != nil {
			panic("test: error marshalling event data: " + err.Error())
		}
		t.Fatalf("expected event data to be:\n%s\nbut got:\n%s", dj, evdj)
	}
	return ev
}
