package service

import (
	"encoding/json"
	"strings"

	"github.com/jirenius/resgate/httpApi"
	"github.com/jirenius/resgate/mq/codec"
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
	Subscribe(rid string, direct bool) (*Subscription, error)
	Unsubscribe(sub *Subscription, direct bool, count int)
	Access(sub *Subscription, callback func(*resourceCache.Access))
	Send(data []byte)
	Enqueue(f func()) bool
	ExpandCID(string) string
}

type Subscription struct {
	rid           string
	resourceName  string
	resourceQuery string

	c ConnSubscriber

	state           subscriptionState
	loadedCallbacks []func(*Subscription)
	resourceSub     *resourceCache.ResourceSubscription
	data            interface{}
	refs            map[string]*reference
	err             error
	resourceCount   int
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

type reference struct {
	sub   *Subscription
	count int
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
func NewSubscription(c ConnSubscriber, rid string) *Subscription {
	name, query := parseRID(c.ExpandCID(rid))

	sub := &Subscription{
		rid:             rid,
		resourceName:    name,
		resourceQuery:   query,
		c:               c,
		loadedCallbacks: make([]func(*Subscription), 0, 2),
	}

	return sub
}

func (s *Subscription) RID() string {
	return s.rid
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
		case resourceCache.Model:
			s.doneLoading()
		default:
			s.state = stateReady
			if debug {
				s.c.Logf("Subscription %s: Unknown resource type", s.rid)
			}
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

func (s *Subscription) IsSent() bool {
	return s.state == stateSent
}

func (s *Subscription) Error() error {
	if s.state == stateDisposed {
		return errDisposedSubscription
	}

	return s.err
}

// GetRpcResource returns a rpc.Resources object.
// It will lock the subscription and queue any events until ReleaseRPCResources is called.
func (s *Subscription) GetRPCResources() *rpc.Resources {
	r := &rpc.Resources{}
	s.populateResources(r)
	return r
}

// populateResources iterates recursively down the subscription tree
// and populates the rpc.Resources object with all non-sent resources
// referenced by the subscription, as well as the subscription's own data.
func (s *Subscription) populateResources(r *rpc.Resources) {
	// Quick exit if resource is already sent
	if s.state == stateSent {
		return
	}

	// Check for errors
	err := s.Error()
	if err != nil {
		// Create Errors map if needed
		if r.Errors == nil {
			r.Errors = make(map[string]*reserr.Error)
		}
		r.Errors[s.rid] = reserr.RESError(err)
		return
	}

	resourceSub := s.resourceSub

	switch resourceSub.GetResourceType() {
	case resourceCache.Collection:
		// Create Collections map if needed
		if r.Collections == nil {
			r.Collections = make(map[string]interface{})
		}
		r.Collections[s.rid] = s.data
		s.data = nil

		for _, sc := range s.refs {
			sc.sub.populateResources(r)
		}

	case resourceCache.Model:
		// Create Models map if needed
		if r.Models == nil {
			r.Models = make(map[string]interface{})
		}
		r.Models[s.rid] = resourceSub.GetModel()
		s.queueEvents()
		resourceSub.Release()
	}
}

// GetHTTPResource returns an empty interface of either a httpApi.Model or a httpApi.Collection object.
// It will lock the subscription and queue any events until ReleaseRPCResources is called.
func (s *Subscription) GetHTTPResource(apiPath string) *httpApi.Resource {
	if s.state == stateDisposed {
		return &httpApi.Resource{APIPath: apiPath, RID: s.rid, Error: errDisposedSubscription}
	}

	if s.err != nil {
		return &httpApi.Resource{APIPath: apiPath, RID: s.rid, Error: s.err}
	}

	resourceSub := s.resourceSub
	var resource *httpApi.Resource

	switch resourceSub.GetResourceType() {
	case resourceCache.Collection:
		col := s.data.([]codec.Value)
		arr := make([]interface{}, len(col))
		for i, v := range col {
			if v.Type == codec.ValueTypeResource {
				sc := s.refs[v.RID]
				arr[i] = sc.sub.GetHTTPResource(apiPath)
			} else {
				arr[i] = v.RawMessage
			}
		}
		resource = &httpApi.Resource{APIPath: apiPath, RID: s.rid, Collection: arr}

	case resourceCache.Model:
		resource = &httpApi.Resource{APIPath: apiPath, RID: s.rid, Model: resourceSub.GetModel()}
		s.queueEvents()
		resourceSub.Release()

	}

	return resource
}

// ReleaseRPCResources will unlock all resources locked by GetRPCResource
// and will mark the subscription as sent.
func (s *Subscription) ReleaseRPCResources() {
	if s.state == stateDisposed ||
		s.state == stateSent ||
		s.err != nil {
		return
	}

	s.state = stateSent

	resourceSub := s.resourceSub
	switch resourceSub.GetResourceType() {
	case resourceCache.Collection:
		for _, sc := range s.refs {
			sc.sub.ReleaseRPCResources()
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

	err := s.subscribeAllRef(col)
	if err != nil {
		s.err = err
		s.doneLoading()
		return
	}

	s.data = col
	s.resourceCount = len(s.refs)

	if s.resourceCount == 0 {
		s.doneLoading()
	} else {
		for _, ref := range s.refs {
			ref.sub.OnLoaded(s.subresourceLoaded)
		}
	}
}

func (s *Subscription) subscribeAllRef(collection []codec.Value) error {
	for _, v := range collection {
		if v.Type != codec.ValueTypeResource {
			continue
		}

		if _, err := s.addReference(v.RID); err != nil {
			// In case of subscribe error,
			// we unsubscribe to all and exit with error
			if debug {
				s.c.Logf("Failed to subscribe to %s. Aborting subscribeAllRef", v.RID)
			}
			for _, ref := range s.refs {
				s.c.Unsubscribe(ref.sub, false, 1)
			}
			s.refs = nil
			return err
		}
	}

	return nil
}

func (s *Subscription) unsubscribeAllRef() {
	for _, ref := range s.refs {
		s.c.Unsubscribe(ref.sub, false, 1)
	}
	s.refs = nil
}

func (s *Subscription) addReference(rid string) (*Subscription, error) {
	refs := s.refs
	var ref *reference

	if refs == nil {
		refs = make(map[string]*reference)
		s.refs = refs
	} else {
		ref = refs[rid]
	}

	if ref == nil {
		sub, err := s.c.Subscribe(rid, false)

		if err != nil {
			return nil, err
		}

		ref = &reference{sub: sub, count: 1}
		refs[rid] = ref
	} else {
		ref.count++
	}

	return ref.sub, nil
}

func (s *Subscription) removeReference(rid string) {
	ref := s.refs[rid]
	ref.count--
	if ref.count == 0 {
		s.c.Unsubscribe(ref.sub, false, 1)
		delete(s.refs, rid)
	}
}

func (s *Subscription) subresourceLoaded(sub *Subscription) {
	// Assert client is still subscribing
	// If not we just unsubscribe
	if s.state == stateDisposed {
		return
	}

	// Assert we did not receive a collection
	if sub.IsCollection() {
		// TODO
	}

	s.resourceCount--

	if s.resourceCount == 0 {
		s.doneLoading()
	}
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

		s.c.Send(rpc.NewEvent(s.rid, event.Event, event.Data))
	default:
		if debug {
			s.c.Logf("Subscription %s: Unknown resource type: %d", s.rid, s.resourceSub.GetResourceType())
		}
	}
}

func (s *Subscription) processCollectionEvent(event *resourceCache.ResourceEvent) {
	switch event.Event {
	case "add":
		idx := event.AddData.Idx

		switch event.AddData.Value.Type {
		case codec.ValueTypeResource:
			rid := event.AddData.Value.RID
			sub, err := s.addReference(rid)
			if err != nil {
				if debug {
					s.c.Logf("Subscription %s: Error subscribing to resource %s: %s", s.rid, event.AddData.Value.RID, err)
				}
				// TODO send error value
				return
			}

			// Quick exit if added resource is already sent to client
			if sub.IsSent() {
				s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.AddEventResource{Idx: idx, Value: event.AddData.Value.RawMessage}))
				return
			}

			// Start queueing again
			s.queueEvents()

			sub.OnLoaded(func(sub *Subscription) {
				if sub.IsCollection() {
					// TODO error handling
				}

				// Assert client is still subscribing
				// If not we just unsubscribe
				if s.state == stateDisposed {
					return
				}

				r := sub.GetRPCResources()

				s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.AddEventResource{Idx: idx, Value: event.AddData.Value.RawMessage, Resources: r}))
				sub.ReleaseRPCResources()

				s.unqueueEvents()
			})
		case codec.ValueTypePrimitive:
			s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.AddEventResource{Idx: idx, Value: event.AddData.Value.RawMessage}))
		}

	case "remove":
		// Remove and unsubscribe to model
		v := event.RemoveData.Value

		if v.Type == codec.ValueTypeResource {
			s.removeReference(v.RID)
		}

		s.c.Send(rpc.NewEvent(s.rid, event.Event, event.Data))
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
			s.c.Send(rpc.NewEvent(s.rid, "unsubscribe", rpc.UnsubscribeEvent{Reason: reserr.RESError(err)}))
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
		s.unsubscribeAllRef()
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

func parseRID(rid string) (name string, query string) {
	i := strings.IndexByte(rid, '?')
	if i == -1 || i == len(rid)-1 {
		return rid, ""
	}

	return rid[:i], rid[i+1:]
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
