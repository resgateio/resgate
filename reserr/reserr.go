package reserr

// Error represents an RES error
type Error struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

// Error converts an error to an *Error. If it isn't of type *Error already, it will become a system.internalError.
func RESError(err error) *Error {
	rerr, ok := err.(*Error)
	if !ok {
		rerr = InternalError(err)
	}
	return rerr
}

// InternalError converts an error to an *Error with the code system.internalError.
func InternalError(err error) *Error {
	return &Error{Code: "system.internalError", Message: "Internal error: " + err.Error()}
}

var (
	ErrAccessDenied   = &Error{Code: "system.accessDenied", Message: "Access denied"}
	ErrDisposing      = &Error{Code: "system.internalError", Message: "Internal error: disposing connection"}
	ErrInternalError  = &Error{Code: "system.internalError", Message: "Internal error"}
	ErrInvalidParams  = &Error{Code: "system.invalidParams", Message: "Invalid parameters"}
	ErrMethodNotFound = &Error{Code: "system.methodNotFound", Message: "Method not found"}
	ErrNoSubscription = &Error{Code: "system.noSubscription", Message: "No subscription"}
	ErrNotFound       = &Error{Code: "system.notFound", Message: "Not found"}
	ErrTimeout        = &Error{Code: "system.timeout", Message: "Request timeout"}
)
