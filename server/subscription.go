package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/resgateio/resgate/server/codec"
	"github.com/resgateio/resgate/server/rescache"
	"github.com/resgateio/resgate/server/reserr"
	"github.com/resgateio/resgate/server/rpc"
)

type subscriptionState byte

// ConnSubscriber represents a client connection making the subscription
type ConnSubscriber interface {
	Logf(format string, v ...interface{})
	Debugf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	CID() string
	Token() json.RawMessage
	Subscribe(rid string, direct bool, throttle *rescache.Throttle, headers map[string][]string) (*Subscription, error)
	Unsubscribe(sub *Subscription, direct bool, count int, tryDelete bool)
	Access(sub *Subscription, callback func(*rescache.Access))
	Send(data []byte)
	Enqueue(f func()) bool
	ExpandCID(string) string
	Disconnect(reason string)
	ProtocolVersion() int
}

// Subscription represents a resource subscription made by a client connection
type Subscription struct {
	rid           string
	resourceName  string
	resourceQuery string

	c     ConnSubscriber
	state subscriptionState

	readyCallbacks []*readyCallback

	resourceSub     *rescache.ResourceSubscription
	typ             rescache.ResourceType
	model           *rescache.Model
	collection      *rescache.Collection
	version         uint
	refs            map[string]*reference
	err             error
	queueFlag       uint8
	eventQueue      []*rescache.ResourceEvent
	access          *rescache.Access
	accessCallbacks []func(*rescache.Access)
	flags           uint8
	throttle        *rescache.Throttle
	traceparent     string

	// Protected by conn
	direct   int // Number of direct subscriptions
	indirect int // Number of indirect subscriptions
}

type reference struct {
	sub   *Subscription
	count int
}

type readyCallback struct {
	refMap  map[string]bool
	cb      func()
	loading int
}

const (
	stateDisposed subscriptionState = iota
	stateLoading
	stateLoaded
	stateReady
	stateToSend
	stateSent
	stateDeleted
)

const (
	queueReasonLoading uint8 = 1 << iota
	queueReasonReaccess
)

const (
	flagAccessCalled uint8 = 1 << iota
	flagReaccess
)

var (
	errSubscriptionLimitExceeded = &reserr.Error{Code: "system.subscriptionLimitExceeded", Message: "Subscription limit exceeded"}
	errDisposedSubscription      = &reserr.Error{Code: "system.disposedSubscription", Message: "Resource subscription is disposed"}
)

// NewSubscription creates a new Subscription
func NewSubscription(c ConnSubscriber, rid string, throttle *rescache.Throttle) *Subscription {
	name, query := parseRID(c.ExpandCID(rid))

	sub := &Subscription{
		rid:           rid,
		resourceName:  name,
		resourceQuery: query,
		c:             c,
		state:         stateLoading,
		queueFlag:     queueReasonLoading,
		throttle:      throttle,
	}

	return sub
}

// RID returns the subscription's resource ID
func (s *Subscription) RID() string {
	return s.rid
}

// ResourceName returns the resource name part of the subscription's resource ID
func (s *Subscription) ResourceName() string {
	return s.resourceName
}

// ResourceQuery returns the query part of the subscription's resource ID
func (s *Subscription) ResourceQuery() string {
	return s.resourceQuery
}

// Token returns the access token held by the subscription's client connection
func (s *Subscription) Token() json.RawMessage {
	return s.c.Token()
}

// ResourceType returns the resource type of the subscribed resource
func (s *Subscription) ResourceType() rescache.ResourceType {
	return s.typ
}

// CID returns the unique connection ID of the client connection
func (s *Subscription) CID() string {
	return s.c.CID()
}

// IsReady returns true if the subscription and all of its dependencies are loaded.
func (s *Subscription) IsReady() bool {
	return s.state >= stateReady
}

// IsSent returns true if the subscribed resource has been sent to the client.
func (s *Subscription) IsSent() bool {
	return s.state == stateSent
}

// Error returns any error that occurred when loading the subscribed resource.
func (s *Subscription) Error() error {
	if s.state == stateDisposed {
		return errDisposedSubscription
	}
	return s.err
}

// ModelValues returns the subscriptions model values.
// Panics if the subscription is not a loaded model.
func (s *Subscription) ModelValues() map[string]codec.Value {
	return s.model.Values
}

// CollectionValues returns the subscriptions collection values.
// Panics if the subscription is not a loaded collection.
func (s *Subscription) CollectionValues() []codec.Value {
	return s.collection.Values
}

