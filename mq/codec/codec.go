package codec

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jirenius/resgate/reserr"
)

var (
	noQueryGetRequest  = []byte(`{}`)
	errMissingResult   = reserr.InternalError(errors.New("Response missing result"))
	errInvalidResponse = reserr.InternalError(errors.New("Invalid service response"))
	errInvalidValue    = reserr.InternalError(errors.New("Invalid value"))
)

const (
	actionDelete = "delete"
)

type Request struct {
	Params interface{} `json:"params,omitempty"`
	Token  interface{} `json:"token,omitempty"`
	Query  string      `json:"query,omitempty"`
	CID    string      `json:"cid"`
}

type AuthRequest struct {
	Request
	Header     http.Header `json:"header,omitempty"`
	Host       string      `json:"host,omitempty"`
	RemoteAddr string      `json:"remoteAddr,omitempty"`
	URI        string      `json:"uri,omitempty"`
}

type GetRequest struct {
	Query string `json:"query,omitempty"`
}

type EventQueryRequest struct {
	Query string `json:"query"`
}

type EventQueryResult struct {
	Events []*EventQueryEvent `json:"events"`
}

type EventQueryResponse struct {
	Result *EventQueryResult `json:"result"`
	Error  *reserr.Error     `json:"error"`
}

type EventQueryEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type GetResult struct {
	Model      map[string]Value `json:"model"`
	Collection []Value          `json:"collection"`
	Query      string           `json:"query"`
}

type GetResponse struct {
	Result *GetResult    `json:"result"`
	Error  *reserr.Error `json:"error"`
}

type AccessResult struct {
	Get  bool   `json:"get"`
	Call string `json:"call"`
}

type AccessResponse struct {
	Result *AccessResult `json:"result"`
	Error  *reserr.Error `json:"error"`
}

type Response struct {
	Result json.RawMessage `json:"result"`
	Error  *reserr.Error   `json:"error"`
}

type NewResponse struct {
	Result *NewResult    `json:"result"`
	Error  *reserr.Error `json:"error"`
}

type NewResult struct {
	RID string `json:"rid"`
}

type QueryEvent struct {
	Subject string `json:"subject"`
}

type ConnTokenEvent struct {
	Token json.RawMessage `json:"token"`
}

type AddEventData struct {
	Idx   int   `json:"idx"`
	Value Value `json:"value"`
}

type RemoveEventData struct {
	Idx int `json:"idx"`
}

type SystemReset struct {
	Resources []string `json:"resources"`
	Access    []string `json:"access"`
}

type Requester interface {
	CID() string
}

type AuthRequester interface {
	CID() string
	HTTPRequest() *http.Request
}

type ValueType byte

const (
	ValueTypeNone ValueType = iota
	ValueTypePrimitive
	ValueTypeResource
	ValueTypeDelete
)

type Value struct {
	json.RawMessage
	Type ValueType
	RID  string
}

type ValueObject struct {
	RID    *string `json:"rid"`
	Action *string `json:"action"`
}

var DeleteValue = Value{
	RawMessage: json.RawMessage(`{"action":"delete"}`),
	Type:       ValueTypeDelete,
}

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

		if mvo.RID != nil {
			// Invalid to have both RID and Action set, or if RID is empty
			if mvo.Action != nil || *mvo.RID == "" {
				return errInvalidValue
			}
			v.Type = ValueTypeResource
			v.RID = *mvo.RID
		} else {
			// Must be an action of type actionDelete
			if mvo.Action == nil || *mvo.Action != actionDelete {
				return errInvalidValue
			}
			v.Type = ValueTypeDelete
		}
	case '[':
		return errInvalidValue
	default:
		v.Type = ValueTypePrimitive
	}

	return nil
}

func (v Value) Equal(w Value) bool {
	if v.Type != w.Type {
		return false
	}

	switch v.Type {
	case ValueTypePrimitive:
		return bytes.Equal(v.RawMessage, w.RawMessage)
	case ValueTypeResource:
		return v.RID == w.RID
	}

	return true
}

func CreateRequest(params interface{}, r Requester, query string, token interface{}) []byte {
	out, _ := json.Marshal(Request{Params: params, Token: token, Query: query, CID: r.CID()})
	return out
}

func CreateEventQueryRequest(query string) []byte {
	out, _ := json.Marshal(EventQueryRequest{Query: query})
	return out
}

func CreateGetRequest(query string) []byte {
	if query == "" {
		return noQueryGetRequest
	}
	out, _ := json.Marshal(GetRequest{Query: query})
	return out
}

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
			if v.Type != ValueTypeResource && v.Type != ValueTypePrimitive {
				return nil, errInvalidResponse
			}
		}
	} else {
		if res.Collection == nil {
			return nil, errInvalidResponse
		}
		// Assert collection only has proper values
		for _, v := range res.Collection {
			if v.Type != ValueTypeResource && v.Type != ValueTypePrimitive {
				return nil, errInvalidResponse
			}
		}
	}

	return r.Result, nil
}

func DecodeEvent(payload []byte) (json.RawMessage, error) {
	var ev json.RawMessage
	if payload == nil || len(payload) == 0 {
		return ev, nil
	}

	err := json.Unmarshal(payload, &ev)
	if err != nil {
		return nil, reserr.RESError(err)
	}
	return ev, nil
}

func DecodeQueryEvent(payload []byte) (*QueryEvent, error) {
	var qe QueryEvent
	err := json.Unmarshal(payload, &qe)
	if err != nil {
		return nil, reserr.RESError(err)
	}
	return &qe, nil
}

func DecodeEventQueryResponse(payload []byte) ([]*EventQueryEvent, error) {
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

	return r.Result.Events, nil
}

func DecodeChangeEventData(data json.RawMessage) (map[string]Value, error) {
	var r map[string]Value
	err := json.Unmarshal(data, &r)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func DecodeAddEventData(data json.RawMessage) (*AddEventData, error) {
	var d AddEventData
	err := json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}

	// Assert it is a proper value
	t := d.Value.Type
	if t != ValueTypeResource && t != ValueTypePrimitive {
		return nil, errInvalidValue
	}

	return &d, nil
}

func EncodeRemoveEventData(d *RemoveEventData) json.RawMessage {
	data, _ := json.Marshal(d)
	return json.RawMessage(data)
}

func EncodeAddEventData(d *AddEventData) json.RawMessage {
	data, _ := json.Marshal(d)
	return json.RawMessage(data)
}

func DecodeRemoveEventData(data json.RawMessage) (*RemoveEventData, error) {
	var d RemoveEventData
	err := json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

func DecodeAccessResponse(payload []byte) (*AccessResult, error) {
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

func DecodeCallResponse(payload []byte) (json.RawMessage, error) {
	var r Response
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

func DecodeNewResponse(payload []byte) (string, error) {
	var r NewResponse
	err := json.Unmarshal(payload, &r)
	if err != nil {
		return "", reserr.RESError(err)
	}

	if r.Error != nil {
		return "", r.Error
	}

	if r.Result == nil {
		return "", errMissingResult
	}

	if r.Result.RID == "" {
		return "", errInvalidResponse
	}

	return r.Result.RID, nil
}

func DecodeConnTokenEvent(payload []byte) (*ConnTokenEvent, error) {
	var e ConnTokenEvent
	err := json.Unmarshal(payload, &e)
	if err != nil {
		return nil, reserr.RESError(err)
	}
	return &e, nil
}

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
