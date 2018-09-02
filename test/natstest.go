package test

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jirenius/resgate/logger"
	"github.com/jirenius/resgate/mq"
	"github.com/jirenius/resgate/reserr"
)

// Subscription implements the mq.Unsubscriber interface.
type Subscription struct {
	c  *NATSTestClient
	ns string
	cb mq.Response
}

// Request represent a request to NATS
type Request struct {
	Subject    string
	RawPayload []byte
	Payload    interface{}
	c          *NATSTestClient
	cb         mq.Response
}

// NATSTestClient holds a client connection to a nats server.
type NATSTestClient struct {
	l         logger.Logger
	subs      map[string]*Subscription
	reqs      chan *Request
	connected bool
	mu        sync.Mutex
}

// ParallelRequests holds multiple requests in undetermined order
type ParallelRequests []*Request

type responseCont struct {
	isReq bool
	f     mq.Response
}

func NewNATSTestClient(l logger.Logger) *NATSTestClient {
	return &NATSTestClient{l: l}
}

// Logf writes a formatted log message
func (c *NATSTestClient) Logf(format string, v ...interface{}) {
	if c.l == nil {
		return
	}
	c.l.Logf("[NATS] ", format, v...)
}

// Debugf writes a formatted debug message
func (c *NATSTestClient) Debugf(format string, v ...interface{}) {
	if c.l == nil {
		return
	}
	c.l.Debugf("[NATS] ", format, v...)
}

// Tracef writes a formatted trace message
func (c *NATSTestClient) Tracef(format string, v ...interface{}) {
	if c.l == nil {
		return
	}
	c.l.Tracef("[NATS] ", format, v...)
}

// Connect establishes a connection to the MQ
func (c *NATSTestClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs = make(map[string]*Subscription)
	c.reqs = make(chan *Request, 256)
	c.connected = true
	return nil
}

// IsClosed tests if the client connection has been closed.
func (c *NATSTestClient) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.connected
}

// Close closes the client connection.
func (c *NATSTestClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected {
		return
	}
	close(c.reqs)
	c.connected = false
	return
}

// SendRequest sends an asynchronous request on a subject, expecting the Response
// callback to be called once.
func (c *NATSTestClient) SendRequest(subj string, payload []byte, cb mq.Response) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var p interface{}
	err := json.Unmarshal(payload, &p)
	if err != nil {
		panic("test: error unmarshalling request payload: " + err.Error())
	}

	r := &Request{
		Subject:    subj,
		RawPayload: payload,
		Payload:    p,
		c:          c,
		cb:         cb,
	}

	c.Tracef("<== %s: %s", subj, payload)
	c.reqs <- r
}

// Subscribe to all events on a resource namespace.
// The namespace has the format "event."+resource
func (c *NATSTestClient) Subscribe(namespace string, cb mq.Response) (mq.Unsubscriber, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.subs[namespace]; ok {
		panic("test: subscription for " + namespace + " already exists")
	}

	s := &Subscription{c: c, ns: namespace, cb: cb}
	c.subs[namespace] = s
	c.Tracef("<=S %s", namespace)
	return s, nil
}

// SetClosedHandler sets the handler when the connection is closed
func (c *NATSTestClient) SetClosedHandler(_ func(error)) {
	return
}

// HasSubscription asserts that there is a subscription for the given resource IDs
func (c *NATSTestClient) HasSubcriptions(t *testing.T, rids ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(rids) != len(c.subs) {
		t.Errorf("expected %d subscription, found %d", len(rids), len(c.subs))
	}

	for _, rid := range rids {
		if _, ok := c.subs["event."+rid]; !ok {
			t.Fatalf("expected subscription for event.%s.* not found", rid)
		}
	}

	if len(rids) != len(c.subs) {
	next:
		for ns := range c.subs {
			for _, rid := range rids {
				if ns == "event."+rid {
					continue next
				}
			}
			t.Fatalf("expected no subscription for %s.*, but found one", ns)
		}
	}
}

// Event sends an event to resgate. The subject will be "event."+rid+"."+event .
// It panics if there is no subscription for such event.
func (c *NATSTestClient) Event(rid string, event string, payload interface{}) {
	c.mu.Lock()

	ns := "event." + rid
	s, ok := c.subs[ns]
	if !ok {
		c.mu.Unlock()
		panic("test: no subscription for " + ns)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		c.mu.Unlock()
		panic("test: error marshalling event: " + err.Error())
	}

	c.mu.Unlock()
	subj := ns + "." + event
	c.Tracef("E=> %s: %s", subj, data)
	s.cb(subj, data, nil)
}

