package rescache

import (
	"sync"

	"github.com/resgateio/resgate/metrics"
	"github.com/resgateio/resgate/server/codec"
	"github.com/resgateio/resgate/server/mq"
	"github.com/resgateio/resgate/server/reserr"
)

// ResourceType is an enum representing a resource type
type ResourceType byte

// Resource types
const (
	TypeCollection ResourceType = ResourceType(stateCollection)
	TypeModel      ResourceType = ResourceType(stateModel)
	TypeError      ResourceType = ResourceType(stateError)
)

// EventSubscription represents a subscription for events on a specific resource
type EventSubscription struct {
	// Immutable
	ResourceName string
	cache        *Cache

	// Protected by cache mutex
	mqSub mq.Unsubscriber
	count int64

	// Protected by single goroutine
	base    *ResourceSubscription
	queries map[string]*ResourceSubscription
	links   map[string]*ResourceSubscription

	// Mutex protected
	mu    sync.Mutex
	queue []func()
	locks []func()
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

func (e *EventSubscription) addSubscriber(sub Subscriber, t *Throttle, requestHeaders map[string][]string) {
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
			// Request directly if we don't throttle, or else add to throttle
			if t == nil {
				e.cache.mq.SendRequest(subj, payload, func(_ string, data []byte, responseHeaders map[string][]string, err error) {
					rs.enqueueGetResponse(data, responseHeaders, err)
				}, requestHeaders)
			} else {
				t.Add(func() {
					e.cache.mq.SendRequest(subj, payload, func(_ string, data []byte, responseHeaders map[string][]string, err error) {
						rs.enqueueGetResponse(data, responseHeaders, err)
						t.Done()
					}, requestHeaders)
				})
			}

		// If a request has already been sent
		// In that case the subscriber will be handled
		// on the response for that request
		case stateRequested:
			return

		// An error occurred during request
		case stateError:
			e.count--
			metrics.SubcriptionsCount.WithLabelValues(metrics.SanitizedString(e.ResourceName)).Dec()
			e.mu.Unlock()
			defer e.mu.Lock()
			sub.Loaded(nil, nil, rs.err)

		// stateModel or stateCollection
		default:
			e.mu.Unlock()
			defer e.mu.Lock()
			sub.Loaded(rs, nil, nil)
		}
	})
}

// Enqueue passes the callback function to be executed by one of the worker goroutines.
// If a worker is already executing a callback on the EventSubscription, the callback
// will be queued on the EventSubscription, and executed in order.
func (e *EventSubscription) Enqueue(f func()) {
	e.mu.Lock()
	count := len(e.queue)
	locks := e.locks
	e.queue = append(e.queue, f)
	e.mu.Unlock()

	// If the queue is empty, there are no worker currently
	// assigned to the event subscription, so we pass it to one.
	// This only applies if no locks are active
	if locks == nil && count == 0 {
		e.cache.inCh <- e
	}
}

// enqueueUnlock uses one of the locked callbacks. If the number of
// locks reaches zero, callbacks passed to Enqueue will no longer
// be queued.
func (e *EventSubscription) enqueueUnlock(f func()) {
	e.mu.Lock()
	count := len(e.locks)
	e.locks = append(e.locks, f)
	e.mu.Unlock()

	if count == 0 {
		e.cache.inCh <- e
	}
}

