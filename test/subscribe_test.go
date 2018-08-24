package test

import (
	"encoding/json"
	"testing"
)

func TestStart(t *testing.T) {
	s := Setup()
	defer Teardown(s)
}

func TestConnectClient(t *testing.T) {
	s := Setup()
	defer Teardown(s)
	s.Connect()
}

func TestGetAndAccessOnSubscribe(t *testing.T) {
	s := Setup()
	defer Teardown(s)
	c := s.Connect()
	c.Request("get.test.model", nil)
	mreqs := s.GetParallelRequests(t, 2)

	// Validate get request
	req := mreqs.GetRequest(t, "get.test.model")
	req.AssertPayload(t, json.RawMessage(`{}`))

	// Validate access request
	req = mreqs.GetRequest(t, "access.test.model")
	req.AssertPathPayload(t, "token", json.RawMessage(`null`))
}

func TestResponseOnPrimitiveModelSubscribe(t *testing.T) {
	s := Setup()
	defer Teardown(s)
	c := s.Connect()
	creq := c.Request("get.test.model", nil)
	mreqs := s.GetParallelRequests(t, 2)

	model := `{"string":"foo","int":42,"bool":true,"null":null}`

	// Send get response
	req := mreqs.GetRequest(t, "get.test.model")
	req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

	// Send access response
	req = mreqs.GetRequest(t, "access.test.model")
	req.RespondSuccess(json.RawMessage(`{"get":true}`))

	// Validate client response
	cresp := creq.GetResponse(t)
	cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))
}

func TestResponseOnPrimitiveModelSubscribeAccessFirst(t *testing.T) {
	s := Setup()
	defer Teardown(s)
	c := s.Connect()
	creq := c.Request("get.test.model", nil)
	mreqs := s.GetParallelRequests(t, 2)

	model := `{"string":"foo","int":42,"bool":true,"null":null}`

	// Send access response
	req := mreqs.GetRequest(t, "access.test.model")
	req.RespondSuccess(json.RawMessage(`{"get":true}`))

	// Send get response
	req = mreqs.GetRequest(t, "get.test.model")
	req.RespondSuccess(json.RawMessage(`{"model":` + model + `}`))

	// Validate client response
	cresp := creq.GetResponse(t)
	cresp.AssertResult(t, json.RawMessage(`{"models":{"test.model":`+model+`}}`))
}
