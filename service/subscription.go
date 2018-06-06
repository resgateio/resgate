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
	Subscribe(rid string, direct bool, path []string) (*Subscription, error)
	Unsubscribe(sub *Subscription, direct bool, count int, tryDelete bool)
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
	path            []string
	resourceSub     *resourceCache.ResourceSubscription
	typ             resourceCache.ResourceType
	model           *resourceCache.Model
	collection      *resourceCache.Collection
	refs            map[string]*reference
	err             error
	resourceCount   int
	isQueueing      bool
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
	stateToSend
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
func NewSubscription(c ConnSubscriber, rid string, path []string) *Subscription {
	name, query := parseRID(c.ExpandCID(rid))

	// Clone path and add RID
	l := len(path)
	p := make([]string, l+1)
	copy(p[:l], path)
	p[l] = rid

	sub := &Subscription{
		rid:             rid,
		resourceName:    name,
		resourceQuery:   query,
		c:               c,
		loadedCallbacks: make([]func(*Subscription), 0, 2),
		path:            p,
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

func (s *Subscription) ResourceType() resourceCache.ResourceType {
	return s.typ
}

func (s *Subscription) CID() string {
	return s.c.CID()
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
		s.typ = resourceSub.GetResourceType()
		switch s.typ {
		case resourceCache.TypeCollection:
			s.setCollection()
		case resourceCache.TypeModel:
			s.setModel()
		default:
			s.state = stateReady
			s.c.Logf("Subscription %s: Unknown resource type", s.rid)
		}
	}) {
		if err == nil {
			resourceSub.Unsubscribe(s)
		}
	}
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

func (s *Subscription) OnLoaded(cb func(*Subscription)) {
	if s.loadedCallbacks != nil {
		s.loadedCallbacks = append(s.loadedCallbacks, cb)
		return
	}

	s.c.Enqueue(func() {
		cb(s)
	})
}

// GetRpcResource returns a rpc.Resources object.
// It will lock the subscription and queue any events until ReleaseRPCResources is called.
func (s *Subscription) GetRPCResources() *rpc.Resources {
	r := &rpc.Resources{}
	s.populateResources(r)
	return r
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

	var resource *httpApi.Resource

	switch s.typ {
	case resourceCache.TypeCollection:
		vals := s.collection.Values
		c := make([]interface{}, len(vals))
		for i, v := range vals {
			if v.Type == codec.ValueTypeResource {
				sc := s.refs[v.RID]
				c[i] = sc.sub.GetHTTPResource(apiPath)
			} else {
				c[i] = v.RawMessage
			}
		}
		resource = &httpApi.Resource{APIPath: apiPath, RID: s.rid, Collection: c}

	case resourceCache.TypeModel:
		vals := s.model.Values
		m := make(map[string]interface{}, len(vals))
		for k, v := range vals {
			if v.Type == codec.ValueTypeResource {
				sc := s.refs[v.RID]
				m[k] = sc.sub.GetHTTPResource(apiPath)
			} else {
				m[k] = v.RawMessage
			}
		}
		resource = &httpApi.Resource{APIPath: apiPath, RID: s.rid, Model: m}
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
	for _, sc := range s.refs {
		sc.sub.ReleaseRPCResources()
	}
	s.unqueueEvents()
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

// populateResources iterates recursively down the subscription tree
// and populates the rpc.Resources object with all non-sent resources
// referenced by the subscription, as well as the subscription's own data.
func (s *Subscription) populateResources(r *rpc.Resources) {
	// Quick exit if resource is already sent
	if s.state == stateSent || s.state == stateToSend {
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

	switch s.typ {
	case resourceCache.TypeCollection:
		// Create Collections map if needed
		if r.Collections == nil {
			r.Collections = make(map[string]interface{})
		}
		r.Collections[s.rid] = s.collection

	case resourceCache.TypeModel:
		// Create Models map if needed
		if r.Models == nil {
			r.Models = make(map[string]interface{})
		}
		r.Models[s.rid] = s.model
	}

	s.state = stateToSend

	for _, sc := range s.refs {
		sc.sub.populateResources(r)
	}
}

// setModel subscribes to all resource references in the model.
// Subscription lock must be held when calling, and will be released on return
func (s *Subscription) setModel() {
	m := s.resourceSub.GetModel()
	s.queueEvents()
	s.resourceSub.Release()
	for _, v := range m.Values {
		if !s.subscribeRef(v) {
			return
		}
	}
	s.model = m
	s.collectRefs()
}

// setCollection subscribes to all resource references in the collection.
// Subscription lock must be held when calling, and will be released on return
func (s *Subscription) setCollection() {
	c := s.resourceSub.GetCollection()
	s.queueEvents()
	s.resourceSub.Release()
	for _, v := range c.Values {
		if !s.subscribeRef(v) {
			return
		}
	}
	s.collection = c
	s.collectRefs()
}

// subscribeRef subscribes to any resource reference value
// and adds it to s.refs.
// If an error is encountered, all subscriptions in s.refs will
// be unsubscribed, s.err set, s.doneLoading called, and false returned.
// If v is not a resource reference, nothing will happen.
func (s *Subscription) subscribeRef(v codec.Value) bool {
	if v.Type != codec.ValueTypeResource {
		return true
	}

	if _, err := s.addReference(v.RID); err != nil {
		// In case of subscribe error,
		// we unsubscribe to all and exit with error
		if debug {
			s.c.Logf("Failed to subscribe to %s. Aborting subscribeRef", v.RID)
		}
		for _, ref := range s.refs {
			s.c.Unsubscribe(ref.sub, false, 1, true)
		}
		s.refs = nil
		s.err = err
		s.doneLoading()
		return false
	}

	return true
}

// collectRefs will wait for all references to be loaded
// and call doneLoading() once completed.
func (s *Subscription) collectRefs() {
	s.resourceCount = len(s.refs)

	s.state = stateCollecting
	for rid, ref := range s.refs {
		// Do not wait for loading if the
		// resource is part of a cyclic path
		if s.pathContains(rid) {
			s.resourceCount--
		} else {
			ref.sub.OnLoaded(s.refLoaded)
		}
	}

	if s.resourceCount == 0 {
		s.doneLoading()
	}
}

func (s *Subscription) pathContains(rid string) bool {
	for _, p := range s.path {
		if p == rid {
			return true
		}
	}
	return false
}

func (s *Subscription) unsubscribeRefs() {
	for _, ref := range s.refs {
		s.c.Unsubscribe(ref.sub, false, 1, false)
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
		sub, err := s.c.Subscribe(rid, false, s.path)

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
		s.c.Unsubscribe(ref.sub, false, 1, true)
		delete(s.refs, rid)
	}
}

func (s *Subscription) refLoaded(sub *Subscription) {
	// Assert client is still subscribing
	// If not we just unsubscribe
	if s.state == stateDisposed {
		return
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
	case resourceCache.TypeCollection:
		s.processCollectionEvent(event)
	case resourceCache.TypeModel:
		s.processModelEvent(event)
	default:
		if debug {
			s.c.Logf("Subscription %s: Unknown resource type: %d", s.rid, s.resourceSub.GetResourceType())
		}
	}
}

func (s *Subscription) processCollectionEvent(event *resourceCache.ResourceEvent) {
	switch event.Event {
	case "add":
		v := event.Value
		idx := event.Idx

		switch v.Type {
		case codec.ValueTypeResource:
			rid := v.RID
			sub, err := s.addReference(rid)
			if err != nil {
				if debug {
					s.c.Logf("Subscription %s: Error subscribing to resource %s: %s", s.rid, v.RID, err)
				}
				// TODO send error value
				return
			}

			// Quick exit if added resource is already sent to client
			if sub.IsSent() {
				s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.AddEvent{Idx: idx, Value: v.RawMessage}))
				return
			}

			// Start queueing again
			s.queueEvents()

			sub.OnLoaded(func(sub *Subscription) {
				// Assert client is still subscribing
				// If not we just unsubscribe
				if s.state == stateDisposed {
					return
				}

				r := sub.GetRPCResources()
				s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.AddEvent{Idx: idx, Value: v.RawMessage, Resources: r}))
				sub.ReleaseRPCResources()

				s.unqueueEvents()
			})
		case codec.ValueTypePrimitive:
			s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.AddEvent{Idx: idx, Value: v.RawMessage}))
		}

	case "remove":
		// Remove and unsubscribe to model
		v := event.Value

		if v.Type == codec.ValueTypeResource {
			s.removeReference(v.RID)
		}
		s.c.Send(rpc.NewEvent(s.rid, event.Event, event.Data))

	default:
		s.c.Send(rpc.NewEvent(s.rid, event.Event, event.Data))
	}
}

