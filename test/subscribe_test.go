package test

import "testing"

func TestSubscribe(t *testing.T) {
	s := Setup()
	Teardown(s)
}
