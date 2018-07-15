package resourceCache

import (
	"encoding/json"
	"errors"

	"github.com/jirenius/resgate/mq/codec"
)

type subscriptionState byte

const (
	stateSubscribed subscriptionState = iota
	stateError
	stateRequested
	stateCollection
	stateModel
)

var errQueryResourceOnNonQueryRequest = errors.New("Query resource on non-query request")

type Model struct {
	Values map[string]codec.Value
	data   []byte
}

func (m *Model) MarshalJSON() ([]byte, error) {
	if m.data == nil {
		data, err := json.Marshal(m.Values)
		if err != nil {
			return nil, err
		}
		m.data = data
	}
	return m.data, nil
}

type Collection struct {
	Values []codec.Value
	data   []byte
}

func (c *Collection) MarshalJSON() ([]byte, error) {
	if c.data == nil {
		data, err := json.Marshal(c.Values)
		if err != nil {
			return nil, err
		}
		c.data = data
	}
	return c.data, nil
}

type ResourceSubscription struct {
	e         *EventSubscription
	query     string
	state     subscriptionState
	subs      map[Subscriber]struct{}
	resetting bool
	links     []string
	// Three types of values stored
	model      *Model
	collection *Collection
	err        error
}

func newResourceSubscription(e *EventSubscription, query string) *ResourceSubscription {
	return &ResourceSubscription{
		e:     e,
		query: query,
		subs:  make(map[Subscriber]struct{}),
	}
}

func (rs *ResourceSubscription) GetResourceSubscription() *ResourceSubscription {
	return rs
}

func (rs *ResourceSubscription) GetResourceType() ResourceType {
	rs.e.mu.Lock()
	defer rs.e.mu.Unlock()
	return ResourceType(rs.state)
}

func (rs *ResourceSubscription) GetError() error {
	rs.e.mu.Lock()
	defer rs.e.mu.Unlock()
	return rs.err
}

// GetCollection will lock the EventSubscription for any changes
// and return the collection string slice.
// The lock must be released by calling Release
func (rs *ResourceSubscription) GetCollection() *Collection {
	rs.e.mu.Lock()
	return rs.collection
}

// GetModel will lock the EventSubscription for any changes
// and return the model map.
// The lock must be released by calling Release
func (rs *ResourceSubscription) GetModel() *Model {
	rs.e.mu.Lock()
	return rs.model
}

// Release releases the lock obtained by calling GetCollection or GetModel
func (rs *ResourceSubscription) Release() {
	rs.e.mu.Unlock()
}

func (rs *ResourceSubscription) handleEvent(r *ResourceEvent) {
	// Discard if event happened before resource was loaded
	if rs.state <= stateRequested {
		return
	}

	switch r.Event {
	case "change":
		if rs.resetting || !rs.handleEventChange(r) {
			return
		}
	case "add":
		if rs.resetting || !rs.handleEventAdd(r) {
			return
		}
	case "remove":
		if rs.resetting || !rs.handleEventRemove(r) {
			return
		}
	}

	rs.e.mu.Unlock()
	for sub := range rs.subs {
		sub.Event(r)
	}
	rs.e.mu.Lock()
}

func (rs *ResourceSubscription) handleEventChange(r *ResourceEvent) bool {
	if rs.state == stateCollection {
		rs.e.cache.Logf("Error processing event %s.%s: change event on collection", rs.e.ResourceName, r.Event)
		return false
	}

	props, err := codec.DecodeChangeEventData(r.Data)
	if err != nil {
		rs.e.cache.Logf("Error processing event %s.%s: %s", rs.e.ResourceName, r.Event, err)
	}

	// Clone old map using old map  size as capacity.
	// It might not be exact, but often sufficient
	m := make(map[string]codec.Value, len(rs.model.Values))
	for k, v := range rs.model.Values {
		m[k] = v
	}

	// Update model properties
	for k, v := range props {
		if v.Type == codec.ValueTypeDelete {
			delete(m, k)
		} else {
			m[k] = v
		}
	}

	r.Changed = props
	r.OldValues = rs.model.Values
	rs.model = &Model{Values: m}
	return true
}

func (rs *ResourceSubscription) handleEventAdd(r *ResourceEvent) bool {
	if rs.state == stateModel {
		rs.e.cache.Logf("Error processing event %s.%s: add event on model", rs.e.ResourceName, r.Event)
		return false
	}

	params, err := codec.DecodeAddEventData(r.Data)
	if err != nil {
		rs.e.cache.Logf("Error processing event %s.%s: %s", rs.e.ResourceName, r.Event, err)
		return false
	}

	idx := params.Idx
	old := rs.collection.Values
	l := len(old)

	if idx < 0 || idx > l {
		rs.e.cache.Logf("Error processing event %s.%s: Idx %d not valid", rs.e.ResourceName, r.Event, idx)
		return false
	}

	// Copy collection as the old slice might have been
	// passed to a Subscriber and should be considered immutable
	col := make([]codec.Value, l+1)
	copy(col, old[0:idx])
	copy(col[idx+1:], old[idx:])
	col[idx] = params.Value

	rs.collection = &Collection{Values: col}
	r.Idx = params.Idx
	r.Value = params.Value

	return true
}

