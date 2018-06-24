package resourceCache

import (
	"sync"

	"github.com/jirenius/resgate/mq"
	"github.com/jirenius/resgate/mq/codec"
)

type responseType byte
type ResourceType byte

const (
	respEvent responseType = iota
	respGet
	respCall
	respCached
)

const (
	TypeCollection ResourceType = ResourceType(stateCollection)
	TypeModel      ResourceType = ResourceType(stateModel)
	TypeError      ResourceType = ResourceType(stateError)
)

type EventSubscription struct {
	// Immutable
	ResourceName string
	cache        *Cache

	// Protected by single goroutine
	mqSub   mq.Unsubscriber
	base    *ResourceSubscription
	queries map[string]*ResourceSubscription
	links   map[string]*ResourceSubscription

	// Mutex protected
	mu    sync.Mutex
	count int64
	queue []func()
	locks []func()
}

type response struct {
	rtype   responseType
	subject string
	payload []byte
	err     error
	sub     Subscriber
}

type queueEvent struct {
	subj    string
	payload []byte
}

func (e *EventSubscription) getResourceSubscription(q string) (rs *ResourceSubscription) {
	if q == "" {
		rs = e.base
		if rs == nil {
			rs = newResourceSubscription(e, "")
			e.base = rs
		}
	} else {
		if e.queries == nil {
			e.queries = make(map[string]*ResourceSubscription)
			rs = newResourceSubscription(e, q)
			e.queries[q] = rs
		} else {
			rs = e.queries[q]
			if rs == nil && e.links != nil {
				rs = e.links[q]
			}

			if rs == nil {
				rs = newResourceSubscription(e, q)
				e.queries[q] = rs
			}
		}
	}
	return
}

func (e *EventSubscription) addSubscriber(sub Subscriber) {
	e.Enqueue(func() {
		var rs *ResourceSubscription
		q := sub.ResourceQuery()
		rs = e.getResourceSubscription(q)

		if rs.state != stateError {
			rs.subs[sub] = struct{}{}
		}

		switch rs.state {
		// A subscription is made, but no request for the data.
		// A request is made and state progressed
		case stateSubscribed:
			// Progress state
			rs.state = stateRequested
			// Create request
			subj := "get." + e.ResourceName
			payload := codec.CreateGetRequest(q)
			e.cache.mq.SendRequest(subj, payload, func(_ string, data []byte, err error) {
				rs.enqueueGetResponse(data, err)
			})

		// If a request has already been sent
		// In that case the subscriber will be handled
		// on the response for that request
		case stateRequested:

		// An error occured during request
		case stateError:
			e.count--
			e.mu.Unlock()
			defer e.mu.Lock()
			sub.Loaded(nil, rs.err)

		// stateModel or stateCollection
		default:
			e.mu.Unlock()
			defer e.mu.Lock()
			sub.Loaded(rs, nil)
		}
	})
}

func (e *EventSubscription) Enqueue(f func()) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.enqueue(f)
}

func (e *EventSubscription) enqueue(f func()) {
	count := len(e.queue)
	e.queue = append(e.queue, f)

	// If the queue is empty, there are no worker currently
	// assigned to the event subscription, so we pass it to one.
	// This only applies if no locks are active
	if e.locks == nil && count == 0 {
		e.cache.inCh <- e
	}
}

func (e *EventSubscription) enqueueUnlock(f func()) {
	e.mu.Lock()
	defer e.mu.Unlock()

	count := len(e.locks)
	e.locks = append(e.locks, f)
	if count == 0 {
		e.cache.inCh <- e
	}
}

// processQueue is called by the cacheWorker
func (e *EventSubscription) processQueue() {
	e.mu.Lock()
	defer e.mu.Unlock()
	var f func()
	idx := 0

	if e.locks != nil {
		for len(e.locks) > idx {
			f = e.locks[idx]
			idx++
			f()
		}

		e.locks = e.locks[idx:]

		if cap(e.locks) > 0 {
			return
		}
		e.locks = nil
		if len(e.queue) == 0 {
			return
		}

		idx = 0
	}

	for len(e.queue) > idx {
		f = e.queue[idx]
		idx++
		f()
		if e.locks != nil {
			copy(e.queue, e.queue[idx:])
			e.queue = e.queue[:len(e.queue)-idx]
			return
		}
	}
	e.queue = e.queue[0:0]
}

