package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"../mq"
	"../mq/codec"
	"../reserr"
	"../resourceCache"
	"../rpc"
	"github.com/gorilla/websocket"
	"github.com/rs/xid"
)

type wsConn struct {
	cid       string
	ws        *websocket.Conn
	request   *http.Request
	token     json.RawMessage
	serv      *Service
	logger    *log.Logger
	subs      map[string]*Subscription
	disposing bool
	mqSub     mq.Unsubscriber

	queue []func()
	work  chan struct{}

	mu sync.Mutex
}

var wsConnChannelSize = 32
var wsConnWorkerQueueSize = 256

func (s *Service) newWSConn(ws *websocket.Conn, request *http.Request) *wsConn {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we are stopped or are stopping
	if s.stop == nil || s.stopping {
		return nil
	}

	cid := xid.New()

	conn := &wsConn{
		cid:     cid.String(),
		ws:      ws,
		request: request,
		serv:    s,
		subs:    make(map[string]*Subscription),
		queue:   make([]func(), 0, wsConnWorkerQueueSize),
		work:    make(chan struct{}, 1),
	}
	conn.logger = log.New(os.Stdout, conn.String()+" ", s.logFlags)

	s.conns[conn.cid] = conn
	s.wg.Add(1)

	// Start an output worker that handles calls to wsConn.Enqueue and wsConn.EnqueueSend
	go conn.outputWorker()

	// Subscribe to conn events on the mq
	conn.subscribeConn()

	return conn
}

func (c *wsConn) CID() string {
	return c.cid
}

func (c *wsConn) Token() json.RawMessage {
	return c.token
}

func (c *wsConn) HTTPRequest() *http.Request {
	return c.request
}

func (c *wsConn) listen() {
	c.Log("Connected")

	var in []byte
	var err error

	// Loop until an error is returned when reading
	for {
		if _, in, err = c.ws.ReadMessage(); err != nil {
			break
		}

		if debug {
			c.Logf("--> %s", in)
		}
		in := in
		c.Enqueue(func() {
			rpc.HandleRequest(in, c)
		})
	}

	c.Dispose()
	c.Logf("Disconnected: %s", err)
}

// dispose closes the wsConn worker and disposes all subscription.
// Returns false if dispose has already been called, otherwise true.
func (c *wsConn) dispose() {
	if c.disposing {
		return
	}

	c.mu.Lock()
	c.disposing = true
	close(c.work)
	c.mu.Unlock()

	c.unsubscribeConn()

	subs := c.subs
	c.subs = nil
	for _, sub := range subs {
		sub.Dispose()
	}

	c.serv.mu.Lock()
	defer c.serv.mu.Unlock()

	c.serv.wg.Done()
	delete(c.serv.conns, c.cid)
}

func (c *wsConn) Dispose() {
	done := make(chan struct{})
	if c.Enqueue(func() {
		c.dispose()
		close(done)
	}) {
		<-done
	}
}

func (c *wsConn) String() string {
	return fmt.Sprintf("[%s]", c.cid)
}

// Log writes a log message
func (c *wsConn) Log(v ...interface{}) {
	c.logger.Print(v...)
}

// Logf writes a formatted log message
func (c *wsConn) Logf(format string, v ...interface{}) {
	c.logger.Printf(format, v...)
}

// Disconnect closes the websocket connection.
func (c *wsConn) Disconnect(reason string) {
	if c.ws != nil {
		c.Logf("Disconnecting - %s", reason)
		c.ws.Close()
	}
}

// Enqueue puts the callback function in queue to be called
// by the wsConn worker goroutine.
// It returns false if the function was not queued due to
// either the connection is disposing, or it is a slow consumer.
func (c *wsConn) Enqueue(f func()) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.disposing {
		return false
	}
	c.enqueue(f)
	return true
}

func (c *wsConn) enqueue(f func()) {
	count := len(c.queue)
	c.queue = append(c.queue, f)

	// If the queue was empty, the worker is idling
	// Let's wake it up.
	if count == 0 {
		c.work <- struct{}{}
	}
}

func (c *wsConn) Send(data []byte) {
	if c.ws != nil {
		if debug {
			c.Logf("<-- %s", data)
		}
		c.ws.WriteMessage(websocket.TextMessage, data)
	}
}