func (rs *ResourceSubscription) handleEventRemove(r *ResourceEvent) bool {
	if rs.state == stateModel {
		rs.e.cache.Logf("Error processing event %s.%s: remove event on model", rs.e.ResourceName, r.Event)
		return false
	}

	params, err := codec.DecodeRemoveEventData(r.Data)
	if err != nil {
		rs.e.cache.Logf("Error processing event %s.%s: %s", rs.e.ResourceName, r.Event, err)
		return false
	}

	idx := params.Idx
	old := rs.collection.Values
	l := len(old)

	if idx < 0 || idx >= l {
		rs.e.cache.Logf("Error processing event %s.%s: Idx %d not valid", rs.e.ResourceName, r.Event, idx)
		return false
	}

	r.Value = old[idx]
	// Copy collection as the old slice might have been
	// passed to a Subscriber and should be considered immutable
	col := make([]codec.Value, l-1)
	copy(col, old[0:idx])
	copy(col[idx:], old[idx+1:])
	rs.collection = &Collection{Values: col}
	r.Idx = params.Idx

	return true
}

func (rs *ResourceSubscription) enqueueGetResponse(data []byte, err error) {
	rs.e.Enqueue(func() {
		rs, sublist := rs.processGetResponse(data, err)

		rs.e.mu.Unlock()
		defer rs.e.mu.Lock()
		if rs.state == stateError {
			for _, sub := range sublist {
				sub.Loaded(nil, rs.err)
			}
		} else {
			for _, sub := range sublist {
				sub.Loaded(rs, nil)
			}
		}
	})
}

// unregister deletes itself and all its links from
// the EventSubscription
func (rs *ResourceSubscription) unregister() {
	if rs.query == "" {
		rs.e.base = nil
	} else {
		delete(rs.e.queries, rs.query)
	}
	for _, q := range rs.links {
		delete(rs.e.links, q)
	}
	rs.links = nil
}

func (rs *ResourceSubscription) processGetResponse(payload []byte, err error) (nrs *ResourceSubscription, sublist []Subscriber) {
	var result *codec.GetResult
	// Either we have an error making the request
	// or an error in the service's response
	if err == nil {
		result, err = codec.DecodeGetResponse(payload)
	}

	// Assert a non-query request did not result in a query resource
	if err == nil && rs.query == "" && result.Query != "" {
		err = errQueryResourceOnNonQueryRequest
	}

	// Get request failed
	if err != nil {
		// Set state and store the error in case any other
		// subscriber are waiting on the Lock to subscribe
		rs.state = stateError
		rs.err = err

		// Clone subscribers to slice
		sublist = make([]Subscriber, len(rs.subs))
		i := 0
		for sub := range rs.subs {
			sublist[i] = sub
			i++
		}

		c := int64(len(sublist))
		rs.subs = nil
		rs.unregister()

		rs.e.removeCount(c)
		nrs = rs
		return
	}

	// Is the normalized query in the response different from the
	// one requested by the Subscriber?
	// Then we should create a link to the normalized query
	if result.Query != rs.query {
		nrs = rs.e.getResourceSubscription(result.Query)
		// Replace resource subscription with the normalized version
		if rs.e.links == nil {
			rs.e.links = make(map[string]*ResourceSubscription)
		}
		rs.e.links[rs.query] = nrs
		delete(rs.e.queries, rs.query)
		nrs.links = append(nrs.links, rs.query)

		// Copy over all subscribers
		for sub := range rs.subs {
			nrs.subs[sub] = struct{}{}
		}

	} else {
		nrs = rs
	}

	// Clone subscribers to slice
	sublist = make([]Subscriber, len(nrs.subs))
	i := 0
	for sub := range nrs.subs {
		sublist[i] = sub
		i++
	}

	// Exit if another request has already progressed the state.
	// Might happen when making a query subscription, directly followed by
	// another subscription using the normalized query of the previous.
	// When the second request returns, its resourceSubscription
	// will already be updated by the response from the first request.
	if nrs.state > stateRequested {
		return
	}

	if result.Model != nil {
		nrs.model = &Model{Values: result.Model}
		nrs.state = stateModel
	} else {
		nrs.collection = &Collection{Values: result.Collection}
		nrs.state = stateCollection
	}
	return
}

func (rs *ResourceSubscription) Unsubscribe(sub Subscriber) {
	rs.e.Enqueue(func() {
		if sub != nil {
			delete(rs.subs, sub)
		}

		if rs.query != "" && len(rs.subs) == 0 {
			rs.unregister()
		}

		rs.e.removeCount(1)
	})
}

