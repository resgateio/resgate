package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/resgateio/resgate/server/codec"
	"github.com/resgateio/resgate/server/mq"
	"github.com/resgateio/resgate/server/rescache"
	"github.com/resgateio/resgate/server/reserr"
	"github.com/resgateio/resgate/server/rpc"
	"github.com/rs/xid"
)

type wsConn struct {
	cid         string
	ws          *websocket.Conn
	request     *http.Request
	token       json.RawMessage
	tid         string
	serv        *Service
	subs        map[string]*Subscription
	disposing   bool
	mqSub       mq.Unsubscriber
	connStr     string
	protocolVer int

	queue []func()
	work  chan struct{}

	mu sync.Mutex
}

var (
	errInvalidNewResourceResponse = reserr.InternalError(errors.New("non-resource response on new request"))
)

func (s *Service) newWSConn(ws *websocket.Conn, request *http.Request, protocol int) *wsConn {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we are stopped or are stopping
	if s.stop == nil || s.stopping {
		return nil
	}

	conn := &wsConn{
		cid:         xid.New().String(),
		ws:          ws,
		request:     request,
		serv:        s,
		subs:        make(map[string]*Subscription),
		queue:       make([]func(), 0, WSConnWorkerQueueSize),
		work:        make(chan struct{}, 1),
		protocolVer: protocol,
	}
	conn.connStr = "[" + conn.cid + "]"

	s.conns[conn.cid] = conn
	s.wg.Add(1)

	// Start an output worker that handles calls to wsConn.Enqueue and wsConn.EnqueueSend
	go conn.outputWorker()

	// Subscribe to conn events on the mq
	conn.subscribeConn()
	s.cache.AddConn(conn)

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

func (c *wsConn) ProtocolVersion() int {
	return c.protocolVer
}

func (c *wsConn) listen() {
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

	c.serv.cache.RemoveConn(c)
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
	return c.connStr
}

// Logf writes a formatted log message
func (c *wsConn) Logf(format string, v ...interface{}) {
	c.serv.logger.Log(fmt.Sprintf(c.connStr+" "+format, v...))
}

// Errorf writes a formatted log message
func (c *wsConn) Errorf(format string, v ...interface{}) {
	c.serv.logger.Error(fmt.Sprintf(c.connStr+" "+format, v...))
}

// Debugf writes a formatted log message
func (c *wsConn) Debugf(format string, v ...interface{}) {
	if c.serv.logger.IsDebug() {
		c.serv.logger.Debug(fmt.Sprintf(c.connStr+" "+format, v...))
	}
}

// Tracef writes a formatted trace message
func (c *wsConn) Tracef(format string, v ...interface{}) {
	if c.serv.logger.IsTrace() {
		c.serv.logger.Trace(fmt.Sprintf(c.connStr+" "+format, v...))
	}
}

// Disconnect closes the websocket connection.
func (c *wsConn) Disconnect(reason string) {
	if c.ws != nil {
		c.Tracef("Disconnecting - %s", reason)
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
		c.Tracef("<<- %s", data)
		c.ws.WriteMessage(websocket.TextMessage, data)
	}
}

func (c *wsConn) Reply(data []byte) {
	if c.ws != nil {
		c.Tracef("<-- %s", data)
		c.ws.WriteMessage(websocket.TextMessage, data)
	}
}

func (c *wsConn) GetResource(rid string, cb func(data *rpc.Resources, err error)) {
	sub, err := c.Subscribe(rid, true, nil, nil)
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

		sub.OnReady(func() {
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

func (c *wsConn) SetVersion(protocol string) (string, error) {
	// Quick exit on empty protocol
	if protocol == "" {
		return ProtocolVersion, nil
	}

	parts := strings.Split(protocol, ".")
	if len(parts) != 3 {
		return "", reserr.ErrInvalidParams
	}

	v := 0
	for i := 0; i < 3; i++ {
		p, err := strconv.Atoi(parts[i])
		if err != nil || p >= 1000 {
			return "", reserr.ErrInvalidParams
		}
		v *= 1000
		v += p
	}

	if v < 1000000 || v >= 2000000 {
		return "", reserr.ErrUnsupportedProtocol
	}

	c.protocolVer = v

	return ProtocolVersion, nil
}

func (c *wsConn) GetSubscription(rid string, cb func(sub *Subscription, err error)) {
	sub, err := c.Subscribe(rid, true, nil, nil)
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

		sub.OnReady(func() {
			err := sub.Error()
			if err != nil {
				cb(nil, err)
				return
			}
			cb(sub, nil)
			sub.ReleaseRPCResources()
			c.Unsubscribe(sub, true, 1, true)
		})
	})
}

func (c *wsConn) SubscribeResource(rid string, cb func(data *rpc.Resources, err error)) {
	sub, err := c.Subscribe(rid, true, nil, nil)
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

		sub.OnReady(func() {
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

func (c *wsConn) CallResource(rid, action string, params interface{}, cb func(result interface{}, err error)) {
	c.call(rid, action, params, func(result json.RawMessage, refRID string, err error) {
		c.handleCallAuthResponse(result, refRID, err, cb)
	})
}

func (c *wsConn) CallHTTPResource(rid, prefix, action string, params interface{}, cb func(result json.RawMessage, href string, err error)) {
	c.call(rid, action, params, func(result json.RawMessage, refRID string, err error) {
		if err != nil {
			cb(nil, "", err)
		} else if refRID != "" {
			cb(nil, RIDToPath(refRID, prefix), nil)
		} else {
			cb(result, "", nil)
		}
	})
}

func (c *wsConn) call(rid, action string, params interface{}, cb func(result json.RawMessage, refRID string, err error)) {
	sub, ok := c.subs[rid]
	if !ok {
		sub = NewSubscription(c, rid, nil)
	}

	sub.CanCall(action, func(err error) {
		if err != nil {
			cb(nil, "", err)
			return
		}
		c.serv.cache.Call(c, sub.ResourceName(), sub.ResourceQuery(), action, c.token, params, func(result json.RawMessage, refRID string, err error) {
			c.Enqueue(func() {
				cb(result, refRID, err)
			})
		})
	})
}

func (c *wsConn) AuthResource(rid, action string, params interface{}, cb func(result interface{}, err error)) {
	rname, query := parseRID(c.ExpandCID(rid))
	c.serv.cache.Auth(c, rname, query, action, c.token, params, func(result json.RawMessage, refRID string, err error) {
		c.Enqueue(func() {
			c.handleCallAuthResponse(result, refRID, err, cb)
		})
	})
}

func (c *wsConn) NewResource(rid string, params interface{}, cb func(result interface{}, err error)) {
	c.call(rid, "new", params, func(result json.RawMessage, refRID string, err error) {
		if err != nil {
			cb(nil, err)
			return
		}

		if refRID == "" {
			cb(nil, errInvalidNewResourceResponse)
			return
		}

		// Handle resource result
		c.handleResourceResult(refRID, cb)
	})
}

func (c *wsConn) handleCallAuthResponse(result json.RawMessage, refRID string, err error, cb func(result interface{}, err error)) {
	if err != nil {
		cb(nil, err)
		return
	}

	// Legacy behavior
	if c.protocolVer < versionCallResourceResponse {
		// Handle resource response by just returning the resource ID without subscription
		if refRID != "" {
			cb(rpc.CallResourceResult{RID: refRID}, nil)
		} else {
			cb(result, err)
		}
		return
	}

	// Handle payload result
	if refRID == "" {
		cb(rpc.CallPayloadResult{Payload: result}, nil)
		return
	}

	// Handle resource result
	c.handleResourceResult(refRID, cb)
}

func (c *wsConn) handleResourceResult(refRID string, cb func(result interface{}, err error)) {
	sub, err := c.Subscribe(refRID, true, nil, nil)
	if err != nil {
		cb(nil, err)
		return
	}
	sub.CanGet(func(err error) {
		if err != nil {
			// Respond with success even if the client is not allowed to get
			// the referenced resource, as the call in itself succeeded.
			// But the resource is the access error.
			cb(rpc.CallResourceResult{
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

		sub.OnReady(func() {
			// Respond with success even if subscription contains errors,
			// as the call in itself succeeded.
			cb(&rpc.CallResourceResult{
				RID:       sub.RID(),
				Resources: sub.GetRPCResources(),
			}, nil)
			sub.ReleaseRPCResources()
		})
	})
}

func (c *wsConn) UnsubscribeResource(rid string, count int, cb func(ok bool)) {
	cb(c.UnsubscribeByRID(rid, count))
}

func (c *wsConn) subscribe(rid string, direct bool, t *rescache.Throttle, requestHeaders map[string][]string) (*Subscription, error) {

	sub, ok := c.subs[rid]
	if ok {
		err := c.addCount(sub, direct)
		return sub, err
	}

	// Create a new throttle if needed
	if t == nil {
		limit := c.serv.cfg.ReferenceThrottle
		if limit > 0 {
			t = rescache.NewThrottle(limit)
		}
	}

	sub = NewSubscription(c, rid, t)
	_ = c.addCount(sub, direct)
	c.serv.cache.Subscribe(sub, t, requestHeaders)

	c.subs[rid] = sub
	return sub, nil
}

// subscribe gets existing subscription or creates a new one to cache
// Will return error if number of allowed subscriptions for the resource is exceeded
func (c *wsConn) Subscribe(rid string, direct bool, t *rescache.Throttle, requestHeaders map[string][]string) (*Subscription, error) {
	if c.disposing {
		return nil, reserr.ErrDisposing
	}

	return c.subscribe(rid, direct, t, requestHeaders)
}

// unsubscribe counts down the subscription counter
// and deletes the subscription if the count reached 0.
func (c *wsConn) Unsubscribe(sub *Subscription, direct bool, count int, tryDelete bool) {
	if c.disposing {
		return
	}

	c.removeCount(sub, direct, count, tryDelete)
}

func (c *wsConn) UnsubscribeByRID(rid string, count int) bool {
	if c.disposing {
		return false
	}

	sub, ok := c.subs[rid]
	if !ok || sub.direct < count {
		return false
	}

	c.removeCount(sub, true, count, true)
	return true
}

func (c *wsConn) addCount(s *Subscription, direct bool) error {
	if direct {
		if s.direct >= SubscriptionCountLimit {
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

func (c *wsConn) setToken(token json.RawMessage, tid string) {
	c.tid = tid

	if c.token == nil {
		// No need to revalidate nil token access
		c.token = token
		return
	}

	c.token = token
	for _, sub := range c.subs {
		sub.reaccess(nil)
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

		if cap(c.queue) > WSConnWorkerQueueSize {
			c.queue = make([]func(), 0, WSConnWorkerQueueSize)
		} else {
			c.queue = c.queue[0:0]
		}
		c.mu.Unlock()
	}

	c.queue = nil
}

func (c *wsConn) subscribeConn() {
	mqSub, err := c.serv.mq.Subscribe("conn."+c.cid, func(subj string, payload []byte, responseHeaders map[string][]string, _ error) {
		c.Enqueue(func() {
			idx := len(c.cid) + 6 // Length of "conn." + "."
			if idx >= len(subj) {
				c.Errorf("Error processing conn event %s: malformed event subject", subj)
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
		c.Errorf("Error subscribing to conn events: %s", err)
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
		c.Errorf("Error processing token event: malformed event payload: %s", err)
		return
	}

	c.setToken(te.Token, te.TID)
}

func (c *wsConn) ExpandCID(rid string) string {
	return strings.Replace(rid, CIDPlaceholder, c.cid, -1)
}

func (c *wsConn) TokenReset(tids map[string]bool, subject string) {
	c.Enqueue(func() {
		// Exit if no token ID is set, or if it isn't affected.
		if c.tid == "" || !tids[c.tid] {
			return
		}
		c.serv.cache.CustomAuth(c, subject, "", c.token, nil, func(_ json.RawMessage, _ string, err error) {
			// Discard response, but log an error if auth request timed out.
			if err == mq.ErrRequestTimeout {
				c.Errorf("Token reset auth request timeout on subject: %s", subject)
			}
		})
	})
}
