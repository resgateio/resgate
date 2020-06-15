package codec

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/resgateio/resgate/server/reserr"
)

var (
	noQueryGetRequest  = []byte(`{}`)
	errMissingResult   = reserr.InternalError(errors.New("response missing result"))
	errInvalidResponse = reserr.InternalError(errors.New("invalid service response"))
	errInvalidValue    = reserr.InternalError(errors.New("invalid value"))
)

const (
	actionDelete = "delete"
)

// Request represents a RES-service request
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#requests
type Request struct {
	Params interface{} `json:"params,omitempty"`
	Token  interface{} `json:"token,omitempty"`
	Query  string      `json:"query,omitempty"`
	CID    string      `json:"cid"`
}

// Response represents a RES-service response
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#response
type Response struct {
	Result   json.RawMessage `json:"result"`
	Resource *Resource       `json:"resource"`
	Error    *reserr.Error   `json:"error"`
}

// AccessResponse represents the response of a RES-service access request
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#access-request
type AccessResponse struct {
	Result *AccessResult `json:"result"`
	Error  *reserr.Error `json:"error"`
}

// AccessResult represents the response result of a RES-service access request
type AccessResult struct {
	Get  bool   `json:"get"`
	Call string `json:"call"`
}

// GetRequest represents a RES-service get request
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#get-request
type GetRequest struct {
	Query string `json:"query,omitempty"`
}

// GetResponse represents the response of a RES-service get request
type GetResponse struct {
	Result *GetResult    `json:"result"`
	Error  *reserr.Error `json:"error"`
}

// GetResult represent the response result of a RES-service get request
type GetResult struct {
	Model      map[string]Value `json:"model"`
	Collection []Value          `json:"collection"`
	Query      string           `json:"query"`
}

// AuthRequest represents a RES-service auth request
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#auth-request
type AuthRequest struct {
	Request
	Header     http.Header `json:"header,omitempty"`
	Host       string      `json:"host,omitempty"`
	RemoteAddr string      `json:"remoteAddr,omitempty"`
	URI        string      `json:"uri,omitempty"`
}

// NewResponse represents the response of a RES-service new call request
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#new-call-request
type NewResponse struct {
	Result *Resource     `json:"result"`
	Error  *reserr.Error `json:"error"`
}

// Resource represents the resource response of a RES-service call or auth request
type Resource struct {
	RID string `json:"rid"`
}

// QueryEvent represents a RES-service query event
type QueryEvent struct {
	Subject string `json:"subject"`
}

// EventQueryRequest represents a RES-service query request
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#query-request
type EventQueryRequest struct {
	Query string `json:"query"`
}

// EventQueryResponse represent the response of a RES-service query request
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#query-event
type EventQueryResponse struct {
	Result *EventQueryResult `json:"result"`
	Error  *reserr.Error     `json:"error"`
}

// EventQueryResult represent the response's result part of a RES-service
// query request
type EventQueryResult struct {
	Events     []*EventQueryEvent `json:"events"`
	Model      map[string]Value   `json:"model"`
	Collection []Value            `json:"collection"`
}

// EventQueryEvent represents an event in the response of a RES-server query request
type EventQueryEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// ConnTokenEvent represents a RES-server connection token event
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#connection-token-event
type ConnTokenEvent struct {
	Token json.RawMessage `json:"token"`
}

// ChangeEvent represent a RES-server model change event
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#model-change-event
type ChangeEvent struct {
	Values map[string]Value `json:"values"`
}

// AddEvent represent a RES-server collection add event
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#collection-add-event
type AddEvent struct {
	Idx   int   `json:"idx"`
	Value Value `json:"value"`
}

// RemoveEvent represent a RES-server collection remove event
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#collection-remove-event
type RemoveEvent struct {
	Idx int `json:"idx"`
}

// SystemReset represents a RES-server system reset event
// https://github.com/resgateio/resgate/blob/master/docs/res-service-protocol.md#system-reset-event
type SystemReset struct {
	Resources []string `json:"resources"`
	Access    []string `json:"access"`
}

// Requester is the connection making the request
type Requester interface {
	// CID returns the connection of the requester
	CID() string
}

// AuthRequester is the connection making the auth request
type AuthRequester interface {
	// CID returns the connection of the requester
	CID() string
	// HTTPRequest returns the http.Request from requesters (upgraded) HTTP connection
	HTTPRequest() *http.Request
}

