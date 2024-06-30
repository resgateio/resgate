package mq

import "github.com/resgateio/resgate/server/reserr"

// Response sends a response to the messaging system
type Response func(subj string, payload []byte, responseHeaders map[string][]string, err error)

// Unsubscriber is the interface that wraps the basic Unsubscribe method
type Unsubscriber interface {
	// Unsubscribe cancels the subscription
	Unsubscribe() error
}

// Client is an interface that represents a client to a messaging system.
type Client interface {
	// Connect establishes a connection to the MQ
	Connect() error

	// SendRequest sends an asynchronous request on a subject, expecting the Response
	// callback to be called once on a separate go routine.
	SendRequest(subject string, payload []byte, cb Response, requestHeaders map[string][]string)

	// Subscribe to all events on a resource namespace.
	// The namespace has the format "event."+resource
	Subscribe(namespace string, cb Response) (Unsubscriber, error)

	// Close closes the connection.
	Close()

	// IsClosed tests if the connection has been closed.
	IsClosed() bool

	// Sets the closed handler
	SetClosedHandler(cb func(error))
}

// ErrNoResponders is the error the client should pass to the Response
// when a call to SendRequest has no reponders.
var ErrNoResponders = reserr.ErrNotFound

// ErrRequestTimeout is the error the client should pass to the Response
// when a call to SendRequest times out
var ErrRequestTimeout = reserr.ErrTimeout

// ErrSubjectTooLong is the error the client should pass to the Response when
// the subject exceeds the maximum control line size
var ErrSubjectTooLong = reserr.ErrSubjectTooLong
