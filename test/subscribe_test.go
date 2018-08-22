package test

import "testing"

func TestConnect(t *testing.T) {
	s := Setup()
	defer Teardown(s)
}

func TestSubscribe(t *testing.T) {
	s := Setup()
	defer Teardown(s)

	s.Connect()

}