// ValueType is an enum reprenting the value type
type ValueType byte

// Value type constants
const (
	ValueTypeNone ValueType = iota
	ValueTypeDelete
	ValueTypePrimitive
	ValueTypeReference
	ValueTypeSoftReference
	ValueTypeData
)

// Value represents a RES value
// https://github.com/resgateio/resgate/blob/master/docs/res-protocol.md#values
type Value struct {
	json.RawMessage
	Type  ValueType
	RID   string
	Inner json.RawMessage
}

// ValueObject represents a resource reference or an action
type ValueObject struct {
	RID    *string         `json:"rid"`
	Soft   bool            `json:"soft"`
	Action *string         `json:"action"`
	Data   json.RawMessage `json:"data"`
}

// IsProper returns true if the value's type is either a primitive, a
// reference, or a data value.
func (v Value) IsProper() bool {
	return v.Type >= ValueTypePrimitive
}

// DeleteValue is a predeclared delete action value
var DeleteValue = Value{
	RawMessage: json.RawMessage(`{"action":"delete"}`),
	Type:       ValueTypeDelete,
}

// UnmarshalJSON sets *v to the RES value represented by the JSON encoded data
func (v *Value) UnmarshalJSON(data []byte) error {
	err := v.RawMessage.UnmarshalJSON(data)
	if err != nil {
		return err
	}

	// Get first non-whitespace character
	var c byte
	i := 0
	for {
		c = v.RawMessage[i]
		if c != 0x20 && c != 0x09 && c != 0x0A && c != 0x0D {
			break
		}
		i++
	}

	switch c {
	case '{':
		var mvo ValueObject
		err = json.Unmarshal(v.RawMessage, &mvo)
		if err != nil {
			return err
		}

		switch {
		case mvo.RID != nil:
			// Invalid to have both RID and Action or Data set, or if RID is empty
			if mvo.Action != nil || mvo.Data != nil || *mvo.RID == "" {
				return errInvalidValue
			}
			v.RID = *mvo.RID
			if !IsValidRID(v.RID, true) {
				return errInvalidValue
			}
			if mvo.Soft {
				v.Type = ValueTypeSoftReference
			} else {
				v.Type = ValueTypeReference
			}
		case mvo.Action != nil:
			// Invalid to have both Action and Data set, or if action is not actionDelete
			if mvo.Data != nil || *mvo.Action != actionDelete {
				return errInvalidValue
			}
			v.Type = ValueTypeDelete
		case mvo.Data != nil:
			v.Inner = mvo.Data
			dc := mvo.Data[0]
			// Is data containing a primitive?
			if dc == '{' || dc == '[' {
				v.Type = ValueTypeData
			} else {
				v.RawMessage = mvo.Data
				v.Type = ValueTypePrimitive
			}
		default:
			return errInvalidValue
		}
	case '[':
		return errInvalidValue
	default:
		v.Type = ValueTypePrimitive
	}

	return nil
}

// Equal reports whether v and w is equal in type and value
func (v Value) Equal(w Value) bool {
	if v.Type != w.Type {
		return false
	}

	switch v.Type {
	case ValueTypeData:
		fallthrough
	case ValueTypePrimitive:
		return bytes.Equal(v.RawMessage, w.RawMessage)
	case ValueTypeReference:
		fallthrough
	case ValueTypeSoftReference:
		return v.RID == w.RID
	}

	return true
}

// CreateRequest creates a JSON encoded RES-service request
func CreateRequest(params interface{}, r Requester, query string, token interface{}) []byte {
	out, _ := json.Marshal(Request{Params: params, Token: token, Query: query, CID: r.CID()})
	return out
}

// CreateGetRequest creates a JSON encoded RES-service get request
func CreateGetRequest(query string) []byte {
	if query == "" {
		return noQueryGetRequest
	}
	out, _ := json.Marshal(GetRequest{Query: query})
	return out
}

// CreateAuthRequest creates a JSON encoded RES-service auth request
func CreateAuthRequest(params interface{}, r AuthRequester, query string, token interface{}) []byte {
	hr := r.HTTPRequest()
	out, _ := json.Marshal(AuthRequest{
		Request:    Request{Params: params, Token: token, Query: query, CID: r.CID()},
		Header:     hr.Header,
		Host:       hr.Host,
		RemoteAddr: hr.RemoteAddr,
		URI:        hr.RequestURI,
	})
	return out
}

