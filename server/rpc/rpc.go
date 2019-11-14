package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/resgateio/resgate/server/codec"
	"github.com/resgateio/resgate/server/reserr"
)

// Requester has the methods required to perform a rpc request
type Requester interface {
	Reply(data []byte)
	GetResource(rid string, callback func(data *Resources, err error))
	SubscribeResource(rid string, callback func(data *Resources, err error))
	UnsubscribeResource(rid string, callback func(ok bool))
	CallResource(rid, action string, params interface{}, callback func(result interface{}, err error))
	AuthResource(rid, action string, params interface{}, callback func(result interface{}, err error))
	NewResource(rid string, params interface{}, callback func(data *NewResult, err error))
	SetVersion(protocol string) (string, error)
	ProtocolVersion() int
}

// Request represent a RES-client request
// https://github.com/resgateio/resgate/blob/master/docs/res-client-protocol.md#requests
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	ID     *uint64         `json:"id"`
}

// Response represents a RES-client response
type Response struct {
	Result interface{} `json:"result,omitempty"`
	ID     *uint64     `json:"id"`
}

// Event represent a RES-client event object
// https://github.com/resgateio/resgate/blob/master/docs/res-client-protocol.md#event-object
type Event struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data,omitempty"`
}

// ErrorResponse represents a JSON-RPC error response
type ErrorResponse struct {
	Error *reserr.Error `json:"error"`
	ID    *uint64       `json:"id"`
}

// Resources holds a resource information to be sent to the client
type Resources struct {
	Models      map[string]interface{}   `json:"models,omitempty"`
	Collections map[string]interface{}   `json:"collections,omitempty"`
	Errors      map[string]*reserr.Error `json:"errors,omitempty"`
}

// VersionRequest represents the params of a version request
type VersionRequest struct {
	Protocol string `json:"protocol"`
}

// VersionResult represents the results of a version request
type VersionResult struct {
	Protocol string `json:"protocol"`
}

// AddEvent represents a RES-client collection add event
// https://github.com/resgateio/resgate/blob/master/docs/res-client-protocol.md#collection-add-event
type AddEvent struct {
	Idx   int         `json:"idx"`
	Value interface{} `json:"value"`
	*Resources
}

// ChangeEvent represents a RES-client model change event
// https://github.com/resgateio/resgate/blob/master/docs/res-client-protocol.md#model-change-event
type ChangeEvent struct {
	Values interface{} `json:"values"`
	*Resources
}

// UnsubscribeEvent represents a RES-client unsubscribe event
// https://github.com/resgateio/resgate/blob/master/docs/res-client-protocol.md#unsubscribe-event
type UnsubscribeEvent struct {
	Reason *reserr.Error `json:"reason"`
}

// NewResult represents a RES-client result to a new request
type NewResult struct {
	RID string `json:"rid"`
	*Resources
}

// CallPayloadResult represents a RES-client result to a call or auth request with payload response
type CallPayloadResult struct {
	Payload json.RawMessage `json:"payload"`
}

// CallResourceResult represents a RES-client result to a call or auth request with resource response
type CallResourceResult struct {
	RID string `json:"rid"`
	*Resources
}

var (
	errMissingID = errors.New("Request is missing id property")
)

var nullBytes = []byte("null")

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
		if r.Method == "version" {
			var vr VersionRequest
			if data != nil && !bytes.Equal(r.Params, nullBytes) {
				err := json.Unmarshal(r.Params, &vr)
				if err != nil {
					req.Reply(r.ErrorResponse(reserr.ErrInvalidParams))
					return nil
				}
			}
			p, err := req.SetVersion(vr.Protocol)
			if err != nil {
				req.Reply(r.ErrorResponse(err))
				return nil
			}
			req.Reply(r.SuccessResponse(VersionResult{Protocol: p}))
			return nil
		}
		req.Reply(r.ErrorResponse(reserr.ErrInvalidRequest))
		return nil
	}

	var method string
	action := r.Method[:idx]
	rid := r.Method[idx+1:]

	if action == "call" || action == "auth" {
		idx = strings.LastIndexByte(rid, '.')
		if idx < 0 {
			req.Reply(r.ErrorResponse(reserr.ErrInvalidRequest))
			return nil
		}
		method = rid[idx+1:]
		if !codec.IsValidRID(method, false) {
			req.Reply(r.ErrorResponse(reserr.ErrInvalidRequest))
			return nil
		}
		rid = rid[:idx]
	}

	if !codec.IsValidRID(rid, true) {
		req.Reply(r.ErrorResponse(reserr.ErrInvalidRequest))
		return nil
	}

	switch action {
	case "get":
		req.GetResource(rid, func(data *Resources, err error) {
			if err != nil {
				req.Reply(r.ErrorResponse(err))
			} else {
				req.Reply(r.SuccessResponse(data))
			}
		})
	case "subscribe":
		req.SubscribeResource(rid, func(data *Resources, err error) {
			if err != nil {
				req.Reply(r.ErrorResponse(err))
			} else {
				req.Reply(r.SuccessResponse(data))
			}
		})
	case "unsubscribe":
		req.UnsubscribeResource(rid, func(ok bool) {
			if ok {
				req.Reply(r.SuccessResponse(nil))
			} else {
				req.Reply(r.ErrorResponse(reserr.ErrNoSubscription))
			}
		})
	case "call":
		req.CallResource(rid, method, r.Params, func(result interface{}, err error) {
			if err != nil {
				req.Reply(r.ErrorResponse(err))
			} else {
				req.Reply(r.SuccessResponse(result))
			}
		})

	case "auth":
		req.AuthResource(rid, method, r.Params, func(result interface{}, err error) {
			if err != nil {
				req.Reply(r.ErrorResponse(err))
			} else {
				req.Reply(r.SuccessResponse(result))
			}
		})

	case "new":
		req.NewResource(rid, r.Params, func(data *NewResult, err error) {
			if err != nil {
				req.Reply(r.ErrorResponse(err))
			} else {
				req.Reply(r.SuccessResponse(data))
			}
		})

	default:
		req.Reply(r.ErrorResponse(reserr.ErrInvalidRequest))
	}

	return nil
}

// SuccessResponse encodes a result to a request response
func (r *Request) SuccessResponse(result interface{}) []byte {
	out, _ := json.Marshal(Response{Result: result, ID: r.ID})
	return out
}

// NewEvent creates an encoded event to be sent to the client
func NewEvent(rid string, event string, data interface{}) []byte {
	out, _ := json.Marshal(Event{Event: rid + "." + event, Data: data})
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
