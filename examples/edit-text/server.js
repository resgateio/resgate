// Connect to NATS server
const nats = require('nats').connect("nats://localhost:4222");

let shared = { message: "Edit me!" };

// Get listener. Reply with the json encoded model
nats.subscribe('get.example.shared', function(req, reply) {
	nats.publish(reply, JSON.stringify({ result: { model: shared }}));
});

// Access listener. Everyone gets read access and access to call the set-method
nats.subscribe('access.example.shared', (req, reply) => {
	nats.publish(reply, JSON.stringify({ result: { get: true, call: "set" }}));
});

// Set listener for updating the shared.message property
nats.subscribe('call.example.shared.set', (req, reply) => {
	let r = JSON.parse(req);
	let p = r.params || {};
	// Check if the message property was changed
	if (typeof p.message === 'string' && p.message !== shared.message) {
		shared.message = p.message;
		// The model is updated. Send a change event.
		nats.publish('event.example.shared.change', JSON.stringify({
			values: { message: p.message }
		}));
	}
	// Reply success by sending an empty result
	nats.publish(reply, JSON.stringify({ result: null }));
});

// System resets tells Resgate that the service has been (re)started.
// Resgate will then update any cached resource from example
nats.publish('system.reset', JSON.stringify({ resources: [ 'example.>' ] }));

// Create a simple webserver to serve the index.html client.
const express = require('express');
let app = express();
app.use('/', express.static(__dirname));
app.listen(8082, () => {
	console.log('Client available at http://localhost:8082');
});