// lockEvents will queue any callback that is passed to Enqueue until
// enqueueUnlock has been called the same number of time as locks.
func (e *EventSubscription) lockEvents(locks int) {
	if locks > 0 {
		e.locks = make([]func(), 0, locks)
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
	metrics.SubcriptionsCount.WithLabelValues(metrics.SanitizedString(e.ResourceName)).Inc()
}

// removeCount decreases the subscription count, and puts the event subscription
// in the unsubscribe queue if count reaches zero.
func (e *EventSubscription) removeCount(n int64) {
	e.count -= n
	if e.count == 0 && n != 0 {
		e.cache.unsubQueue.Add(e)
	}
	metrics.SubcriptionsCount.WithLabelValues(metrics.SanitizedString(e.ResourceName)).Sub(float64(n))
}

func (e *EventSubscription) enqueueEvent(subj string, payload []byte) {
	e.Enqueue(func() {
		idx := len(e.ResourceName) + 7 // Length of "event." + "."
		if idx >= len(subj) {
			e.cache.Errorf("Error processing event %s: malformed event subject", subj)
			return
		}

		event := subj[idx:]
		switch event {
		case "query":
			e.handleQueryEvent(subj, payload)
		default:

			// Validate we have a base resource,
			// and that it is not a link to a query resource.
			if e.base == nil || e.base.query != "" {
				return
			}

			ev, err := codec.DecodeEvent(payload)
			if err != nil {
				e.cache.Errorf("Error processing event %s: malformed payload %s", subj, payload)
				return
			}

			e.base.handleEvent(&ResourceEvent{Event: event, Payload: ev})
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
		e.cache.Errorf("Error processing event %s: malformed payload %s", subj, payload)
		return
	}

	if qe.Subject == "" {
		e.cache.Errorf("Missing subject in event %s: %s", subj, payload)
		return
	}

	// We lock events from being handled until all event queries has been handled first
	e.lockEvents(l)

	for q, rs := range e.queries {
		// Do not include queries still being requested
		if rs.state <= stateRequested {
			go e.enqueueUnlock(func() {})
			continue
		}
		payload := codec.CreateEventQueryRequest(q)
		rs := rs
		e.cache.mq.SendRequest(qe.Subject, payload, func(subj string, data []byte, requestHeaders map[string][]string, err error) {
			e.enqueueUnlock(func() {
				if err != nil {
					return
				}

				result, err := codec.DecodeEventQueryResponse(data)
				if err != nil {
					// In case of a system.notFound error,
					// a delete event is generated. Otherwise we
					// just log the error.
					if reserr.IsError(err, reserr.CodeNotFound) {
						rs.handleEvent(&ResourceEvent{Event: "delete"})
					} else {
						e.cache.Errorf("Error processing query event for %s?%s: %s", e.ResourceName, rs.query, err)
					}
					return
				}

				switch {
				// Handle array of events
				case result.Events != nil:
					for _, ev := range result.Events {
						rs.handleEvent(&ResourceEvent{Event: ev.Event, Payload: ev.Data})
					}
				// Handle model response
				case result.Model != nil:
					if rs.state != stateModel {
						e.cache.Errorf("Error processing query event for %s?%s: non-model payload on model %s", e.ResourceName, rs.query, data)
						return
					}
					rs.processResetModel(result.Model)
				// Handle collection response
				case result.Collection != nil:
					if rs.state != stateCollection {
						e.cache.Errorf("Error processing query event for %s?%s: non-model payload on model %s", e.ResourceName, rs.query, data)
						return
					}
					rs.processResetCollection(result.Collection)
				}
			})
		}, nil)
	}
}

// mqUnsubscribe unsubscribes to the MQ.
// Returns true on success, otherwise false.
// It may fail if the subscription count is not zero, or an error
// is returned from the MQ.
func (e *EventSubscription) mqUnsubscribe() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Have we just received a subscription?
	// In that case we abort
	if e.count > 0 {
		return false
	}

	// Clear the response queue
	e.queue = nil

	// Unsubscribe from messaging system
	if e.mqSub != nil {
		err := e.mqSub.Unsubscribe()
		if err != nil {
			e.cache.Errorf("Error unsubscribing to %s: %s", e.ResourceName, err)
			return false
		}
	}
	return true
}

func (e *EventSubscription) handleResetResource(t *Throttle) {
	e.Enqueue(func() {
		if e.base != nil && e.base.query == "" {
			e.base.handleResetResource(t)
		}

		for _, rs := range e.queries {
			rs.handleResetResource(t)
		}
	})
}

func (e *EventSubscription) handleResetAccess(t *Throttle) {
	e.Enqueue(func() {
		if e.base != nil && e.base.query == "" {
			e.base.handleResetAccess(t)
		}

		for _, rs := range e.queries {
			rs.handleResetAccess(t)
		}
	})
}
