package rescache

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jirenius/timerqueue"
	"github.com/resgateio/resgate/logger"
	"github.com/resgateio/resgate/metrics"
	"github.com/resgateio/resgate/server/codec"
	"github.com/resgateio/resgate/server/mq"
	"github.com/resgateio/resgate/server/reserr"
)

// Cache is an in memory resource cache.
type Cache struct {
	mq               mq.Client
	logger           logger.Logger
	workers          int
	resetThrottle    int
	unsubscribeDelay time.Duration
	conns            map[string]Conn

	mu         sync.Mutex
	started    bool
	eventSubs  map[string]*EventSubscription
	inCh       chan *EventSubscription
	unsubQueue *timerqueue.Queue
	resetSub   mq.Unsubscriber

	// Deprecated behavior logging
	depMutex  sync.Mutex
	depLogged map[string]featureType
}

// Subscriber interface represents a subscription made on a client connection
type Subscriber interface {
	CID() string
	Loaded(resourceSub *ResourceSubscription, responseHeaders map[string][]string, err error)
	Event(event *ResourceEvent)
	ResourceName() string
	ResourceQuery() string
	Reaccess(t *Throttle)
}

// Conn interface represents a connection listening on events
type Conn interface {
	CID() string
	TokenReset(tids map[string]bool, subject string)
}

// ResourceEvent represents an event on a resource
type ResourceEvent struct {
	Event     string
	Payload   json.RawMessage
	Idx       int
	Value     codec.Value
	Changed   map[string]codec.Value
	OldValues map[string]codec.Value
	// Version is the targeted internal version of the resource
	Version uint
	// Update flags if the event causes a version bump. Set by eg. add/remove/change.
	Update bool
}

// NewCache creates a new Cache instance
func NewCache(mq mq.Client, workers int, resetThrottle int, unsubscribeDelay time.Duration, l logger.Logger) *Cache {
	return &Cache{
		mq:               mq,
		logger:           l,
		workers:          workers,
		resetThrottle:    resetThrottle,
		unsubscribeDelay: unsubscribeDelay,
		conns:            make(map[string]Conn),
		depLogged:        make(map[string]featureType),
	}
}

// SetLogger sets the logger
func (c *Cache) SetLogger(l logger.Logger) {
	c.logger = l
}

// Start will initialize the cache, subscribing to global events
// It is assumed mq.Connect has already been called
func (c *Cache) Start() error {
	if c.started {
		return errors.New("cache: already started")
	}
	inCh := make(chan *EventSubscription, 100)
	c.eventSubs = make(map[string]*EventSubscription)
	c.unsubQueue = timerqueue.New(c.mqUnsubscribe, c.unsubscribeDelay)
	c.inCh = inCh

	for i := 0; i < c.workers; i++ {
		go c.startWorker(inCh)
	}

	resetSub, err := c.mq.Subscribe("system", func(subj string, payload []byte, responseHeaders map[string][]string, _ error) {
		ev := subj[7:]
		switch ev {
		case "reset":
			c.handleSystemReset(payload)
		case "tokenReset":
			c.handleSystemTokenReset(payload)
		}

	})
	if err != nil {
		c.Stop()
		return err
	}

	c.resetSub = resetSub
	c.started = true
	return nil
}

// Logf writes a formatted log message
func (c *Cache) Logf(format string, v ...interface{}) {
	c.logger.Log(fmt.Sprintf(format, v...))
}

// Errorf writes a formatted log message
func (c *Cache) Errorf(format string, v ...interface{}) {
	c.logger.Error(fmt.Sprintf(format, v...))
}

// Subscribe fetches a resource from the cache, and if it is
// not cached, starts subscribing to the resource and sends a get request
func (c *Cache) Subscribe(sub Subscriber, t *Throttle, requestHeaders map[string][]string) {
	eventSub, err := c.getSubscription(sub.ResourceName(), true)
	if err != nil {
		sub.Loaded(nil, nil, err)
		return
	}

	eventSub.addSubscriber(sub, t, requestHeaders)
}

// Access sends an access request
func (c *Cache) Access(sub Subscriber, token interface{}, callback func(access *Access)) {
	rname := sub.ResourceName()
	payload := codec.CreateRequest(nil, sub, sub.ResourceQuery(), token)
	subj := "access." + rname
	c.sendRequest(rname, subj, payload, func(data []byte, err error) {
		if err != nil {
			callback(&Access{Error: reserr.RESError(err)})
			return
		}

		access, rerr := codec.DecodeAccessResponse(data)
		callback(&Access{AccessResult: access, Error: rerr})
	}, nil)
}

// Call sends a method call request
func (c *Cache) Call(req codec.Requester, rname, query, action string, token, params interface{}, callback func(result json.RawMessage, rid string, err error)) {
	payload := codec.CreateRequest(params, req, query, token)
	subj := "call." + rname + "." + action
	c.sendRequest(rname, subj, payload, func(data []byte, err error) {
		if err != nil {
			callback(nil, "", err)
			return
		}

		// [DEPRECATED:deprecatedNewCallRequest]
		if action == "new" {
			result, rid, err := codec.DecodeCallResponse(data)
			if err == nil && rid == "" {
				rid, err = codec.TryDecodeLegacyNewResult(result)
				if err != nil || rid != "" {
					c.deprecated(rname, deprecatedNewCallRequest)
					callback(nil, rid, err)
					return
				}
			}
			callback(result, rid, err)
			return
		}

		callback(codec.DecodeCallResponse(data))
	}, nil)
}