func (rs *ResourceSubscription) handleResetResource() {
	// Are we already resetting. Then quick exit
	if rs.resetting {
		return
	}

	rs.resetting = true

	// Create request
	subj := "get." + rs.e.ResourceName
	payload := codec.CreateGetRequest(rs.query)
	rs.e.cache.mq.SendRequest(subj, payload, func(_ string, data []byte, err error) {
		rs.e.Enqueue(func() {
			rs.resetting = false
			rs.processResetGetResponse(data, err)
		})
	})
}

func (rs *ResourceSubscription) handleResetAccess() {
	for sub := range rs.subs {
		sub.Reaccess()
	}
}

func (rs *ResourceSubscription) processResetGetResponse(payload []byte, err error) {
	var result *codec.GetResult
	// Either we have an error making the request
	// or an error in the service's response
	if err == nil {
		result, err = codec.DecodeGetResponse(payload)
	}

	// Get request failed
	if err != nil {
		rs.e.cache.Logf("Subscription %s: Reset get error - %s", rs.e.ResourceName, err)
		return
	}

	switch rs.state {
	case stateModel:
		rs.processResetModel(result.Model)
	case stateCollection:
		rs.processResetCollection(result.Collection)
	}
}

func (rs *ResourceSubscription) processResetModel(props map[string]codec.Value) {
	// Update cached model properties
	vals := rs.model.Values
	for k, v := range props {
		if v.Equal(vals[k]) {
			delete(props, k)
		}
	}

	if len(props) == 0 {
		return
	}

	data, _ := json.Marshal(props)

	r := &ResourceEvent{
		Event: "change",
		Data:  json.RawMessage(data),
	}

	rs.handleEvent(r)
}

func (rs *ResourceSubscription) processResetCollection(collection []codec.Value) {
	events := lcs(rs.collection.Values, collection)

	for _, r := range events {
		rs.handleEvent(r)
	}
}

func lcs(a, b []codec.Value) []*ResourceEvent {
	var i, j int
	// Do a LCS matric calculation
	// https://en.wikipedia.org/wiki/Longest_common_subsequence_problem
	s := 0
	m := len(a)
	n := len(b)

	// Trim of matches at the start and end
	for s < m && s < n && a[s].Equal(b[s]) {
		s++
	}

	if m == n && s == m {
		return nil
	}

	for s <= m && s <= n && a[m-1].Equal(b[n-1]) {
		m--
		n--
	}

	var aa, bb []codec.Value
	if s > 0 || m < len(a) {
		aa = a[s:m]
		m = m - s
	} else {
		aa = a
	}
	if s > 0 || n < len(b) {
		bb = b[s:n]
		n = n - s
	} else {
		bb = b
	}

	// Create matrix and initialize it
	w := m + 1
	c := make([]int, w*(n+1))

	for i = 0; i < m; i++ {
		for j = 0; j < n; j++ {
			if aa[i].Equal(bb[j]) {
				c[(i+1)+w*(j+1)] = c[i+w*j] + 1
			} else {
				v1 := c[(i+1)+w*j]
				v2 := c[i+w*(j+1)]
				if v2 > v1 {
					c[(i+1)+w*(j+1)] = v2
				} else {
					c[(i+1)+w*(j+1)] = v1
				}
			}
		}
	}

	steps := make([]*ResourceEvent, 0, m+n-2*c[w*(n+1)-1])

	idx := m + s
	i = m
	j = n
	r := 0

	var adds [][3]int
	addCount := n - c[w*(n+1)-1]
	if addCount > 0 {
		adds = make([][3]int, 0, addCount)
	}
Loop:
	for {
		m = i - 1
		n = j - 1
		switch {
		case i > 0 && j > 0 && aa[m].Equal(bb[n]):
			idx--
			i--
			j--
		case j > 0 && (i == 0 || c[i+w*n] >= c[m+w*j]):
			adds = append(adds, [3]int{n, idx, r})
			j--
		case i > 0 && (j == 0 || c[i+w*n] < c[m+w*j]):
			idx--
			steps = append(steps, &ResourceEvent{
				Event: "remove",
				Data: codec.EncodeRemoveEventData(&codec.RemoveEventData{
					Idx: idx,
				}),
			})
			r++
			i--
		default:
			break Loop
		}
	}

	// Do the adds
	l := len(adds) - 1
	for i := l; i >= 0; i-- {
		add := adds[i]
		steps = append(steps, &ResourceEvent{
			Event: "add",
			Data: codec.EncodeAddEventData(&codec.AddEventData{
				Value: bb[add[0]],
				Idx:   add[1] - r + add[2] + l - i,
			}),
		})
	}

	return steps
}
