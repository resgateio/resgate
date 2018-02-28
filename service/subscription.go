package service

import (
	"encoding/json"
	"strings"

	"github.com/jirenius/resgate/httpApi"
	"github.com/jirenius/resgate/reserr"
	"github.com/jirenius/resgate/resourceCache"
	"github.com/jirenius/resgate/rpc"
)

type subscriptionState byte

type ConnSubscriber interface {
	Log(v ...interface{})
	Logf(format string, v ...interface{})
	CID() string
	Token() json.RawMessage
	Subscribe(resourceID string, direct bool) (*Subscription, error)
	SubscribeAll(resourceIDs []string) ([]*Subscription, error)
	Unsubscribe(sub *Subscription, direct bool, count int)
	UnsubscribeAll(subs []*Subscription)
	Access(sub *Subscription, callback func(*resourceCache.Access))
	Send(data []byte)
	Enqueue(f func()) bool
	ExpandCID(string) string
}

type Subscription struct {
	resourceID    string
	resourceName  string
	resourceQuery string

	c ConnSubscriber

	state           subscriptionState
	loadedCallbacks []func(*Subscription)
	resourceSub     *resourceCache.ResourceSubscription
	subs            []*Subscription
	err             error
	modelCount      int
	isQueueing      bool
	isCollection    bool
	eventQueue      []*resourceCache.ResourceEvent
	access          *resourceCache.Access
	accessCallbacks []func()
	accessCalled    bool

	// Protected by conn
	direct   int
	indirect int
}

const (
	stateCreated subscriptionState = iota
	stateCollecting
	stateReady
	stateSent
	stateDisposed
)

const (
	subscriptionCountLimit = 256
)

var (
	errSubscriptionLimitExceeded = &reserr.Error{Code: "system.subscriptionLimitExceeded", Message: "Subscription limit exceeded"}
	errDisposedSubscription      = &reserr.Error{Code: "system.disposedSubscription", Message: "Resource subscription is disposed"}
)

// NewSubscription creates a new Subscription
func NewSubscription(c ConnSubscriber, resourceID string) *Subscription {
	name, query := parseResourceID(c.ExpandCID(resourceID))

	sub := &Subscription{
		resourceID:      resourceID,
		resourceName:    name,
		resourceQuery:   query,
		c:               c,
		loadedCallbacks: make([]func(*Subscription), 0, 2),
	}

	return sub
}

func (s *Subscription) ResourceID() string {
	return s.resourceID
}

func (s *Subscription) ResourceName() string {
	return s.resourceName
}

func (s *Subscription) ResourceQuery() string {
	return s.resourceQuery
}

func (s *Subscription) Token() json.RawMessage {
	return s.c.Token()
}

func (s *Subscription) Loaded(resourceSub *resourceCache.ResourceSubscription, err error) {
	if !s.c.Enqueue(func() {
		if err != nil {
			s.err = err
			s.doneLoading()
			return
		}

		if s.state == stateDisposed {
			resourceSub.Unsubscribe(s)
			return
		}

		s.resourceSub = resourceSub
		switch resourceSub.GetResourceType() {
		case resourceCache.Collection:
			s.isCollection = true
			s.state = stateCollecting
			s.setCollection()
			for _, sub := range s.subs {
				sub.OnLoaded(s.collectionModelLoaded)
			}
		case resourceCache.Model:
			s.doneLoading()
		default:
			s.state = stateReady
			s.c.Logf("Subscription %s: Unknown resource type", s.resourceID)
		}
	}) {
		if err == nil {
			resourceSub.Unsubscribe(s)
		}
	}
}

func (s *Subscription) IsCollection() bool {
	return s.isCollection
}

// GetRpcResource returns a rpc.Resource object.
// It will lock the subscription and queue any events until ReleaseRPCResource is called.
func (s *Subscription) GetRPCResource() *rpc.Resource {
	if s.state == stateDisposed {
		return &rpc.Resource{ResourceID: s.resourceID, Error: errDisposedSubscription}
	}

	if s.state == stateSent {
		return &rpc.Resource{ResourceID: s.resourceID}
	}

	if s.err != nil {
		return &rpc.Resource{ResourceID: s.resourceID, Error: s.err}
	}

	resourceSub := s.resourceSub
	if resourceSub == nil {
		s.c.Logf("Subscription %s: About to crash. State: %d", s.resourceID, s.state)
	}
	switch resourceSub.GetResourceType() {
	case resourceCache.Collection:
		arr := make([]*rpc.Resource, len(s.subs))
		for i, sub := range s.subs {
			arr[i] = sub.GetRPCResource()
		}
		return &rpc.Resource{ResourceID: s.resourceID, Data: arr}

	case resourceCache.Model:
		resource := &rpc.Resource{ResourceID: s.resourceID, Data: resourceSub.GetModel()}
		s.queueEvents()
		resourceSub.Release()
		return resource
	}

	// Dummy
	return nil
}

