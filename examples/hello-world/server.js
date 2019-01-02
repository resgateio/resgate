// Connect to NATS server
const nats = require('nats').connect("nats://localhost:4222");

let mymodel = { message: "Hello world" };

// Get listener. Reply with the json encoded model
nats.subscribe('get.example.mymodel', function(req, reply) {
  nats.publish(reply, JSON.stringify({ result: { model: mymodel }}));
});

// Access listener. Everyone gets read access and access to call the set-method
nats.subscribe('access.example.mymodel', (req, reply) => {
	nats.publish(reply, JSON.stringify({ result: { get: true, call: "set" }}));
});

// Set listener for updating the mymodel.message property
nats.subscribe('call.example.mymodel.set', (req, reply) => {
	let r = JSON.parse(req);
	let p = r.params || {};
	// Check if the message property was changed
	if (typeof p.message === 'string' && p.message !== mymodel.message) {
		mymodel.message = p.message;
		// The model is updated. Send a change event.
		nats.publish('event.example.mymodel.change', JSON.stringify({ message: p.message }));
	}
	// Reply success by sending an empty result
	nats.publish(reply, JSON.stringify({result: null}));
});

// System resets tells Resgate that the service has been (re)started.
// Resgate will then update any cached resource from example
nats.publish('system.reset', JSON.stringify({ resources: [ 'example.>' ]}));


// Run a simple webserver to serve the client.
// This is only for the purpose of making the example easier to run
const connect = require('connect');
const serveStatic = require('serve-static');
connect().use(serveStatic(__dirname)).listen(8081, function(){
    console.log('Client available at http://localhost:8081');
});