// DecodeGetResponse decodes a JSON encoded RES-service get response
func DecodeGetResponse(payload []byte) (*GetResult, error) {
	var r GetResponse
	err := json.Unmarshal(payload, &r)
	if err != nil {
		return nil, reserr.InternalError(err)
	}

	if r.Error != nil {
		return nil, r.Error
	}

	if r.Result == nil {
		return nil, errMissingResult
	}

	// Assert we got either a model or a collection
	res := r.Result
	if res.Model != nil {
		if res.Collection != nil {
			return nil, errInvalidResponse
		}
		// Assert model only has proper values
		for _, v := range res.Model {
			if !v.IsProper() {
				return nil, errInvalidResponse
			}
		}
	} else if res.Collection != nil {
		// Assert collection only has proper values
		for _, v := range res.Collection {
			if !v.IsProper() {
				return nil, errInvalidResponse
			}
		}
	} else {
		return nil, errInvalidResponse
	}

	return r.Result, nil
}

// DecodeEvent decodes a JSON encoded RES-service event
func DecodeEvent(payload []byte) (json.RawMessage, error) {
	var ev json.RawMessage
	if len(payload) == 0 {
		return ev, nil
	}

	err := json.Unmarshal(payload, &ev)
	if err != nil {
		return nil, reserr.RESError(err)
	}
	return ev, nil
}

// DecodeQueryEvent decodes a JSON encoded query event
func DecodeQueryEvent(payload []byte) (*QueryEvent, error) {
	var qe QueryEvent
	err := json.Unmarshal(payload, &qe)
	if err != nil {
		return nil, reserr.RESError(err)
	}
	return &qe, nil
}

// CreateEventQueryRequest creates a JSON encoded RES-service event query request
func CreateEventQueryRequest(query string) []byte {
	out, _ := json.Marshal(EventQueryRequest{Query: query})
	return out
}

// DecodeEventQueryResponse decodes a JSON encoded RES-service event query response
func DecodeEventQueryResponse(payload []byte) (*EventQueryResult, error) {
	var r EventQueryResponse
	err := json.Unmarshal(payload, &r)
	if err != nil {
		return nil, reserr.RESError(err)
	}

	if r.Error != nil {
		return nil, r.Error
	}

	if r.Result == nil {
		return nil, errMissingResult
	}

	// Assert we got either a model or a collection
	res := r.Result
	switch {
	case res.Events != nil:
		if res.Model != nil || res.Collection != nil {
			return nil, errInvalidResponse
		}
	case res.Model != nil:
		if res.Collection != nil {
			return nil, errInvalidResponse
		}
		// Assert model only has proper values
		for _, v := range res.Model {
			if !v.IsProper() {
				return nil, errInvalidResponse
			}
		}
	case res.Collection != nil:
		// Assert collection only has proper values
		for _, v := range res.Collection {
			if !v.IsProper() {
				return nil, errInvalidResponse
			}
		}
	}

	return res, nil
}

// IsLegacyChangeEvent returns true if the model change event is detected as v1.0 legacy
// [DEPRECATED:deprecatedModelChangeEvent]
func IsLegacyChangeEvent(data json.RawMessage) bool {
	var r map[string]json.RawMessage
	err := json.Unmarshal(data, &r)
	if err != nil {
		return false
	}

	if len(r) != 1 {
		return true
	}

	v, ok := r["values"]
	if !ok {
		return true
	}

	for _, c := range v {
		// Check character unless it is a whitespace
		if c != '\t' && c != '\n' && c != '\r' && c != ' ' {
			return c != '{'
		}
	}
	return true
}

// EncodeChangeEvent creates a JSON encoded RES-service change event
func EncodeChangeEvent(values map[string]Value) json.RawMessage {
	data, _ := json.Marshal(ChangeEvent{Values: values})
	return json.RawMessage(data)
}

// DecodeChangeEvent decodes a JSON encoded RES-service model change event
func DecodeChangeEvent(data json.RawMessage) (map[string]Value, error) {
	var r ChangeEvent
	err := json.Unmarshal(data, &r)
	if err != nil {
		return nil, err
	}

	return r.Values, nil
}