// GetHTTPResource returns a httpApi.Resource object.
// It will lock the subscription and queue any events until ReleaseRPCResource is called.
func (s *Subscription) GetHTTPResource(apiPath string) *httpApi.Resource {
	if s.state == stateDisposed {
		return &httpApi.Resource{APIPath: apiPath, ResourceID: s.resourceID, Error: errDisposedSubscription}
	}

	if s.err != nil {
		return &httpApi.Resource{APIPath: apiPath, ResourceID: s.resourceID, Error: s.err}
	}

	resourceSub := s.resourceSub
	if resourceSub == nil {
		s.c.Logf("Subscription %s: About to crash. State: %d", s.resourceID, s.state)
	}
	switch resourceSub.GetResourceType() {
	case resourceCache.Collection:
		arr := make([]*httpApi.Resource, len(s.subs))
		for i, sub := range s.subs {
			arr[i] = sub.GetHTTPResource(apiPath)
		}
		return &httpApi.Resource{APIPath: apiPath, ResourceID: s.resourceID, Data: arr}

	case resourceCache.Model:
		resource := &httpApi.Resource{APIPath: apiPath, ResourceID: s.resourceID, Data: resourceSub.GetModel()}
		s.queueEvents()
		resourceSub.Release()
		return resource
	}

	// Dummy
	return nil
}

// ReleaseRPCResource will unlock all resources locked by GetRPCResource
// and will mark the subscription as sent.
func (s *Subscription) ReleaseRPCResource() {
	if s.state == stateDisposed ||
		s.state == stateSent ||
		s.err != nil {
		return
	}

	s.state = stateSent

	resourceSub := s.resourceSub
	switch resourceSub.GetResourceType() {
	case resourceCache.Collection:
		for _, sub := range s.subs {
			sub.ReleaseRPCResource()
		}
		s.unqueueEvents()

	case resourceCache.Model:
		s.unqueueEvents()
	}
}

func (s *Subscription) queueEvents() {
	s.isQueueing = true
}

func (s *Subscription) unqueueEvents() {
	s.isQueueing = false

	for i, event := range s.eventQueue {
		s.processEvent(event)
		// Did one of the events activate queueing again?
		if s.isQueueing {
			s.eventQueue = s.eventQueue[i+1:]
			return
		}
	}

	s.eventQueue = nil
}

func (s *Subscription) OnLoaded(cb func(*Subscription)) {
	if s.loadedCallbacks != nil {
		s.loadedCallbacks = append(s.loadedCallbacks, cb)
		return
	}

	s.c.Enqueue(func() {
		cb(s)
	})
}

func (s *Subscription) CID() string {
	return s.c.CID()
}

// setCollection subscribes to all collection models.
// Subscription lock must be held when calling, and will be released on return
func (s *Subscription) setCollection() {
	col := s.resourceSub.GetCollection()
	s.queueEvents()
	s.resourceSub.Release()

	s.modelCount = len(col)

	if s.modelCount == 0 {
		s.doneLoading()
		return
	}

	subs, err := s.c.SubscribeAll(col)
	if err != nil {
		s.err = err
		s.doneLoading()
		return
	}
	s.subs = subs
}

func (s *Subscription) collectionModelLoaded(sub *Subscription) {
	// Assert client is still subscribing
	// If not we just unsubscribe
	if s.state == stateDisposed {
		return
	}

	// Assert we did not receive a collection
	if sub.IsCollection() {
		// TODO
	}

	s.modelCount--

	if s.modelCount > 0 {
		return
	}

	s.doneLoading()
}

func (s *Subscription) Event(event *resourceCache.ResourceEvent) {
	s.c.Enqueue(func() {
		// Discard any event prior to resourceSubscription being loaded or disposed
		if s.resourceSub == nil {
			return
		}

		if s.isQueueing {
			s.eventQueue = append(s.eventQueue, event)
			return
		}

		s.processEvent(event)
	})
}

func (s *Subscription) processEvent(event *resourceCache.ResourceEvent) {
	if event.Event == "reaccess" {
		s.handleReaccess()
		return
	}

	switch s.resourceSub.GetResourceType() {
	case resourceCache.Collection:
		s.processCollectionEvent(event)
	case resourceCache.Model:
		if s.state != stateSent {
			return
		}

		s.c.Send(rpc.NewEvent(s.resourceID, event.Event, event.Data))
	default:
		s.c.Logf("Subscription %s: Unknown resource type: %d", s.resourceID, s.resourceSub.GetResourceType())
	}
}

