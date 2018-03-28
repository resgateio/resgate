package mq

import "github.com/jirenius/resgate/reserr"

type Response func(subj string, payload []byte, err error)

type Unsubscriber interface {
	Unsubscribe() error
}

type Client interface {
	// Connect establishes a connection to the MQ
	Connect() error

	// SendRequest sends an asynchronous request on a subject, expecting the Response
	// callback to be called once.
	SendRequest(subject string, payload []byte, cb Response)

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

var (
	ErrRequestTimeout = reserr.ErrTimeout
)
