package test

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/resgateio/resgate/logger"
	"github.com/resgateio/resgate/server/mq"
	"github.com/resgateio/resgate/server/reserr"
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

// NewNATSTestClient creates a new NATSTestClient instance
func NewNATSTestClient(l logger.Logger) *NATSTestClient {
	return &NATSTestClient{l: l}
}

// Logf writes a formatted log message
func (c *NATSTestClient) Logf(format string, v ...interface{}) {
	if c.l != nil {
		c.l.Log(fmt.Sprintf(format, v...))
	}
}

// Errorf writes a formatted error message
func (c *NATSTestClient) Errorf(format string, v ...interface{}) {
	if c.l != nil {
		c.l.Error(fmt.Sprintf(format, v...))
	}
}

// Debugf writes a formatted debug message
func (c *NATSTestClient) Debugf(format string, v ...interface{}) {
	if c.l != nil && c.l.IsDebug() {
		c.l.Debug(fmt.Sprintf(format, v...))
	}
}

// Tracef writes a formatted trace message
func (c *NATSTestClient) Tracef(format string, v ...interface{}) {
	if c.l != nil && c.l.IsTrace() {
		c.l.Trace(fmt.Sprintf(format, v...))
	}
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
}

// SendRequest sends an asynchronous request on a subject, expecting the Response
// callback to be called once.
func (c *NATSTestClient) SendRequest(subj string, payload []byte, cb mq.Response, requestHeaders map[string][]string) {
	// Validate max control line size
	// 7  = nats inbox prefix length
	// 22 = nuid size
	if len(subj)+7+22 > nats.MAX_CONTROL_LINE_SIZE {
		go cb("", nil, nil, mq.ErrSubjectTooLong)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var p interface{}
	err := json.Unmarshal(payload, &p)
	if err != nil {
		panic("test: error unmarshaling request payload: " + err.Error())
	}

	r := &Request{
		Subject:    subj,
		RawPayload: payload,
		Payload:    p,
		c:          c,
		cb:         cb,
	}

	c.Tracef("<== %s: %s", subj, payload)
	if c.connected {
		c.reqs <- r
	} else {
		c.Errorf("Connection closed")
	}
}

// Subscribe to all events on a resource namespace.
// The namespace has the format "event."+resource
func (c *NATSTestClient) Subscribe(namespace string, cb mq.Response) (mq.Unsubscriber, error) {
	// Validate max control line size
	if len(namespace) > nats.MAX_CONTROL_LINE_SIZE-2 {
		return nil, mq.ErrSubjectTooLong
	}

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
	// Does nothing
}

// HasSubscriptions asserts that there is a subscription for the given resource IDs
func (c *NATSTestClient) HasSubscriptions(t *testing.T, rids ...string) {
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

// NoSubscriptions asserts that there isn't any subscription for the given
// resource IDs.
func (c *NATSTestClient) NoSubscriptions(t *testing.T, rids ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rid := range rids {
		if _, ok := c.subs["event."+rid]; ok {
			t.Fatalf("expected no subscription for event.%s.*, but found one", rid)
		}
	}
}

// ResourceEvent sends a resource event to resgate. The subject will be "event."+rid+"."+event .
// It panics if there is no subscription for such event.
func (c *NATSTestClient) ResourceEvent(rid string, event string, payload interface{}) {
	c.event("event."+rid, event, payload)
}

// ConnEvent sends a connection event to resgate. The subject will be "conn."+cid+"."+event .
// It panics if there is no subscription for such event.
func (c *NATSTestClient) ConnEvent(cid string, event string, payload interface{}) {
	c.event("conn."+cid, event, payload)
}

// SystemEvent sends a system event to resgate. The subject will be "system."+event .
// It panics if there is no subscription for such event.
func (c *NATSTestClient) SystemEvent(event string, payload interface{}) {
	c.event("system", event, payload)
}

// event sends an event to resgate. The subject will be ns+"."+event .
// It panics if there is no subscription for such event.
func (c *NATSTestClient) event(ns string, event string, payload interface{}) {
	c.mu.Lock()

	s, ok := c.subs[ns]
	if !ok {
		c.mu.Unlock()
		panic("test: no subscription for " + ns)
	}

	var data []byte
	var err error
	if data, ok = payload.([]byte); !ok {
		data, err = json.Marshal(payload)
		if err != nil {
			c.mu.Unlock()
			panic("test: error marshaling event: " + err.Error())
		}
	}

	c.mu.Unlock()
	subj := ns + "." + event
	c.Tracef("=>> %s: %s", subj, data)
	s.cb(subj, data, nil, nil)
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

	s.c.Tracef("U=> %s", s.ns)
	delete(s.c.subs, s.ns)
	return nil
}

// GetRequest gets a pending request that is sent to NATS.
// If no request is received within a set amount of time,
// it will log it as a fatal error.
func (c *NATSTestClient) GetRequest(t *testing.T) *Request {
	select {
	case r := <-c.reqs:
		return r
	case <-time.After(timeoutSeconds * time.Second):
		if t == nil {
			pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
			panic("expected a request but found none")
		} else {
			t.Fatal("expected a request but found none")
		}
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
	out, err := json.Marshal(data)
	if err != nil {
		panic("test: error marshaling response: " + err.Error())
	}
	r.RespondRaw(out)
}

// RespondRaw sends a raw byte response
func (r *Request) RespondRaw(out []byte) {
	r.c.Tracef("==> %s: %s", r.Subject, out)
	r.getCallback()("__RESPONSE_SUBJECT__", out, nil, nil)
}

// SendError sends an error response
func (r *Request) SendError(err error) {
	cb := r.getCallback()
	r.c.Tracef("X== %s: %s", r.Subject, err)
	cb("", nil, nil, err)
}

// RespondSuccess sends a success response
func (r *Request) RespondSuccess(result interface{}) {
	r.Respond(struct {
		Result interface{} `json:"result"`
	}{
		Result: result,
	})
}

// RespondResource sends a resource response
func (r *Request) RespondResource(rid string) {
	type Ref struct {
		RID string `json:"rid"`
	}
	r.Respond(struct {
		Resource Ref `json:"resource"`
	}{
		Resource: Ref{RID: rid},
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
func (r *Request) Equals(t *testing.T, subject string, payload interface{}) *Request {
	r.AssertSubject(t, subject)
	r.AssertPayload(t, payload)
	return r
}

// AssertSubject asserts that the request has the expected subject
func (r *Request) AssertSubject(t *testing.T, subject string) *Request {
	if r.Subject != subject {
		t.Fatalf("expected subject to be %#v, but got %#v", subject, r.Subject)
	}
	return r
}

// AssertPayload asserts that the request has the expected payload
func (r *Request) AssertPayload(t *testing.T, payload interface{}) *Request {
	var err error
	pj, err := json.Marshal(payload)
	if err != nil {
		panic("test: error marshaling assertion payload: " + err.Error())
	}

	var p interface{}
	err = json.Unmarshal(pj, &p)
	if err != nil {
		panic("test: error unmarshaling assertion payload: " + err.Error())
	}

	if !reflect.DeepEqual(p, r.Payload) {
		t.Fatalf("expected request payload to be:\n%s\nbut got:\n%s", pj, r.RawPayload)
	}
	return r
}

// AssertPathPayload asserts that a the request payload at a given dot-separated
// path in a nested object has the expected payload.
func (r *Request) AssertPathPayload(t *testing.T, path string, payload interface{}) *Request {
	pp := r.PathPayload(t, path)

	var err error
	pj, err := json.Marshal(payload)
	if err != nil {
		panic("test: error marshaling assertion path payload: " + err.Error())
	}
	var p interface{}
	err = json.Unmarshal(pj, &p)
	if err != nil {
		panic("test: error unmarshaling assertion path payload: " + err.Error())
	}

	if !reflect.DeepEqual(p, pp) {
		ppj, err := json.Marshal(pp)
		if err != nil {
			panic("test: error marshaling request path payload: " + err.Error())
		}

		t.Fatalf("expected request payload of path %#v to be:\n%s\nbut got:\n%s", path, pj, ppj)
	}
	return r
}

// AssertPathType asserts that a the request payload at a given dot-separated
// path in a nested object has the same type as typ.
func (r *Request) AssertPathType(t *testing.T, path string, typ interface{}) *Request {
	pp := r.PathPayload(t, path)

	ppt := reflect.TypeOf(pp)
	pt := reflect.TypeOf(typ)

	if ppt != pt {
		t.Fatalf("expected request payload of path %#v to be of type \"%s\", but got \"%s\"", path, pt, ppt)
	}
	return r
}

// PathPayload returns the request payload at a given dot-separated path in a nested object.
// It gives a fatal error if the path doesn't exist.
func (r *Request) PathPayload(t *testing.T, path string) interface{} {
	parts := strings.Split(path, ".")
	v := reflect.ValueOf(r.Payload)
	for _, part := range parts {
		if v.Kind() == reflect.Interface {
			v = v.Elem()
		}
		typ := v.Type()
		if typ.Kind() != reflect.Map {
			t.Fatalf("expected to find path %#v, but part %#v is of type %s", path, part, typ)
		}
		if typ.Key().Kind() != reflect.String {
			panic("test: key of part " + part + " of path " + path + " is not of type string")
		}
		v = v.MapIndex(reflect.ValueOf(part))
		if !v.IsValid() {
			t.Fatalf("expected to find path %#v, but missing map key %#v", path, part)
		}
	}

	return v.Interface()
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

// AssertPanic expects the callback function to panic, otherwise
// logs an error with t.Errorf
func AssertPanic(t *testing.T, cb func()) {
	defer func() {
		v := recover()
		if v == nil {
			t.Errorf("expected callback to panic, but it didn't")
		}
	}()
	cb()
}