func (s *Subscription) processCollectionEvent(event *resourceCache.ResourceEvent) {
	switch event.Event {
	case "add":
		idx := event.AddData.Idx
		sub, err := s.c.Subscribe(event.AddData.ResourceID, false)
		if err != nil {
			s.c.Logf("Subscription %s: Error subscribing to resource %s: %s", s.resourceID, event.AddData.ResourceID, err)
			return
		}

		// Start queueing again
		s.queueEvents()

		// Insert into subs slice
		s.subs = append(s.subs, nil)
		copy(s.subs[idx+1:], s.subs[idx:])
		s.subs[idx] = sub

		sub.OnLoaded(func(sub *Subscription) {
			if sub.IsCollection() {
				// TODO error handling
			}

			// Assert client is still subscribing
			// If not we just unsubscribe
			if s.state == stateDisposed {
				return
			}

			r := sub.GetRPCResource()
			s.c.Send(rpc.NewEvent(s.resourceID, event.Event, rpc.AddEventResource{Resource: r, Idx: idx}))
			sub.ReleaseRPCResource()

			s.unqueueEvents()
		})

	case "remove":
		// Remove and unsubscribe to model
		idx := event.RemoveData.Idx
		subs := s.subs
		if idx < 0 || idx >= len(subs) {
			s.c.Logf("Subscription %s: Remove event index out of range: %d", s.resourceID, idx)
		}
		sub := subs[idx]
		s.subs = subs[:idx+copy(subs[idx:], subs[idx+1:])]

		s.c.Unsubscribe(sub, false, 1)

		s.c.Send(rpc.NewEvent(s.resourceID, event.Event, event.Data))
	}
}

func (s *Subscription) handleReaccess() {
	s.access = nil
	if s.direct == 0 {
		return
	}
	// Start queueing again
	s.queueEvents()

	s.loadAccess(func() {
		err := s.access.CanGet()
		if err != nil {
			s.c.Unsubscribe(s, true, s.direct)
			s.c.Send(rpc.NewEvent(s.resourceID, "unsubscribe", rpc.UnsubscribeEvent{Reason: reserr.RESError(err)}))
		}

		s.unqueueEvents()
	})
}

// unsubscribe removes any resourceSubscription
func (s *Subscription) unsubscribe() {
	if s.state == stateDisposed {
		return
	}

	s.state = stateDisposed
	s.loadedCallbacks = nil
	s.eventQueue = nil

	if s.resourceSub != nil {
		if s.subs != nil {
			s.c.UnsubscribeAll(s.subs)
		}
		s.resourceSub.Unsubscribe(s)
		s.resourceSub = nil
	}
}

// Dispose obtains a lock and calls unsubscribe
func (s *Subscription) Dispose() {
	s.unsubscribe()
}

// doneLoading calls all OnLoaded callbacks.
// Subscription lock must be held when calling doneLoading,
func (s *Subscription) doneLoading() {
	s.state = stateReady
	cbs := s.loadedCallbacks
	s.loadedCallbacks = nil

	for _, cb := range cbs {
		cb(s)
	}
}

func (s *Subscription) Reaccess() {
	// Discard any event prior to resourceSubscription being loaded or disposed
	if s.resourceSub == nil {
		s.queueEvents()
	}

	event := &resourceCache.ResourceEvent{Event: "reaccess"}

	if s.isQueueing {
		s.eventQueue = append(s.eventQueue, event)
		return
	}

	s.processEvent(event)
	return
}

func parseResourceID(resourceID string) (name string, query string) {
	i := strings.IndexByte(resourceID, '?')
	if i == -1 || i == len(resourceID)-1 {
		return resourceID, ""
	}

	return resourceID[:i], resourceID[i+1:]
}

func (s *Subscription) loadAccess(cb func()) {
	if s.access != nil {
		cb()
		return
	}

	s.accessCallbacks = append(s.accessCallbacks, cb)

	if s.accessCalled {
		return
	}

	s.accessCalled = true

	s.c.Access(s, func(access *resourceCache.Access) {
		s.c.Enqueue(func() {
			if s.state == stateDisposed {
				return
			}

			cbs := s.accessCallbacks
			s.accessCalled = false
			s.access = access
			s.accessCallbacks = nil

			for _, cb := range cbs {
				cb()
			}
		})
	})
}

func (s *Subscription) CanGet(cb func(err error)) {
	if s.indirect > 0 {
		cb(nil)
		return
	}

	s.loadAccess(func() {
		cb(s.access.CanGet())
	})
}

func (s *Subscription) CanCall(action string, cb func(err error)) {
	s.loadAccess(func() {
		cb(s.access.CanCall(action))
	})
}