// Ref returns the referenced subscription, or nil if subscription has no such reference.
func (s *Subscription) Ref(rid string) *Subscription {
	r := s.refs[rid]
	if r != nil {
		return r.sub
	}
	return nil
}

// Loaded is called by rescache when the subscribed resource has been loaded.
// If the resource was successfully loaded, err will be nil. If an error occurred
// when loading the resource, resourceSub will be nil, and err will be the error.
func (s *Subscription) Loaded(resourceSub *rescache.ResourceSubscription, responseHeaders map[string][]string, err error) {
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

		// We check if we received a traceparent header in the response. If we did, we set it on the subscription.
		if responseHeaders != nil {
			val, ok := responseHeaders["traceparent"]
			if ok && len(val) > 0 && s.traceparent == "" {
				s.traceparent = val[0]
			}
		}

		s.resourceSub = resourceSub
		s.typ = resourceSub.GetResourceType()
		s.state = stateLoaded

		s.setResource()
		if s.err != nil {
			s.doneLoading()
			return
		}

		rcbs := s.readyCallbacks
		s.readyCallbacks = nil
		// Collect references for any waiting ready callbacks
		for _, rcb := range rcbs {
			s.collectRefs(rcb)
		}
	}) {
		if err == nil {
			resourceSub.Unsubscribe(s)
		}
	}
}

// setResource is called after Loaded is called
func (s *Subscription) setResource() {
	switch s.typ {
	case rescache.TypeCollection:
		s.setCollection()
	case rescache.TypeModel:
		s.setModel()
	default:
		err := fmt.Errorf("subscription %s: unknown resource type", s.rid)
		s.c.Errorf("Error loading %s", err)
		s.err = err
	}
}

// OnReady gets a callback that should be called once the subscribed resource
// and all its referenced resources recursively, has been loaded from the rescache.
// If the resource is already ready, the callback will directly be called.
func (s *Subscription) OnReady(cb func()) {
	if s.IsReady() {
		cb()
		return
	}

	s.onLoaded(&readyCallback{
		refMap: make(map[string]bool),
		cb:     cb,
	})
}

// onLoaded gets a readyCallback that should be called once the subscribed resource
// has been loaded from the rescache. If the resource is already loaded,
// the callback will directly be queued onto the connections worker goroutine.
func (s *Subscription) onLoaded(rcb *readyCallback) {
	// Add itself to refMap
	rcb.refMap[s.rid] = true
	rcb.loading++

	if s.state >= stateLoaded {
		s.collectRefs(rcb)
	} else {
		s.readyCallbacks = append(s.readyCallbacks, rcb)
	}
}

// GetRPCResources returns a rpc.Resources object.
// It will lock the subscription and queue any events until ReleaseRPCResources is called.
func (s *Subscription) GetRPCResources() *rpc.Resources {
	r := &rpc.Resources{}
	if s.c.ProtocolVersion() < versionSoftResourceReferenceAndDataValue {
		s.populateResourcesLegacy(r)
	} else {
		s.populateResources(r)
	}
	return r
}

// ReleaseRPCResources will unlock all resources locked by GetRPCResource,
// unqueue any events, and mark the subscription as sent.
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
	s.unqueueEvents(queueReasonLoading)
}

func (s *Subscription) queueEvents(reason uint8) {
	s.queueFlag |= reason
}

func (s *Subscription) unqueueEvents(reason uint8) {
	s.queueFlag &= ^reason
	if s.queueFlag != 0 {
		return
	}

	// Start with reaccess calls
	if s.flags&flagReaccess != 0 {
		s.handleReaccess(nil)
		if s.queueFlag != 0 {
			return
		}
	}

	eq := s.eventQueue
	s.eventQueue = nil

	for i, event := range eq {
		s.processEvent(event)
		// Did one of the events activate queueing again?
		if s.queueFlag != 0 {
			s.eventQueue = append(eq[i+1:], s.eventQueue...)
			return
		}
	}
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
	case rescache.TypeCollection:
		// Create Collections map if needed
		if r.Collections == nil {
			r.Collections = make(map[string]interface{})
		}
		r.Collections[s.rid] = s.collection

	case rescache.TypeModel:
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

// populateResourcesLegacy is the same as populateResources, but uses legacy
// encodings of resources.
func (s *Subscription) populateResourcesLegacy(r *rpc.Resources) {
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
	case rescache.TypeCollection:
		// Create Collections map if needed
		if r.Collections == nil {
			r.Collections = make(map[string]interface{})
		}
		r.Collections[s.rid] = (*rescache.Legacy120Collection)(s.collection)

	case rescache.TypeModel:
		// Create Models map if needed
		if r.Models == nil {
			r.Models = make(map[string]interface{})
		}
		r.Models[s.rid] = (*rescache.Legacy120Model)(s.model)
	}

	s.state = stateToSend

	for _, sc := range s.refs {
		sc.sub.populateResourcesLegacy(r)
	}
}

