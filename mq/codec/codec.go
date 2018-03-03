package codec

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jirenius/resgate/reserr"
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
	Model      map[string]json.RawMessage `json:"model"`
	Collection []string                   `json:"collection"`
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

type QueryEvent struct {
	Subject string `json:"subject"`
}

type ConnTokenEvent struct {
	Token json.RawMessage `json:"token"`
}

type AddEventData struct {
	ResourceID string `json:"resourceId"`
	Idx        int    `json:"idx"`
}

type RemoveEventData struct {
	ResourceID string `json:"resourceId"`
	Idx        int    `json:"idx"`
}

type SystemReset struct {
	Resources []string `json:"resources"`
}

type Requester interface {
	CID() string
}

type AuthRequester interface {
	CID() string
	HTTPRequest() *http.Request
}

var noQueryGetRequest = []byte(`{"query":null}`)
var errMissingResult = reserr.InternalError(errors.New("Response missing result"))

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

func CreateAuthRequest(params interface{}, r AuthRequester, token interface{}) []byte {
	hr := r.HTTPRequest()
	out, _ := json.Marshal(AuthRequest{
		Request:    Request{Params: params, Token: token, CID: r.CID()},
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

	return r.Result, nil
}

func DecodeEvent(payload []byte) (json.RawMessage, error) {
	var ev json.RawMessage
	if payload == nil || len(payload) == 0 {
		return ev, nil
	}

	err := json.Unmarshal(payload, &ev)
	if err != nil {
		return nil, reserr.InternalError(err)
	}
	return ev, nil
}

func DecodeQueryEvent(payload []byte) (*QueryEvent, error) {
	var qe QueryEvent
	err := json.Unmarshal(payload, &qe)
	if err != nil {
		return nil, reserr.InternalError(err)
	}
	return &qe, nil
}

func DecodeEventQueryResponse(payload []byte) ([]*EventQueryEvent, error) {
	var r EventQueryResponse
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

	return r.Result.Events, nil
}

func DecodeChangeEventData(data json.RawMessage) (map[string]json.RawMessage, error) {
	var r map[string]json.RawMessage
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
		return nil, reserr.InternalError(err)
	}

	if r.Error != nil {
		return nil, err
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
		return nil, reserr.InternalError(err)
	}

	if r.Error == nil {
		return r.Result, nil
	}

	return nil, r.Error
}

func DecodeConnTokenEvent(payload []byte) (*ConnTokenEvent, error) {
	var e ConnTokenEvent
	err := json.Unmarshal(payload, &e)
	if err != nil {
		return nil, reserr.InternalError(err)
	}
	return &e, nil
}

func DecodeSystemReset(data json.RawMessage) ([]string, error) {
	var r SystemReset
	if len(data) == 0 {
		return nil, nil
	}

	err := json.Unmarshal(data, &r)
	if err != nil {
		return nil, err
	}

	return r.Resources, nil
}
