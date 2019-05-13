package nats

import (
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/resgateio/resgate/logger"
	"github.com/resgateio/resgate/server/mq"
	"github.com/jirenius/timerqueue"
	nats "github.com/nats-io/go-nats"
)

const (
	natsChannelSize = 256
)

const logPrefix = "[NATS] "

// Client holds a client connection to a nats server.
type Client struct {
	RequestTimeout time.Duration
	URL            string
	Logger         logger.Logger

	mq      *nats.Conn
	mqCh    chan *nats.Msg
	mqReqs  map[*nats.Subscription]responseCont
	tq      *timerqueue.Queue
	mu      sync.Mutex
	stopped chan struct{}
}

// Subscription implements the mq.Unsubscriber interface.
type Subscription struct {
	c   *Client
	sub *nats.Subscription
}

type responseCont struct {
	isReq bool
	f     mq.Response
	t     *time.Timer
}

// Logf writes a formatted log message
func (c *Client) Logf(format string, v ...interface{}) {
	if c.Logger == nil {
		return
	}
	c.Logger.Logf(logPrefix, format, v...)
}

// Debugf writes a formatted debug message
func (c *Client) Debugf(format string, v ...interface{}) {
	if c.Logger == nil {
		return
	}
	c.Logger.Debugf(logPrefix, format, v...)
}

// Tracef writes a formatted trace message
func (c *Client) Tracef(format string, v ...interface{}) {
	if c.Logger == nil {
		return
	}
	c.Logger.Tracef(logPrefix, format, v...)
}

// Connect creates a connection to the nats server.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// No reconnects as all resources are instantly stale anyhow
	nc, err := nats.Connect(c.URL, nats.NoReconnect())
	if err != nil {
		return err
	}

	c.mq = nc
	c.mqCh = make(chan *nats.Msg, natsChannelSize)
	c.mqReqs = make(map[*nats.Subscription]responseCont)
	c.tq = timerqueue.New(c.onTimeout, c.RequestTimeout)
	c.stopped = make(chan struct{})

	go c.listener(c.mqCh, c.stopped)

	return nil
}

// IsClosed tests if the client connection has been closed.
func (c *Client) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mq == nil {
		return true
	}

	return c.mq.IsClosed()
}

// Close closes the client connection.
func (c *Client) Close() {
	c.mu.Lock()

	if c.mq == nil {
		c.mu.Unlock()
		return
	}

	if !c.mq.IsClosed() {
		c.mq.Close()
	}

	close(c.mqCh)
	c.mqCh = nil

	c.mq = nil
	// Set mqReqs to empty map to avoid possible nil reference error in listener
	c.mqReqs = make(map[*nats.Subscription]responseCont)

	c.tq.Clear()
	c.tq = nil

	stopped := c.stopped
	c.stopped = nil

	c.mu.Unlock()

	<-stopped
}

// SetClosedHandler sets the handler when the connection is closed
func (c *Client) SetClosedHandler(cb func(error)) {
	c.mq.SetClosedHandler(func(conn *nats.Conn) {
		cb(conn.LastError())
	})
}

// SendRequest sends a request to the MQ.
func (c *Client) SendRequest(subj string, payload []byte, cb mq.Response) {
	inbox := nats.NewInbox()

	c.mu.Lock()
	defer c.mu.Unlock()

	sub, err := c.mq.ChanSubscribe(inbox, c.mqCh)
	if err != nil {
		cb("", nil, err)
		return
	}

	c.Tracef("<== %s: %s", subj, payload)

	err = c.mq.PublishRequest(subj, inbox, payload)
	if err != nil {
		sub.Unsubscribe()
		cb("", nil, err)
		return
	}

	c.tq.Add(sub)
	c.mqReqs[sub] = responseCont{isReq: true, f: cb}
}

// Subscribe to all events on a resource namespace.
// The namespace has the format "event."+resource
func (c *Client) Subscribe(namespace string, cb mq.Response) (mq.Unsubscriber, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sub, err := c.mq.ChanSubscribe(namespace+".*", c.mqCh)
	if err != nil {
		return nil, err
	}

	c.mqReqs[sub] = responseCont{f: cb}

	us := &Subscription{c: c, sub: sub}
	return us, nil
}

// Unsubscribe removes the subscription.
func (s *Subscription) Unsubscribe() error {
	s.c.mu.Lock()
	defer s.c.mu.Unlock()

	delete(s.c.mqReqs, s.sub)
	return s.sub.Unsubscribe()
}

func (c *Client) listener(ch chan *nats.Msg, stopped chan struct{}) {
	for msg := range ch {
		c.mu.Lock()
		rc, ok := c.mqReqs[msg.Sub]
		if ok && rc.isReq {
			// Is the first character a-z or A-Z?
			// Then it is a meta response
			if len(msg.Data) > 0 && (msg.Data[0]|32)-'a' < 26 {
				c.parseMeta(msg, rc)
				c.mu.Unlock()
				c.Tracef("==> %s: %s", msg.Subject, msg.Data)
				continue
			}

			delete(c.mqReqs, msg.Sub)
			c.tq.Remove(msg.Sub)
			if rc.t != nil {
				rc.t.Stop()
			}
			msg.Sub.Unsubscribe()
		}
		c.mu.Unlock()

		if ok {
			c.Tracef("==> %s: %s", msg.Subject, msg.Data)
			rc.f(msg.Subject, msg.Data, nil)
		}
	}

	close(stopped)
}

func (c *Client) parseMeta(msg *nats.Msg, rc responseCont) {
	tag := reflect.StructTag(msg.Data)

	// timeout tag
	if v, ok := tag.Lookup("timeout"); ok {
		timeout, err := strconv.Atoi(v)
		if err == nil {
			if rc.t == nil {
				c.tq.Remove(msg.Sub)
			} else {
				rc.t.Stop()
			}
			rc.t = time.AfterFunc(time.Duration(timeout)*time.Millisecond, func() {
				c.onTimeout(msg.Sub)
			})
			c.mqReqs[msg.Sub] = rc
		}
	}
}

func (c *Client) onTimeout(v interface{}) {
	sub := v.(*nats.Subscription)

	c.mu.Lock()
	rc, ok := c.mqReqs[sub]
	delete(c.mqReqs, sub)
	c.mu.Unlock()

	if !ok {
		return
	}

	if rc.t != nil {
		rc.t.Stop()
	}
	sub.Unsubscribe()

	c.Tracef("x=> Request timeout")
	rc.f("", nil, mq.ErrRequestTimeout)
}