func (e *EventSubscription) lockEvents(locks int) {
	if locks > 0 {
		e.locks = make([]func(), 0, locks)
	}
}

// addCount increments the subscription count.
// If the previous count was 0, indicating it has previously been added to the unsubQueue,
// the method will remove itself from the queue.
func (e *EventSubscription) addCount() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.count == 0 {
		e.cache.unsubQueue.Remove(e)
	}
	e.count++
}

func (e *EventSubscription) removeCount(n int64) {
	e.count -= n
	if e.count == 0 {
		e.cache.unsubQueue.Add(e)
	}
}

func (e *EventSubscription) enqueueEvent(subj string, payload []byte) {
	e.Enqueue(func() {
		idx := len(e.ResourceName) + 7 // Length of "event." + "."
		if idx >= len(subj) {
			e.cache.Logf("Error processing event %s: malformed event subject", subj)
			return
		}

		event := subj[idx:]
		switch event {
		case "query":
			e.handleQueryEvent(subj, payload)
		default:

			if e.base == nil {
				return
			}

			ev, err := codec.DecodeEvent(payload)
			if err != nil {
				e.cache.Logf("Error processing event %s: malformed payload %s", subj, payload)
				return
			}

			e.base.handleEvent(&ResourceEvent{Event: event, Data: ev})
		}
	})
}

func (e *EventSubscription) handleQueryEvent(subj string, payload []byte) {
	l := len(e.queries)
	if l == 0 {
		return
	}

	qe, err := codec.DecodeQueryEvent(payload)
	if err != nil {
		e.cache.Logf("Error processing event %s: malformed payload %s", subj, payload)
		return
	}

	if qe.Subject == "" {
		e.cache.Logf("Missing subject in event %s: %s", subj, payload)
		return
	}

	// We lock events from being handled until all event queries has been handled first
	e.lockEvents(l)

	for q, rs := range e.queries {
		payload := codec.CreateEventQueryRequest(q)
		rs := rs
		e.cache.mq.SendRequest(qe.Subject, payload, func(subj string, data []byte, err error) {
			e.enqueueUnlock(func() {
				if err != nil {
					return
				}

				events, err := codec.DecodeEventQueryResponse(data)
				if err != nil {
					e.cache.Logf("Error processing query event: malformed payload %s", data)
					return
				}

				for _, ev := range events {
					rs.handleEvent(&ResourceEvent{Event: ev.Event, Data: ev.Data})
				}
			})
		})
	}
}

// mqUnsubscribe unsubscribes to the MQ.
// Returns true on success, otherwise false.
// It may fail if the subscription count is not zero, or an error
// is returned from the MQ.
func (e *EventSubscription) mqUnsubscribe() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Have we just recieved a subscription?
	// In that case we abort
	if e.count > 0 {
		return false
	}

	// Clear the response queue
	e.queue = nil

	// Unsubscribe from message queue
	if e.mqSub != nil {
		err := e.mqSub.Unsubscribe()
		if err != nil {
			e.cache.Logf("Error unsubscribing to %s: %s", e.ResourceName, err)
			return false
		}
	}
	return true
}

func (e *EventSubscription) handleResetResource() {
	e.Enqueue(func() {
		if e.base != nil {
			e.base.handleResetResource()
		}

		for _, rs := range e.queries {
			rs.handleResetResource()
		}
	})
}

func (e *EventSubscription) handleResetAccess() {
	e.Enqueue(func() {
		if e.base != nil {
			e.base.handleResetAccess()
		}

		for _, rs := range e.queries {
			rs.handleResetAccess()
		}
	})
}