func (c *wsConn) GetResource(resourceID string, cb func(data interface{}, err error)) {
	sub, err := c.Subscribe(resourceID, true)
	if err != nil {
		cb(nil, err)
		return
	}

	sub.CanGet(func(err error) {
		if err != nil {
			cb(nil, err)
			c.Unsubscribe(sub, true, 1)
			return
		}

		sub.OnLoaded(func(sub *Subscription) {
			r := sub.GetRPCResource()

			if r.Error != nil {
				cb(nil, r.Error)
			} else {
				cb(r.Data, nil)
			}

			sub.ReleaseRPCResource()
			c.Unsubscribe(sub, true, 1)
		})
	})
}

func (c *wsConn) GetHTTPResource(resourceID string, prefix string, cb func(data interface{}, err error)) {
	sub, err := c.Subscribe(resourceID, true)
	if err != nil {
		cb(nil, err)
		return
	}

	sub.CanGet(func(err error) {
		if err != nil {
			cb(nil, err)
			c.Unsubscribe(sub, true, 1)
			return
		}

		sub.OnLoaded(func(sub *Subscription) {
			r := sub.GetHTTPResource(prefix)

			if r.Error != nil {
				cb(nil, r.Error)
			} else {
				cb(r.Data, nil)
			}

			sub.ReleaseRPCResource()
			c.Unsubscribe(sub, true, 1)
		})
	})
}

func (c *wsConn) SubscribeResource(resourceID string, cb func(data interface{}, err error)) {
	sub, err := c.Subscribe(resourceID, true)
	if err != nil {
		cb(nil, err)
		return
	}

	sub.CanGet(func(err error) {
		if err != nil {
			cb(nil, err)
			c.Unsubscribe(sub, true, 1)
			return
		}

		sub.OnLoaded(func(sub *Subscription) {
			r := sub.GetRPCResource()
			defer sub.ReleaseRPCResource()

			if r.Error != nil {
				cb(nil, r.Error)
				c.Unsubscribe(sub, true, 1)
			} else {
				cb(r.Data, nil)
			}
		})
	})
}

func (c *wsConn) CallResource(resourceID, action string, params interface{}, callback func(result interface{}, err error)) {
	c.call(resourceID, action, params, callback)
}

func (c *wsConn) AuthResource(resourceID, action string, params interface{}, callback func(result interface{}, err error)) {
	c.serv.cache.Auth(c, resourceID, action, c.token, params, func(token json.RawMessage, result json.RawMessage, err error) {
		c.Enqueue(func() {
			c.setToken(token)
			callback(result, err)
		})
	})
}

func (c *wsConn) UnsubscribeResource(resourceID string, cb func(ok bool)) {
	cb(c.UnsubscribeById(resourceID))
}

func (c *wsConn) call(resourceID string, action string, params interface{}, cb func(result interface{}, err error)) {
	sub, ok := c.subs[resourceID]
	if !ok {
		sub = NewSubscription(c, resourceID)
	}

	sub.CanCall(action, func(err error) {
		if err != nil {
			cb(nil, err)
		} else {
			c.serv.cache.Call(c, resourceID, action, c.token, params, func(result interface{}, err error) {
				c.Enqueue(func() {
					cb(result, err)
				})
			})
		}
	})
}

func (c *wsConn) subscribe(resourceID string, direct bool) (*Subscription, error) {

	sub, ok := c.subs[resourceID]
	if ok {
		err := c.addCount(sub, direct)
		return sub, err
	}

	sub = NewSubscription(c, resourceID)
	_ = c.addCount(sub, direct)
	c.serv.cache.Subscribe(sub)

	c.subs[resourceID] = sub
	return sub, nil
}

// subscribe gets existing subscription or creates a new one to cache
// Will return error if number of allowed subscriptions for the resource is exceeded
func (c *wsConn) Subscribe(resourceID string, direct bool) (*Subscription, error) {
	if c.disposing {
		return nil, reserr.ErrDisposing
	}

	return c.subscribe(resourceID, direct)
}

