package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/jirenius/resgate/server/codec"
	"github.com/jirenius/resgate/server/httpapi"
	"github.com/jirenius/resgate/server/mq"
	"github.com/jirenius/resgate/server/rescache"
	"github.com/jirenius/resgate/server/reserr"
	"github.com/jirenius/resgate/server/rpc"
	"github.com/rs/xid"
)

type wsConn struct {
	cid       string
	ws        *websocket.Conn
	request   *http.Request
	token     json.RawMessage
	serv      *Service
	logPrefix string
	subs      map[string]*Subscription
	disposing bool
	mqSub     mq.Unsubscriber

	queue []func()
	work  chan struct{}

	mu sync.Mutex
}

const wsConnWorkerQueueSize = 256
const cidPlaceholder = "{cid}"

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
	conn.logPrefix = conn.String() + " "

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
	c.Tracef("Connected")

	var in []byte
	var err error

	// Loop until an error is returned when reading
	for {
		if _, in, err = c.ws.ReadMessage(); err != nil {
			break
		}

		c.Tracef("--> %s", in)
		in := in
		c.Enqueue(func() {
			rpc.HandleRequest(in, c)
		})
	}

	c.Dispose()
	c.Tracef("Disconnected: %s", err)
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

// Logf writes a formatted log message
func (c *wsConn) Logf(format string, v ...interface{}) {
	if c.serv.logger == nil {
		return
	}
	c.serv.logger.Logf(c.logPrefix, format, v...)
}

// Debugf writes a formatted log message
func (c *wsConn) Debugf(format string, v ...interface{}) {
	if c.serv.logger == nil {
		return
	}
	c.serv.logger.Debugf(c.logPrefix, format, v...)
}

// Tracef writes a formatted trace message
func (c *wsConn) Tracef(format string, v ...interface{}) {
	if c.serv.logger == nil {
		return
	}
	c.serv.logger.Tracef(c.logPrefix, format, v...)
}

// Disconnect closes the websocket connection.
func (c *wsConn) Disconnect(reason string) {
	if c.ws != nil {
		c.Debugf("Disconnecting - %s", reason)
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
		c.Tracef("<-- %s", data)
		c.ws.WriteMessage(websocket.TextMessage, data)
	}
}

func (c *wsConn) GetResource(rid string, cb func(data *rpc.Resources, err error)) {
	sub, err := c.Subscribe(rid, true, nil)
	if err != nil {
		cb(nil, err)
		return
	}

	sub.CanGet(func(err error) {
		if err != nil {
			cb(nil, err)
			c.Unsubscribe(sub, true, 1, true)
			return
		}

		sub.OnLoaded(func(sub *Subscription) {
			err := sub.Error()
			if err != nil {
				cb(nil, err)
				return
			}

			cb(sub.GetRPCResources(), nil)
			sub.ReleaseRPCResources()
			c.Unsubscribe(sub, true, 1, true)
		})
	})
}

func (c *wsConn) GetHTTPResource(rid string, prefix string, cb func(data interface{}, err error)) {
	sub, err := c.Subscribe(rid, true, nil)
	if err != nil {
		cb(nil, err)
		return
	}

	sub.CanGet(func(err error) {
		if err != nil {
			cb(nil, err)
			c.Unsubscribe(sub, true, 1, true)
			return
		}

		sub.OnLoaded(func(sub *Subscription) {
			c.outputHTTPResource(prefix, sub, cb)
			c.Unsubscribe(sub, true, 1, true)
		})
	})
}

func (c *wsConn) outputHTTPResource(prefix string, sub *Subscription, cb func(data interface{}, err error)) {
	err := sub.Error()
	if err != nil {
		cb(nil, err)
		return
	}

	r := sub.GetHTTPResource(prefix, make([]string, 0, 32))

	// Select which part of the httpapi.Resource
	// that is to be sent in the response.
	var data interface{}
	switch {
	case r.Model != nil:
		data = r.Model
	case r.Collection != nil:
		data = r.Collection
	default:
		data = r
	}
	cb(data, nil)
	sub.ReleaseRPCResources()
}

func (c *wsConn) SubscribeResource(rid string, cb func(data *rpc.Resources, err error)) {
	sub, err := c.Subscribe(rid, true, nil)
	if err != nil {
		cb(nil, err)
		return
	}

	sub.CanGet(func(err error) {
		if err != nil {
			cb(nil, err)
			c.Unsubscribe(sub, true, 1, true)
			return
		}

		sub.OnLoaded(func(sub *Subscription) {
			err := sub.Error()
			if err != nil {
				cb(nil, err)
				c.Unsubscribe(sub, true, 1, true)
				return
			}

			cb(sub.GetRPCResources(), nil)
			sub.ReleaseRPCResources()
		})
	})
}

func (c *wsConn) CallResource(rid, action string, params interface{}, callback func(result interface{}, err error)) {
	c.call(rid, action, params, callback)
}

