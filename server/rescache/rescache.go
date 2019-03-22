package rescache

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/jirenius/resgate/logger"
	"github.com/jirenius/resgate/server/codec"
	"github.com/jirenius/resgate/server/mq"
	"github.com/jirenius/resgate/server/reserr"
	"github.com/jirenius/timerqueue"
)

// Cache is an in memory resource cache.
type Cache struct {
	mq      mq.Client
	logger  logger.Logger
	workers int

	started    bool
	eventSubs  map[string]*EventSubscription
	inCh       chan *EventSubscription
	unsubQueue *timerqueue.Queue
	resetSub   mq.Unsubscriber
	mu         sync.Mutex
}

// Subscriber interface represents a subscription made on a client connection
type Subscriber interface {
	CID() string
	Loaded(resourceSub *ResourceSubscription, err error)
	Event(event *ResourceEvent)
	ResourceName() string
	ResourceQuery() string
	Reaccess()
}

// ResourceEvent represents an event on a resource
type ResourceEvent struct {
	Event     string
	Payload   json.RawMessage
	Idx       int
	Value     codec.Value
	Changed   map[string]codec.Value
	OldValues map[string]codec.Value
}

const unsubscribeDelay = time.Second * 5

// NewCache creates a new Cache instance
func NewCache(mq mq.Client, workers int, l logger.Logger) *Cache {
	return &Cache{
		mq:      mq,
		logger:  l,
		workers: workers,
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
	c.unsubQueue = timerqueue.New(c.mqUnsubscribe, unsubscribeDelay)
	c.inCh = inCh

	for i := 0; i < c.workers; i++ {
		go c.startWorker(inCh)
	}

	resetSub, err := c.mq.Subscribe("system", func(subj string, payload []byte, _ error) {
		ev := subj[7:]
		switch ev {
		case "reset":
			c.handleSystemReset(payload)
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
	if c.logger == nil {
		return
	}
	c.logger.Logf("[Cache] ", format, v...)
}

// Subscribe fetches a resource from the cache, and if it is
// not cached, starts subscribing to the resource and sends a get request
func (c *Cache) Subscribe(sub Subscriber) {
	eventSub, err := c.getSubscription(sub.ResourceName(), true)
	if err != nil {
		sub.Loaded(nil, err)
		return
	}

	eventSub.addSubscriber(sub)
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
	})
}

// Call sends a method call request
func (c *Cache) Call(req codec.Requester, rname, query, action string, token, params interface{}, callback func(result json.RawMessage, err error)) {
	payload := codec.CreateRequest(params, req, query, token)
	subj := "call." + rname + "." + action
	c.sendRequest(rname, subj, payload, func(data []byte, err error) {
		if err != nil {
			callback(nil, err)
			return
		}

		callback(codec.DecodeCallResponse(data))
	})
}

// CallNew sends a call request with the new method, expecting a response with an RID
func (c *Cache) CallNew(req codec.Requester, rname, query string, token, params interface{}, callback func(newRID string, err error)) {
	payload := codec.CreateRequest(params, req, query, token)
	subj := "call." + rname + ".new"
	c.sendRequest(rname, subj, payload, func(data []byte, err error) {
		if err != nil {
			callback("", err)
			return
		}

		callback(codec.DecodeNewResponse(data))
	})
}

// Auth sends an auth method call
func (c *Cache) Auth(req codec.AuthRequester, rname, query, action string, token, params interface{}, callback func(result json.RawMessage, err error)) {
	payload := codec.CreateAuthRequest(params, req, query, token)
	subj := "auth." + rname + "." + action
	c.sendRequest(rname, subj, payload, func(data []byte, err error) {
		if err != nil {
			callback(nil, err)
			return
		}

		callback(codec.DecodeCallResponse(data))
	})
}

func (c *Cache) sendRequest(rname, subj string, payload []byte, cb func(data []byte, err error)) {
	eventSub, _ := c.getSubscription(rname, false)
	c.mq.SendRequest(subj, payload, func(_ string, data []byte, err error) {
		eventSub.Enqueue(func() {
			cb(data, err)
			eventSub.removeCount(1)
		})
	})
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

		c.eventSubs[name] = eventSub
	} else {
		eventSub.addCount()
	}

	if subscribe && eventSub.mqSub == nil {
		mqSub, err := c.mq.Subscribe("event."+name, func(subj string, payload []byte, _ error) {
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
	c.mu.Lock()
	defer c.mu.Unlock()

	r, err := codec.DecodeSystemReset(payload)
	if err != nil {
		c.Logf("Error decoding system reset: %s", err)
		return
	}

	c.forEachMatch(r.Resources, func(e *EventSubscription) {
		e.handleResetResource()
	})

	c.forEachMatch(r.Access, func(e *EventSubscription) {
		e.handleResetAccess()
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