// Auth sends an auth method call
func (c *Cache) Auth(req codec.AuthRequester, rname, query, action string, token, params interface{}, callback func(result json.RawMessage, rid string, err error)) {
	payload := codec.CreateAuthRequest(params, req, query, token)
	subj := "auth." + rname + "." + action
	c.sendRequest(rname, subj, payload, func(data []byte, err error) {
		if err != nil {
			callback(nil, "", err)
			return
		}

		callback(codec.DecodeCallResponse(data))
	}, nil)
}

// CustomAuth sends an auth method call to a custom subject
func (c *Cache) CustomAuth(req codec.AuthRequester, subj, query string, token, params interface{}, callback func(result json.RawMessage, rid string, err error)) {
	payload := codec.CreateAuthRequest(params, req, query, token)
	c.mq.SendRequest(subj, payload, func(_ string, data []byte, responseHeaders map[string][]string, err error) {
		if err != nil {
			callback(nil, "", err)
			return
		}

		callback(codec.DecodeCallResponse(data))
	}, nil)
}

func (c *Cache) sendRequest(rname, subj string, payload []byte, cb func(data []byte, err error), requestHeaders map[string][]string) {
	eventSub, _ := c.getSubscription(rname, false)
	c.mq.SendRequest(subj, payload, func(_ string, data []byte, responseHeaders map[string][]string, err error) {
		eventSub.Enqueue(func() {
			cb(data, err)
			eventSub.removeCount(1)
		})
	}, requestHeaders)
}

// AddConn adds a connection listening to events such as system token reset
// event.
func (c *Cache) AddConn(conn Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.conns[conn.CID()] = conn
	metrics.WSStablishedConnections.Set(float64(len(c.conns)))
}

// RemoveConn removes a connection listening to events.
func (c *Cache) RemoveConn(conn Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.conns, conn.CID())
	metrics.WSStablishedConnections.Set(float64(len(c.conns)))
}

// getSubscription returns the existing eventSubscription after adding its count, or creates a new
// subscription with count of 1. If the subscribe flag is true, a mq subscription is also made.
func (c *Cache) getSubscription(name string, subscribe bool) (*EventSubscription, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	eventSub, ok := c.eventSubs[name]
	if !ok {
		eventSub = &EventSubscription{
			ResourceName: name,
			cache:        c,
			count:        1,
		}
		metrics.SubcriptionsCount.WithLabelValues(metrics.SanitizedString(name)).Inc()

		c.eventSubs[name] = eventSub
	} else {
		eventSub.addCount()
	}

	if subscribe && eventSub.mqSub == nil {
		mqSub, err := c.mq.Subscribe("event."+name, func(subj string, payload []byte, responseHeaders map[string][]string, _ error) {
			eventSub.enqueueEvent(subj, payload)
		})
		if err != nil {
			return nil, err
		}

		eventSub.mqSub = mqSub
	}

	return eventSub, nil
}

// Stop closes the worker channel, stops all the workers,  and clears
// the unsubscribe queue
func (c *Cache) Stop() {
	if !c.started {
		return
	}
	close(c.inCh)
	c.unsubQueue.Clear()
	c.resetSub = nil
	c.started = false
}

func (c *Cache) startWorker(ch chan *EventSubscription) {
	for eventSub := range ch {
		eventSub.processQueue()
	}
}

func (c *Cache) mqUnsubscribe(v interface{}) {
	eventSub := v.(*EventSubscription)
	c.mu.Lock()
	defer c.mu.Unlock()

	if !eventSub.mqUnsubscribe() {
		return
	}

	delete(c.eventSubs, eventSub.ResourceName)
}

func (c *Cache) handleSystemReset(payload []byte) {
	r, err := codec.DecodeSystemReset(payload)
	if err != nil {
		c.Errorf("Error decoding system reset: %s", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var t *Throttle
	if c.resetThrottle > 0 {
		t = NewThrottle(c.resetThrottle)
	}

	c.forEachMatch(r.Resources, func(e *EventSubscription) {
		e.handleResetResource(t)
	})
	c.forEachMatch(r.Access, func(e *EventSubscription) {
		e.handleResetAccess(t)
	})
}

func (c *Cache) forEachMatch(p []string, cb func(e *EventSubscription)) {
	if len(p) == 0 {
		return
	}

	patterns := make([]ResourcePattern, 0, len(p))

	for _, r := range p {
		pattern := ParseResourcePattern(r)
		if pattern.IsValid() {
			patterns = append(patterns, pattern)
		}
	}

	for resourceName, eventSub := range c.eventSubs {
		for _, p := range patterns {
			if p.Match(resourceName) {
				cb(eventSub)
			}
		}
	}
}

func (c *Cache) handleSystemTokenReset(payload []byte) {
	r, err := codec.DecodeSystemTokenReset(payload)
	if err != nil {
		c.Errorf("Error decoding system token reset: %s", err)
		return
	}

	if r.Subject == "" {
		c.Errorf("Missing subject in system token reset")
		return
	}

	// Quick exit if no token IDs are available.
	if len(r.TIDs) == 0 {
		return
	}

	m := make(map[string]bool, len(r.TIDs))
	for _, tid := range r.TIDs {
		m[tid] = true
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Iterate over all currenct connects and let them each validate against any
	// existing token ID (tid).
	for _, sub := range c.conns {
		sub.TokenReset(m, r.Subject)
	}
}