func (c *wsConn) NewResource(rid string, params interface{}, cb func(result *rpc.NewResult, err error)) {
	c.callNew(rid, params, func(newRID string, err error) {
		c.Enqueue(func() {
			if err != nil {
				cb(nil, err)
				return
			}

			sub, err := c.Subscribe(newRID, true, nil)
			if err != nil {
				cb(nil, err)
				return
			}

			sub.CanGet(func(err error) {
				if err != nil {
					// Respond with success even if the client is not allowed to get
					// the resource it just created, as the call to 'new'
					// atleast succeeded. But the resource is the access error.
					cb(&rpc.NewResult{
						RID: sub.RID(),
						Resources: &rpc.Resources{
							Errors: map[string]*reserr.Error{
								sub.RID(): reserr.RESError(err),
							},
						},
					}, nil)
					c.Unsubscribe(sub, true, 1, true)
					return
				}

				sub.OnLoaded(func(sub *Subscription) {
					// Respond with success even if subscription contains errors,
					// as the call to 'new' atleast succeeded.
					cb(&rpc.NewResult{
						RID:       sub.RID(),
						Resources: sub.GetRPCResources(),
					}, nil)
					sub.ReleaseRPCResources()
				})
			})
		})
	})
}

func (c *wsConn) NewHTTPResource(rid, prefix string, params interface{}, cb func(href string, err error)) {
	c.callNew(rid, params, func(newRID string, err error) {
		c.Enqueue(func() {
			if err != nil {
				cb("", err)
			} else {
				cb(httpapi.RIDToPath(newRID, prefix), nil)
			}
		})
	})
}

func (c *wsConn) AuthResource(rid, action string, params interface{}, callback func(result interface{}, err error)) {
	rname, query := parseRID(c.ExpandCID(rid))
	c.serv.cache.Auth(c, rname, query, action, c.token, params, func(result json.RawMessage, err error) {
		c.Enqueue(func() {
			callback(result, err)
		})
	})
}

func (c *wsConn) UnsubscribeResource(rid string, cb func(ok bool)) {
	cb(c.UnsubscribeByRID(rid))
}

func (c *wsConn) call(rid, action string, params interface{}, cb func(result interface{}, err error)) {
	sub, ok := c.subs[rid]
	if !ok {
		sub = NewSubscription(c, rid, nil)
	}

	sub.CanCall(action, func(err error) {
		if err != nil {
			cb(nil, err)
		} else {
			c.serv.cache.Call(c, sub.ResourceName(), sub.ResourceQuery(), action, c.token, params, func(result json.RawMessage, err error) {
				c.Enqueue(func() {
					cb(result, err)
				})
			})
		}
	})
}

func (c *wsConn) callNew(rid string, params interface{}, cb func(newRID string, err error)) {
	sub, ok := c.subs[rid]
	if !ok {
		sub = NewSubscription(c, rid, nil)
	}

	sub.CanCall("new", func(err error) {
		if err != nil {
			cb("", err)
			return
		}

		c.serv.cache.CallNew(c, sub.ResourceName(), sub.ResourceQuery(), c.token, params, cb)
	})
}

func (c *wsConn) subscribe(rid string, direct bool, path []string) (*Subscription, error) {

	sub, ok := c.subs[rid]
	if ok {
		err := c.addCount(sub, direct)
		return sub, err
	}

	sub = NewSubscription(c, rid, path)
	_ = c.addCount(sub, direct)
	c.serv.cache.Subscribe(sub)

	c.subs[rid] = sub
	return sub, nil
}

// subscribe gets existing subscription or creates a new one to cache
// Will return error if number of allowed subscriptions for the resource is exceeded
func (c *wsConn) Subscribe(rid string, direct bool, path []string) (*Subscription, error) {
	if c.disposing {
		return nil, reserr.ErrDisposing
	}

	return c.subscribe(rid, direct, path)
}

// unsubscribe counts down the subscription counter
// and deletes the subscription if the count reached 0.
func (c *wsConn) Unsubscribe(sub *Subscription, direct bool, count int, tryDelete bool) {
	if c.disposing {
		return
	}

	c.removeCount(sub, direct, count, tryDelete)
}

func (c *wsConn) UnsubscribeByRID(rid string) bool {
	if c.disposing {
		return false
	}

	sub, ok := c.subs[rid]
	if !ok || sub.direct == 0 {
		return false
	}

	c.removeCount(sub, true, 1, true)
	return true
}

func (c *wsConn) addCount(s *Subscription, direct bool) error {
	if direct {
		if s.direct >= subscriptionCountLimit {
			c.Debugf("Subscription %s: Subscription limit exceeded (%d)", s.RID(), s.direct)
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
func (c *wsConn) removeCount(s *Subscription, direct bool, count int, tryDelete bool) {
	if s.direct+s.indirect == 0 {
		return
	}

	if direct {
		s.direct -= count
	} else {
		s.indirect -= count
	}

	if tryDelete {
		c.tryDelete(s)
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
		sub.reaccess()
	}
}

func (c *wsConn) Access(s *Subscription, cb func(*rescache.Access)) {
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
				c.Debugf("Error processing conn event %s: malformed event subject", subj)
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
		c.Debugf("Error processing conn event: malformed event payload: %s", err)
		return
	}

	c.setToken(te.Token)
}

func (c *wsConn) ExpandCID(rid string) string {
	return strings.Replace(rid, cidPlaceholder, c.cid, -1)
}