func (c *wsConn) SubscribeAll(resourceIDs []string) ([]*Subscription, error) {
	if c.disposing {
		return nil, reserr.ErrDisposing
	}

	subs := make([]*Subscription, len(resourceIDs))
	for i, resourceID := range resourceIDs {
		sub, err := c.subscribe(resourceID, false)

		if err != nil {
			// In case of subscribe error,
			// we unsubscribe to all and exit with error
			c.Logf("Failed to subscribe to %s. Aborting subscribeAll")
			for j := 0; j < i; j++ {
				s := subs[j]
				c.removeCount(s, false, 1)
			}
			return nil, err
		}
		subs[i] = sub
	}

	return subs, nil
}

// unsubscribe counts down the subscription counter
// and deletes the subscription if the count reached 0.
func (c *wsConn) Unsubscribe(sub *Subscription, direct bool, count int) {
	if c.disposing {
		return
	}

	c.removeCount(sub, direct, count)
}

func (c *wsConn) UnsubscribeById(resourceID string) bool {
	if c.disposing {
		return false
	}

	sub, ok := c.subs[resourceID]
	if !ok || sub.direct == 0 {
		return false
	}

	c.removeCount(sub, true, 1)
	return true
}

func (c *wsConn) UnsubscribeAll(subs []*Subscription) {
	if c.disposing {
		return
	}

	c.unsubscribeAll(subs, false, 1)
}

func (c *wsConn) unsubscribeAll(subs []*Subscription, direct bool, count int) {
	for _, sub := range subs {
		c.removeCount(sub, direct, count)
	}
}

func (c *wsConn) addCount(s *Subscription, direct bool) error {
	if direct {
		if s.direct >= subscriptionCountLimit {
			c.Logf("Subscription %s: Subscription limit exceeded (%d)", s.ResourceID(), s.direct)
			return errSubscriptionLimitExceeded
		}

		s.direct++
	} else {
		s.indirect++
	}

	return nil
}

// removeCount decreases the subscription count and disposes the subscription
// if indirect and direct subscription count reaches 0
func (c *wsConn) removeCount(s *Subscription, direct bool, count int) {
	if s.direct+s.indirect == 0 {
		return
	}

	if direct {
		s.direct -= count
	} else {
		s.indirect -= count
	}

	if s.direct+s.indirect == 0 {
		s.Dispose()
		delete(c.subs, s.ResourceID())
	}
}

func (c *wsConn) setToken(token json.RawMessage) {
	if c.token == nil {
		// No need to revalidate nil token access
		c.token = token
		return
	}

	c.token = token
	for _, sub := range c.subs {
		sub.Reaccess()
	}
}

func (c *wsConn) Access(s *Subscription, cb func(*resourceCache.Access)) {
	c.serv.cache.Access(s, c.token, cb)
}

func (c *wsConn) outputWorker() {
	for range c.work {
		idx := 0
		var f func()
		c.mu.Lock()
		for len(c.queue) > idx {
			f = c.queue[idx]
			c.mu.Unlock()
			f()
			idx++
			c.mu.Lock()
		}

		if cap(c.queue) > wsConnWorkerQueueSize {
			c.queue = make([]func(), 0, wsConnWorkerQueueSize)
		} else {
			c.queue = c.queue[0:0]
		}
		c.mu.Unlock()
	}

	c.queue = nil
}

func (c *wsConn) subscribeConn() {
	mqSub, err := c.serv.mq.Subscribe("conn."+c.cid, func(subj string, payload []byte, _ error) {
		c.Enqueue(func() {
			idx := len(c.cid) + 6 // Length of "conn." + "."
			if idx >= len(subj) {
				c.Logf("Error processing conn event %s: malformed event subject", subj)
				return
			}

			event := subj[idx:]

			switch event {
			case "token":
				c.handleConnToken(payload)
			}
		})
	})

	if err != nil {
		c.Logf("Error subscribing to conn events: %s", err)
	}

	c.mqSub = mqSub
}

func (c *wsConn) unsubscribeConn() {
	if c.mqSub != nil {
		c.mqSub.Unsubscribe()
	}
}

func (c *wsConn) handleConnToken(payload []byte) {
	te, err := codec.DecodeConnTokenEvent(payload)
	if err != nil {
		c.Logf("Error processing conn event: malformed event payload: %s", err)
		return
	}

	c.setToken(te.Token)
}
