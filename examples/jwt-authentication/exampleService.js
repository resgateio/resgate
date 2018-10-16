// Connect to NATS server
const nats = require('nats').connect("nats://localhost:4222");

let myModel = { message: "Hello secure world" };

// Get listener. Reply with the json encoded model
nats.subscribe('get.exampleService.myModel', function(req, reply) {
  nats.publish(reply, JSON.stringify({ result: { model: myModel }}));
});

// Access listener. Only get access if the token is set with { foo:"bar" }
nats.subscribe('access.exampleService.myModel', (req, reply) => {
	let { token } = JSON.parse(req);
	if (token && token.foo === 'bar') {
		nats.publish(reply, JSON.stringify({ result: { get: true, call: "set" }}));
	} else {
		nats.publish(reply, JSON.stringify({ result: { get: false }}));
	}
});

// Set listener for updating the myModel.message property
nats.subscribe('call.exampleService.myModel.set', (req, reply) => {
	let r = JSON.parse(req);
	let p = r.params || {};
	// Check if the message property was changed
	if (typeof p.message === 'string' && p.message !== myModel.message) {
		myModel.message = p.message;
		// The model is updated. Send a change event.
		nats.publish('event.exampleService.myModel.change', JSON.stringify({ message: p.message }));
	}
	// Reply success by sending an empty result
	nats.publish(reply, JSON.stringify({result: null}));
});

// System resets tells Resgate that the service has been (re)started.
// Resgate will then update any cached resource from exampleService
nats.publish('system.reset', JSON.stringify({ resources: [ 'exampleService.>' ]}));
