const jwt = require('jsonwebtoken');
const cookie = require('cookie');
// Connect to NATS server
const nats = require('nats').connect("nats://localhost:4222");

const mySecret = 'shhhhh';
const jwtCookieName = 'access-token';

// Auth listener for header login with jwt
nats.subscribe('auth.auth.jwtHeader', function(req, reply) {
	let { cid, header } = JSON.parse(req);

	// Parse Cookie header
	let cookies = header && header['Cookie'] && cookie.parse(header['Cookie'][0]);

	// Verify we have received the wanted cookie
	if (!cookies || !cookies[jwtCookieName]) {
		nats.publish(reply, JSON.stringify({ error: {
			code: 'system.invalidParams',
			message: "Invalid params: missing jwt token"
		}}));
		return;
	}

	// Get the jwt token from the header
	let jwtToken = cookies[jwtCookieName];

	// Verify the token asynchronously
	jwt.verify(jwtToken, mySecret, function(err, decoded) {
		if (err) {
			nats.publish(reply, JSON.stringify({ error: {
				code: 'system.invalidParams',
				message: "Invalid params: invalid jwt token"
			}}));
			return;
		}

		// Set the decoded token for the client.
		// This will be stored by Resgate, but never sent to client.
		// Resgate will pass the token to the services with any
		// access, call, or auth request.
		nats.publish('conn.' + cid + '.token', JSON.stringify({ token: decoded }));

		// Reply to the request with a successful empty response
		nats.publish(reply, JSON.stringify({ result: null }));
	});
});