// setModel subscribes to all resource references in the model.
func (s *Subscription) setModel() {
	s.queueEvents(queueReasonLoading)
	m, version := s.resourceSub.GetModel()
	for _, v := range m.Values {
		if !s.subscribeRef(v) {
			return
		}
	}
	s.model = m
	s.version = version
}

// setCollection subscribes to all resource references in the collection.
func (s *Subscription) setCollection() {
	s.queueEvents(queueReasonLoading)
	c, version := s.resourceSub.GetCollection()
	for _, v := range c.Values {
		if !s.subscribeRef(v) {
			return
		}
	}
	s.collection = c
	s.version = version
}

// subscribeRef subscribes to any resource reference value
// and adds it to s.refs.
// If an error is encountered, all subscriptions in s.refs will
// be unsubscribed, s.err set, s.doneLoading called, and false returned.
// If v is not a resource reference, nothing will happen.
func (s *Subscription) subscribeRef(v codec.Value) bool {
	if v.Type != codec.ValueTypeReference {
		return true
	}

	if _, err := s.addReference(v.RID); err != nil {
		// In case of subscribe error,
		// we unsubscribe to all and exit with error
		s.c.Debugf("Failed to subscribe to %s. Aborting subscribeRef", v.RID)
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
// and call the callback once completed.
func (s *Subscription) collectRefs(rcb *readyCallback) {
	for rid, ref := range s.refs {
		// Don't wait for already ready references
		// or references already included in the refMap
		if ref.sub.IsReady() || rcb.refMap[rid] {
			continue
		}

		ref.sub.onLoaded(rcb)
	}

	rcb.loading--
	s.testReady(rcb)
}

func (s *Subscription) testReady(rcb *readyCallback) {
	if rcb.loading == 0 {
		rcb.cb()
	}
}

func containsString(path []string, rid string) bool {
	for _, p := range path {
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

	requestHeaders := make(map[string][]string)

	// If the subscription has a traceparent, add it to the subsiquent requests as a request header.
	if s.traceparent != "" {
		requestHeaders["traceparent"] = []string{s.traceparent}
	}

	if ref == nil {
		sub, err := s.c.Subscribe(rid, false, s.throttle, requestHeaders)

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

// removeReference removes a reference from the subscription due to an
// event such as collection remove or model change.
func (s *Subscription) removeReference(rid string) {
	ref := s.refs[rid]
	ref.count--
	if ref.count == 0 {
		s.c.Unsubscribe(ref.sub, false, 1, true)
		delete(s.refs, rid)
	}
}

// Event passes an event to the subscription to be processed.
func (s *Subscription) Event(event *rescache.ResourceEvent) {
	s.c.Enqueue(func() {
		if event.Event == "reaccess" {
			s.reaccess(nil)
			return
		}

		// Discard any event prior to resourceSubscription being loaded or disposed
		if s.resourceSub == nil {
			return
		}

		if s.queueFlag != 0 {
			s.eventQueue = append(s.eventQueue, event)
			return
		}

		s.processEvent(event)
	})
}

func (s *Subscription) processEvent(event *rescache.ResourceEvent) {
	// Discard events targeting a different internal version
	if s.version != event.Version {
		return
	}

	// Bump the version if it is an update
	if event.Update {
		s.version++
	}

	switch s.resourceSub.GetResourceType() {
	case rescache.TypeCollection:
		s.processCollectionEvent(event)
	case rescache.TypeModel:
		s.processModelEvent(event)
	default:
		s.c.Errorf("Subscription %s: Unknown resource type: %d", s.rid, s.resourceSub.GetResourceType())
	}
}

func (s *Subscription) processCollectionEvent(event *rescache.ResourceEvent) {
	switch event.Event {
	case "add":
		v := event.Value
		idx := event.Idx

		switch v.Type {
		case codec.ValueTypeReference:
			rid := v.RID
			sub, err := s.addReference(rid)
			if err != nil {
				s.c.Errorf("Subscription %s: Error subscribing to resource %s: %s", s.rid, v.RID, err)
				// TODO send error value
				return
			}

			// Quick exit if added resource is already sent to client
			if sub.IsSent() {
				s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.AddEvent{Idx: idx, Value: v.RawMessage}))
				return
			}

			// Start queueing again
			s.queueEvents(queueReasonLoading)

			sub.OnReady(func() {
				// Assert client is still subscribing
				// If not we just unsubscribe
				if s.state == stateDisposed {
					return
				}

				r := sub.GetRPCResources()
				s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.AddEvent{Idx: idx, Value: v.RawMessage, Resources: r}))
				sub.ReleaseRPCResources()

				s.unqueueEvents(queueReasonLoading)
			})
		case codec.ValueTypeData:
			fallthrough
		case codec.ValueTypeSoftReference:
			if s.c.ProtocolVersion() < versionSoftResourceReferenceAndDataValue {
				s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.AddEvent{Idx: idx, Value: rescache.Legacy120Value(v)}))
				break
			}
			fallthrough
		case codec.ValueTypePrimitive:
			s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.AddEvent{Idx: idx, Value: v.RawMessage}))
		}

	case "remove":
		// Remove and unsubscribe to model
		v := event.Value

		if v.Type == codec.ValueTypeReference {
			s.removeReference(v.RID)
		}
		s.c.Send(rpc.NewEvent(s.rid, event.Event, event.Payload))

	case "delete":
		s.state = stateDeleted
		s.c.Send(rpc.NewEvent(s.rid, event.Event, event.Payload))
		s.unsubscribeDirect(reserr.ErrDeleted)
	default:
		s.c.Send(rpc.NewEvent(s.rid, event.Event, event.Payload))
	}
}