// DecodeLegacyChangeEvent decodes a JSON encoded RES-service v1.0 model change event
func DecodeLegacyChangeEvent(data json.RawMessage) (map[string]Value, error) {
	var r map[string]Value
	err := json.Unmarshal(data, &r)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// EncodeAddEvent creates a JSON encoded RES-service collection add event
func EncodeAddEvent(d *AddEvent) json.RawMessage {
	data, _ := json.Marshal(d)
	return json.RawMessage(data)
}

// DecodeAddEvent decodes a JSON encoded RES-service collection add event
func DecodeAddEvent(data json.RawMessage) (*AddEvent, error) {
	var d AddEvent
	err := json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}

	// Assert it is a proper value
	if !d.Value.IsProper() {
		return nil, errInvalidValue
	}

	return &d, nil
}

// EncodeRemoveEvent creates a JSON encoded RES-service collection remove event
func EncodeRemoveEvent(d *RemoveEvent) json.RawMessage {
	data, _ := json.Marshal(d)
	return json.RawMessage(data)
}

// DecodeRemoveEvent decodes a JSON encoded RES-service collection remove event
func DecodeRemoveEvent(data json.RawMessage) (*RemoveEvent, error) {
	var d RemoveEvent
	err := json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

// DecodeAccessResponse decodes a JSON encoded RES-service access response
func DecodeAccessResponse(payload []byte) (*AccessResult, *reserr.Error) {
	var r AccessResponse
	err := json.Unmarshal(payload, &r)
	if err != nil {
		return nil, reserr.RESError(err)
	}

	if r.Error != nil {
		return nil, r.Error
	}

	if r.Result == nil {
		return nil, errMissingResult
	}

	return r.Result, nil
}

// DecodeCallResponse decodes a JSON encoded RES-service call response
func DecodeCallResponse(payload []byte) (json.RawMessage, string, error) {
	var r Response
	err := json.Unmarshal(payload, &r)
	if err != nil {
		return nil, "", reserr.RESError(err)
	}

	if r.Error != nil {
		return nil, "", r.Error
	}

	if r.Resource != nil {
		rid := r.Resource.RID
		if !IsValidRID(rid, true) {
			return nil, "", errInvalidResponse
		}
		return nil, rid, nil
	}

	if r.Result == nil {
		return nil, "", errMissingResult
	}

	return r.Result, "", nil
}

// TryDecodeLegacyNewResult tries to detect legacy v1.1.1 behavior.
// Returns empty string and nil error when the result is not detected as legacy.
// [DEPRECATED:deprecatedNewCallRequest]
func TryDecodeLegacyNewResult(result json.RawMessage) (string, error) {
	var r map[string]interface{}
	err := json.Unmarshal(result, &r)
	if err != nil {
		return "", nil
	}

	if len(r) != 1 {
		return "", nil
	}

	rid, ok := r["rid"].(string)
	if !ok {
		return "", nil
	}

	if !IsValidRID(rid, true) {
		return "", errInvalidResponse
	}

	return rid, nil
}

// DecodeConnTokenEvent decodes a JSON encoded RES-service connection token event
func DecodeConnTokenEvent(payload []byte) (*ConnTokenEvent, error) {
	var e ConnTokenEvent
	err := json.Unmarshal(payload, &e)
	if err != nil {
		return nil, reserr.RESError(err)
	}
	return &e, nil
}

// DecodeSystemReset decodes a JSON encoded RES-service system reset event
func DecodeSystemReset(data json.RawMessage) (SystemReset, error) {
	var r SystemReset
	if len(data) == 0 {
		return r, nil
	}

	err := json.Unmarshal(data, &r)
	if err != nil {
		return r, err
	}

	return r, nil
}

// IsValidRID returns true if the RID is valid, otherwise false.
// If allowQuery flag is false, encountering a question mark (?) will
// cause IsValidRID to return false.
func IsValidRID(rid string, allowQuery bool) bool {
	start := true
	for _, r := range rid {
		if r == '?' {
			return allowQuery && !start
		}
		if r < 33 || r > 126 || r == '*' || r == '>' {
			return false
		}
		if r == '.' {
			if start {
				return false
			}
			start = true
		} else {
			start = false
		}
	}

	return !start
}

// IsValidRIDPart returns true if the RID part is valid, otherwise false.
func IsValidRIDPart(part string) bool {
	for _, r := range part {
		if r < 33 || r > 126 || r == '.' || r == '*' || r == '>' || r == '?' {
			return false
		}
	}
	return len(part) > 0
}
