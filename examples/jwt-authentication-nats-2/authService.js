const jwt = require('jsonwebtoken');
const cookie = require('cookie');
const { connect, StringCodec } = require("nats");

// Json encode/decode helper functions
const json = (function() {
	const sc = new StringCodec();
	return {
		encode: o => sc.encode(JSON.stringify(o)),
		decode: o => JSON.parse(sc.decode(o))
	}
}());

module.exports.authService = async function() {
	// Connect to NATS server
	const nats = await connect("nats://localhost:4222");

	const mySecret = 'shhhhh';
	const jwtCookieName = 'access-token';

	// Auth listener for header login with jwt
	(async (sub) => {
		for await (const m of sub) {
			let { cid, header } = json.decode(m.data);

			// Parse Cookie header
			let cookies = header && header['Cookie'] && cookie.parse(header['Cookie'][0]);

			// Verify we have received the wanted cookie
			if (!cookies || !cookies[jwtCookieName]) {
				m.respond(json.encode({ error: {
					code: 'system.invalidParams',
					message: "Invalid params: missing jwt token"
				}}));
				continue;
			}

			// Get the jwt token from the header
			let jwtToken = cookies[jwtCookieName];

			// Verify the token asynchronously
			try {
				let decoded = jwt.verify(jwtToken, mySecret);

				// Set the decoded token for the client.
				// This will be stored by Resgate, but never sent to client.
				// Resgate will pass the token to the services with any
				// access, call, or auth request.
				nats.publish('conn.' + cid + '.token', json.encode({ token: decoded }));

				// Reply to the request with a successful empty response
				m.respond(json.encode({ result: null }));
			} catch (err) {
				m.respond(json.encode({ error: {
					code: 'system.invalidParams',
					message: "Invalid params: invalid jwt token"
				}}));
			}
		}
	})(nats.subscribe('auth.auth.jwtHeader'));

	// Don't exit until the client closes
	await nats.closed();
}