func (s *Subscription) processModelEvent(event *rescache.ResourceEvent) {
	switch event.Event {
	case "change":
		ch := event.Changed
		old := event.OldValues
		var subs []*Subscription

		for _, v := range ch {
			if v.Type == codec.ValueTypeReference {
				sub, err := s.addReference(v.RID)
				if err != nil {
					s.c.Errorf("Subscription %s: Error subscribing to resource %s: %s", s.rid, v.RID, err)
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
			if ov, ok := old[k]; ok && ov.Type == codec.ValueTypeReference {
				s.removeReference(ov.RID)
			}
		}

		// Quick exit if there are no new unsent subscriptions
		if subs == nil {
			// Legacy behavior
			if s.c.ProtocolVersion() < versionSoftResourceReferenceAndDataValue {
				s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.ChangeEvent{Values: rescache.Legacy120ValueMap(event.Changed)}))
			} else {
				s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.ChangeEvent{Values: event.Changed}))
			}
			return
		}

		// Start queueing again
		s.queueEvents(queueReasonLoading)
		count := len(subs)
		for _, sub := range subs {
			sub.OnReady(func() {
				// Assert client is not disposed
				if s.state == stateDisposed {
					return
				}

				count--
				if count > 0 {
					return
				}

				r := &rpc.Resources{}

				// Legacy behavior
				if s.c.ProtocolVersion() < versionSoftResourceReferenceAndDataValue {
					for _, sub := range subs {
						sub.populateResourcesLegacy(r)
					}
					s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.ChangeEvent{Values: rescache.Legacy120ValueMap(event.Changed), Resources: r}))
				} else {
					for _, sub := range subs {
						sub.populateResources(r)
					}
					s.c.Send(rpc.NewEvent(s.rid, event.Event, rpc.ChangeEvent{Values: event.Changed, Resources: r}))
				}
				for _, sub := range subs {
					sub.ReleaseRPCResources()
				}

				s.unqueueEvents(queueReasonLoading)
			})
		}
	case "delete":
		s.state = stateDeleted
		s.c.Send(rpc.NewEvent(s.rid, event.Event, event.Payload))
		s.unsubscribeDirect(reserr.ErrDeleted)
	default:
		s.c.Send(rpc.NewEvent(s.rid, event.Event, event.Payload))
	}
}

func (s *Subscription) handleReaccess(t *rescache.Throttle) {
	s.access = nil
	s.flags &= ^flagReaccess

	if s.direct == 0 {
		return
	}

	// If we already have an access instance set, use that one to test without queueing
	if s.access != nil {
		s.validateAccess(s.access)
		return
	}

	s.queueEvents(queueReasonReaccess)
	s.loadAccess(func(a *rescache.Access) {
		s.validateAccess(a)
		s.unqueueEvents(queueReasonReaccess)
	}, t)
}

