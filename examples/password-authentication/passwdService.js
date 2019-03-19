// Connect to NATS server
const nats = require('nats').connect("nats://localhost:4222");

const myPassword = 'secret';

// Auth listener for password login
nats.subscribe('auth.passwd.login', function(req, reply) {
	let { cid, params } = JSON.parse(req);

	// Verify we have received the correct password
	if (params && params.password == myPassword) {
		// Set a token for the client.
		// This will be stored by Resgate, but never sent to client.
		// Resgate will pass the token to the services with any
		// access, call, or auth request.
		nats.publish('conn.' + cid + '.token', JSON.stringify({ token: { foo: "bar" }}));

		// Reply to the request with a successful empty response
		nats.publish(reply, JSON.stringify({ result: null }));
	} else {
		nats.publish(reply, JSON.stringify({ error: {
			code: 'system.invalidParams',
			message: "Invalid params: wrong password"
		}}));
	}
});

// Auth listener for logout
nats.subscribe('auth.passwd.logout', function(req, reply) {
	let { cid } = JSON.parse(req);

	// Set a null token for the client
	nats.publish('conn.' + cid + '.token', JSON.stringify({ token: null }));
	// Reply to the request with a successful empty response
	nats.publish(reply, JSON.stringify({ result: null }));
});