func (s *Subscription) processModelEvent(event *resourceCache.ResourceEvent) {
	switch event.Event {
	case "change":
		ch := event.Changed
		old := event.OldValues
		var subs []*Subscription

		for _, v := range ch {
			if v.Type == codec.ValueTypeResource {
				sub, err := s.addReference(v.RID)
				if err != nil {
					if debug {
						s.c.Logf("Subscription %s: Error subscribing to resource %s: %s", s.rid, v.RID, err)
					}
					// TODO handle error properly
					return
				}
				if !sub.IsSent() {
					if subs == nil {
						subs = make([]*Subscription, 0, len(ch))
					}
					subs = append(subs, sub)
				}
			}
		}

		// Check for removing changed references after adding references to avoid unsubscribing to
		// a resource that is going to be subscribed again because it has moved between properties.
		for k := range ch {
			if ov, ok := old[k]; ok && ov.Type == codec.ValueTypeResource {
				s.removeReference(ov.RID)
			}
		}

		// Quick exit if there are no new unsent subscriptions
		if subs == nil {
			s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.ChangeEvent{Values: event.Data}))
			return
		}

		// Start queueing again
		s.queueEvents()
		count := len(subs)
		for _, sub := range subs {
			sub.OnLoaded(func(sub *Subscription) {
				// Assert client is not disposed
				if s.state == stateDisposed {
					return
				}

				count--
				if count > 0 {
					return
				}

				r := &rpc.Resources{}
				for _, sub := range subs {
					sub.populateResources(r)
				}
				s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.ChangeEvent{Values: event.Data, Resources: r}))
				for _, sub := range subs {
					sub.ReleaseRPCResources()
				}

				s.unqueueEvents()
			})
		}

	default:
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
			s.c.Unsubscribe(s, true, s.direct, true)
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
		s.unsubscribeRefs()
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
	s.path = nil

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