// validateAccess checks if subscription has get access, or else unsubscribes.
func (s *Subscription) validateAccess(a *rescache.Access) {
	err := a.CanGet()
	if err != nil {
		s.unsubscribeDirect(reserr.RESError(err))
	}
}

// unsubscribeDirect removes any direct subscription of the resource and sends
// an unsubscribe event if any direct subscriptions existed.
func (s *Subscription) unsubscribeDirect(reason *reserr.Error) {
	if s.direct > 0 {
		s.c.Unsubscribe(s, true, s.direct, true)
		s.c.Send(rpc.NewEvent(s.rid, "unsubscribe", rpc.UnsubscribeEvent{Reason: reason}))
	}
}

// Dispose removes any resourceSubscription and sets
// the subscription state to stateDisposed
func (s *Subscription) Dispose() {
	if s.state == stateDisposed {
		return
	}

	state := s.state
	s.state = stateDisposed
	s.readyCallbacks = nil
	s.eventQueue = nil
	s.throttle = nil

	if s.resourceSub != nil {
		s.unsubscribeRefs()
		if state != stateDeleted {
			s.resourceSub.Unsubscribe(s)
		}
		s.resourceSub = nil
	}
}

// doneLoading will decrease all loading counters for
// each readyCallback, and test if they reach 0.
func (s *Subscription) doneLoading() {
	s.state = stateReady
	rcbs := s.readyCallbacks
	s.readyCallbacks = nil
	s.throttle = nil

	for _, rcb := range rcbs {
		rcb.loading--
		s.testReady(rcb)
	}
}

// Reaccess adds a reaccess event to the eventQueue,
// triggering a new access request to be sent to the service.
func (s *Subscription) Reaccess(t *rescache.Throttle) {
	s.c.Enqueue(func() { s.reaccess(t) })
}

func (s *Subscription) reaccess(t *rescache.Throttle) {
	if s.state == stateDisposed {
		return
	}

	if s.queueFlag != 0 {
		s.flags |= flagReaccess
		return
	}

	s.handleReaccess(t)
}

func parseRID(rid string) (name string, query string) {
	i := strings.IndexByte(rid, '?')
	if i == -1 {
		return rid, ""
	}

	return rid[:i], rid[i+1:]
}

func (s *Subscription) loadAccess(cb func(*rescache.Access), t *rescache.Throttle) {
	if s.access != nil {
		cb(s.access)
		return
	}

	s.accessCallbacks = append(s.accessCallbacks, cb)

	if s.flags&flagAccessCalled != 0 {
		return
	}

	s.flags |= flagAccessCalled

	if t != nil {
		t.Add(func() {
			s.c.Access(s, func(access *rescache.Access) {
				s.c.Enqueue(func() {
					if s.state == stateDisposed {
						return
					}

					cbs := s.accessCallbacks
					s.flags &= ^flagAccessCalled
					// Only store in case of an actual result or system.accessDenied error
					if access.Error == nil || access.Error.Code == reserr.CodeAccessDenied {
						s.access = access
					}
					s.accessCallbacks = nil

					for _, cb := range cbs {
						cb(access)
					}
				})
				t.Done()
			})
		})
	} else {
		s.c.Access(s, func(access *rescache.Access) {
			s.c.Enqueue(func() {
				if s.state == stateDisposed {
					return
				}

				cbs := s.accessCallbacks
				s.flags &= ^flagAccessCalled
				// Only store in case of an actual result or system.accessDenied error
				if access.Error == nil || access.Error.Code == reserr.CodeAccessDenied {
					s.access = access
				}
				s.accessCallbacks = nil

				for _, cb := range cbs {
					cb(access)
				}
			})
		})
	}
}

// CanGet checks asynchronously if the client connection has access to get (read)
// the resource. If access is denied, the callback will be called with an error
// describing the reason. If access is granted, the callback will be called with
// err being nil.
func (s *Subscription) CanGet(cb func(err error)) {
	s.loadAccess(func(a *rescache.Access) {
		cb(a.CanGet())
	}, nil)
}

// CanCall checks asynchronously if the client connection has access to call
// the actionn. If access is denied, the callback will be called with an error
// describing the reason. If access is granted, the callback will be called with
// err being nil.
func (s *Subscription) CanCall(action string, cb func(err error)) {
	s.loadAccess(func(a *rescache.Access) {
		cb(a.CanCall(action))
	}, nil)
}
