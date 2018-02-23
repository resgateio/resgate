package rpc

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/jirenius/resgate/reserr"
)

// Requester has the methods required to perform a rpc request
type Requester interface {
	Send(data []byte)
	GetResource(resourceID string, callback func(data interface{}, err error))
	SubscribeResource(resourceID string, callback func(data interface{}, err error))
	UnsubscribeResource(resourceID string, callback func(ok bool))
	CallResource(resourceID, action string, params interface{}, callback func(result interface{}, err error))
	AuthResource(resourceID, action string, params interface{}, callback func(result interface{}, err error))
}

// Request represent a JSON-RPC request
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	ID     *uint64         `json:"id"`
}

// Response represents a JSON-RPC response
type Response struct {
	Result interface{} `json:"result,omitempty"`
	ID     *uint64     `json:"id"`
}

// Event represent a JSON-RPC event
type Event struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data,omitempty"`
}

// ErrorResponse represents a JSON-RPC error response
type ErrorResponse struct {
	Error *reserr.Error `json:"error"`
	ID    *uint64       `json:"id"`
}

// Resource holds a resource information to be sent to the client
type Resource struct {
	ResourceID string      `json:"resourceId"`
	Data       interface{} `json:"data,omitempty"`
	Error      error       `json:"error,omitempty"`
}

type AddEventResource struct {
	*Resource
	Idx int `json:"idx"`
}

type UnsubscribeEvent struct {
	Reason *reserr.Error `json:"reason"`
}

var (
	errMissingID      = errors.New("Request is missing id property")
	errNotImplemented = errors.New("Not implemented")
)

// HandleRequest unmarshals a request byte array and dispatches the request to the requester
func HandleRequest(data []byte, req Requester) error {
	r := &Request{}
	err := json.Unmarshal(data, r)
	if err != nil {
		return err
	}

	if r.ID == nil {
		return errMissingID
	}

	idx := strings.IndexByte(r.Method, '.')
	if idx < 0 {
		req.Send(r.ErrorResponse(reserr.ErrMethodNotFound))
		return nil
	}

	var method string
	action := r.Method[:idx]
	resourceID := r.Method[idx+1:]

	if action == "call" || action == "auth" {
		idx = strings.LastIndexByte(resourceID, '.')
		if idx < 0 {
			req.Send(r.ErrorResponse(reserr.ErrMethodNotFound))
			return nil
		}
		method = resourceID[idx+1:]
		resourceID = resourceID[:idx]
	}

	switch action {
	case "get":
		req.GetResource(resourceID, func(data interface{}, err error) {
			if err != nil {
				req.Send(r.ErrorResponse(err))
			} else {
				req.Send(r.SuccessResponse(data))
			}
		})
	case "subscribe":
		req.SubscribeResource(resourceID, func(data interface{}, err error) {
			if err != nil {
				req.Send(r.ErrorResponse(err))
			} else {
				req.Send(r.SuccessResponse(data))
			}
		})
	case "unsubscribe":
		req.UnsubscribeResource(resourceID, func(ok bool) {
			if ok {
				req.Send(r.SuccessResponse(nil))
			} else {
				req.Send(r.ErrorResponse(reserr.ErrNoSubscription))
			}
		})
	case "new":
		req.Send(r.ErrorResponse(errNotImplemented))
	case "call":
		req.CallResource(resourceID, method, r.Params, func(result interface{}, err error) {
			if err != nil {
				req.Send(r.ErrorResponse(err))
			} else {
				req.Send(r.SuccessResponse(result))
			}
		})

	case "auth":
		req.AuthResource(resourceID, method, r.Params, func(result interface{}, err error) {
			if err != nil {
				req.Send(r.ErrorResponse(err))
			} else {
				req.Send(r.SuccessResponse(result))
			}
		})
	default:
		req.Send(r.ErrorResponse(reserr.ErrMethodNotFound))
	}

	return nil
}

// SuccessResponse encodes a result to a request response
func (r *Request) SuccessResponse(result interface{}) []byte {
	out, _ := json.Marshal(Response{Result: result, ID: r.ID})
	return out
}

// NewEvent creates an encoded event to be sent to the client
func NewEvent(resourceID string, event string, data interface{}) []byte {
	out, _ := json.Marshal(Event{Event: resourceID + "." + event, Data: data})
	return out
}

// ErrorResponse encodes an error to a request response
func (r *Request) ErrorResponse(err error) []byte {
	rerr := reserr.RESError(err)
	d, err := json.Marshal(ErrorResponse{Error: rerr, ID: r.ID})
	if err != nil {
		return r.ErrorResponse(reserr.InternalError(err))
	}
	return d
}
