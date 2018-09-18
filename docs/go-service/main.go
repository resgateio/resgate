/*
This is an example of a simple RES service written in Go.
* It exposes a single resource: "exampleService.myModel".
* It allows setting the resource's Message property through the "set" method.
* It will stop on connection problems.

Visit https://github.com/jirenius/resgate#client for the matching client.
*/
package main

import (
	"encoding/json"
	"os"
	"os/signal"

	"github.com/nats-io/go-nats"
)

// Request represent a RES request
type Request struct {
	Params json.RawMessage `json:"params"`
	CID    string          `json:"cid"`
	Token  json.RawMessage `json:"token"`
}

// Response represents a response to a RES request
type Response struct {
	Result interface{} `json:"result"`
}

// ModelResponse represents the model response to a RES get request
type ModelResponse struct {
	Model interface{} `json:"model"`
}

// Model represents the custom model
type Model struct {
	Message string `json:"message"`
}

var myModel = &Model{Message: "Hello Go World"}

// Static responses and events
var (
	responseAllAccess          = []byte(`{ "result": { "get": true, "call": "set" }}`)
	responseSuccess            = []byte(`{ "result": null }`)
	responseInternalError      = []byte(`{ "error": { "code": "system.internalError", "message": "Internal error" }}`)
	responseInvalidParamsError = []byte(`{ "error": { "code": "system.invalidParams", "message": "Invalid parameters" }}`)
	eventSystemReset           = []byte(`{ "resources": [ "exampleService.>" ]}`)
)

// Globals for handling subscriptions
var (
	ncSubs = make(map[*nats.Subscription]func(*nats.Msg))
	inCh   = make(chan *nats.Msg, 32)
)

// natsSubscribe is like nc.Subscribe, except it handles all callbacks
// on the same go routine. This is because all events, get request
// and call request for any single resource must be synchronized.
// Synchronization with go routines is preferred over mutexes
func natsSubscribe(nc *nats.Conn, subj string, cb func(*nats.Msg)) {
	sub, err := nc.ChanSubscribe(subj, inCh)
	if err != nil {
		panic(err)
	}

	ncSubs[sub] = cb
}

// listener listens for incoming messages and calls the callback.
// In an example with multiple resources, the listener would dispatch
// the callback to a pool of go routines based on the resource name.
// Only one go routine would work with each unique resource at any given time.
func listener() {
	for m := range inCh {
		ncSubs[m.Sub](m)
	}
}

func main() {
	// Connect to NATS Server
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		panic(err)
	}
	defer nc.Close()

	// Access listener. Everyone gets read access and access to call the set-method
	natsSubscribe(nc, "access.exampleService.myModel", func(m *nats.Msg) {
		nc.Publish(m.Reply, responseAllAccess)
	})

	// Get listener. Reply with the json encoded model
	natsSubscribe(nc, "get.exampleService.myModel", func(m *nats.Msg) {
		dta, err := json.Marshal(Response{Result: ModelResponse{Model: myModel}})
		if err != nil {
			nc.Publish(m.Reply, responseInternalError)
		} else {
			nc.Publish(m.Reply, dta)
		}
	})

	// Set listener for updating the myModel.message property
	natsSubscribe(nc, "call.exampleService.myModel.set", func(m *nats.Msg) {
		var r Request
		err := json.Unmarshal(m.Data, &r)
		if err != nil {
			nc.Publish(m.Reply, responseInternalError)
			return
		}

		// Anonymous struct for unmarshalling parameters
		var p struct {
			Message *string `json:"message,omitempty"`
		}
		err = json.Unmarshal(r.Params, &p)
		if err != nil {
			nc.Publish(m.Reply, responseInvalidParamsError)
			return
		}

		// Check if the message property was changed
		if p.Message != nil && *p.Message != myModel.Message {
			// Update the model
			myModel.Message = *p.Message
			// Send a change event with updated fields
			dta, err := json.Marshal(p)
			if err != nil {
				panic(err)
			}
			nc.Publish("event.exampleService.myModel.change", dta)
		}

		// Send success response
		nc.Publish(m.Reply, responseSuccess)
	})

	// System resets tells resgate that the service's resources it has cached
	// might no longer be valid. Resgate will then update any cached resources
	// from exampleService.
	nc.Publish("system.reset", eventSystemReset)

	// System resets also needs to be sent on reconnects, as the service
	// cannot guarantee no change events were lost due to connection failure
	nc.SetReconnectHandler(func(nc *nats.Conn) {
		nc.Publish("system.reset", eventSystemReset)
	})

	// Start listener
	go listener()

	// Wait for interrupt signal
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	<-c
}
