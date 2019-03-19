// Connect to NATS server
const nats = require('nats').connect("nats://localhost:4222");

let model = { message: "Hello, secure World!" };

// Get listener. Reply with the json encoded model
nats.subscribe('get.example.model', function(req, reply) {
	nats.publish(reply, JSON.stringify({ result: { model: model }}));
});

// Access listener. Only get access if the token is set with { foo:"bar" }
nats.subscribe('access.example.model', (req, reply) => {
	let { token } = JSON.parse(req);
	if (token && token.foo === 'bar') {
		nats.publish(reply, JSON.stringify({ result: { get: true, call: "set" }}));
	} else {
		nats.publish(reply, JSON.stringify({ result: { get: false }}));
	}
});

// Set listener for updating the model.message property
nats.subscribe('call.example.model.set', (req, reply) => {
	let r = JSON.parse(req);
	let p = r.params || {};
	// Check if the message property was changed
	if (typeof p.message === 'string' && p.message !== model.message) {
		model.message = p.message;
		// The model is updated. Send a change event.
		nats.publish('event.example.model.change', JSON.stringify({ message: p.message }));
	}
	// Reply success by sending an empty result
	nats.publish(reply, JSON.stringify({ result: null }));
});

// System resets tells Resgate that the service has been (re)started.
// Resgate will then update any cached resource from example
nats.publish('system.reset', JSON.stringify({ resources: [ 'example.>' ] }));
