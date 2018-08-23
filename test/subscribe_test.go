package test

import "testing"

func TestStart(t *testing.T) {
	s := Setup()
	defer Teardown(s)
}

func TestConnectClient(t *testing.T) {
	s := Setup()
	defer Teardown(s)
	s.Connect()
}

func TestSubscribe(t *testing.T) {
	s := Setup()
	defer Teardown(s)
	c := s.Connect()
	c.Request("get.test.model", nil)
	r := s.GetRequest(t)
	r.Equals(t, "get.test.model", nil)
}