// Unsubscribe removes the subscription.
func (s *Subscription) Unsubscribe() error {
	s.c.mu.Lock()
	defer s.c.mu.Unlock()

	v, ok := s.c.subs[s.ns]
	if !ok {
		panic("test: no subscription for " + s.ns)
	}
	if v != s {
		panic("test: subscription inconsistency")
	}

	s.c.Tracef("<=U %s", s.ns)
	delete(s.c.subs, s.ns)
	return nil
}

func (c *NATSTestClient) GetRequest(t *testing.T) *Request {
	select {
	case r := <-c.reqs:
		return r
	case <-time.After(1 * time.Second):
		t.Fatal("expected a request but found none")
	}
	return nil
}

// GetParallelRequests gets n number of requests where the order is uncertain.
func (c *NATSTestClient) GetParallelRequests(t *testing.T, n int) ParallelRequests {
	pr := make(ParallelRequests, n)
	for i := 0; i < n; i++ {
		pr[i] = c.GetRequest(t)
	}
	return pr
}

// getCallback returns the request's callback.
// It panics if the request is already responded to.
func (r *Request) getCallback() mq.Response {
	if r.cb == nil {
		panic("test: request already responded to")
	}

	cb := r.cb
	r.cb = nil
	return cb
}

// Respond sends a low level response
func (r *Request) Respond(data interface{}) {
	cb := r.getCallback()
	out, err := json.Marshal(data)
	if err != nil {
		panic("test: error marshalling response: " + err.Error())
	}

	r.c.Tracef("==> %s: %s", r.Subject, out)
	cb("__RESPONSE_SUBJECT__", out, nil)
}

// SendError sends an error response
func (r *Request) SendError(err error) {
	cb := r.getCallback()
	r.c.Tracef("X== %s: %s", r.Subject, err)
	cb("", nil, err)
}

// RespondSuccess sends a success response
func (r *Request) RespondSuccess(result interface{}) {
	r.Respond(struct {
		Result interface{} `json:"result"`
	}{
		Result: result,
	})
}

// RespondError sends an error response
func (r *Request) RespondError(err *reserr.Error) {
	r.Respond(struct {
		Error *reserr.Error `json:"error"`
	}{
		Error: err,
	})
}

// Timeout lets the request timeout
func (r *Request) Timeout() {
	r.SendError(mq.ErrRequestTimeout)
}

// Equals asserts that the request has the expected subject and payload
func (r *Request) Equals(t *testing.T, subject string, payload interface{}) {
	r.AssertSubject(t, subject)
	r.AssertPayload(t, payload)
}

// AssertSubject asserts that the request has the expected subject
func (r *Request) AssertSubject(t *testing.T, subject string) {
	if r.Subject != subject {
		t.Fatalf("expected subject to be %#v, but got %#v", subject, r.Subject)
	}
}

// AssertPayload asserts that the request has the expected payload
func (r *Request) AssertPayload(t *testing.T, payload interface{}) {
	var err error
	pj, err := json.Marshal(payload)
	if err != nil {
		panic("test: error marshalling assertion payload: " + err.Error())
	}

	var p interface{}
	err = json.Unmarshal(pj, &p)
	if err != nil {
		panic("test: error unmarshalling assertion payload: " + err.Error())
	}

	if !reflect.DeepEqual(p, r.Payload) {
		t.Fatalf("expected request payload to be:\n%s\nbut got:\n%s", pj, r.RawPayload)
	}
}

// Asserts that a the request payload at a given dot-separated path in a nested object
// has the expected payload.
func (r *Request) AssertPathPayload(t *testing.T, path string, payload interface{}) {
	parts := strings.Split(path, ".")
	v := reflect.ValueOf(r.Payload)
	for _, part := range parts {
		typ := v.Type()
		if typ.Kind() != reflect.Map {
			t.Fatalf("expected to find path %#v, but could not find %#v", path, part)
		}
		if typ.Key().Kind() != reflect.String {
			panic("test: key of part " + part + " of path " + path + " is not of type string")
		}
		v = v.MapIndex(reflect.ValueOf(part))
	}

	var err error
	pj, err := json.Marshal(payload)
	if err != nil {
		panic("test: error marshalling assertion path payload: " + err.Error())
	}
	var p interface{}
	err = json.Unmarshal(pj, &p)
	if err != nil {
		panic("test: error unmarshalling assertion path payload: " + err.Error())
	}

	pp := v.Interface()
	if !reflect.DeepEqual(p, pp) {
		ppj, err := json.Marshal(pp)
		if err != nil {
			panic("test: error marshalling request path payload: " + err.Error())
		}

		t.Fatalf("expected request payload of path %+v to be:\n%s\nbut got:\n%s", path, pj, ppj)
	}
}

// GetRequest returns a request based on subject.
func (pr ParallelRequests) GetRequest(t *testing.T, subject string) *Request {
	for _, r := range pr {
		if r.Subject == subject {
			return r
		}
	}

	t.Fatalf("expected parallel requests to contain subject %#v, but found none", subject)
	return nil
}
