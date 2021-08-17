const { connect, StringCodec } = require("nats");

// Json encode/decode helper functions
const json = (function() {
	const sc = new StringCodec();
	return {
		encode: o => sc.encode(JSON.stringify(o)),
		decode: o => JSON.parse(sc.decode(o))
	}
}());

module.exports.exampleService = async function() {
	// Connect to NATS server
	const nats = await connect("nats://localhost:4222");

	const model = { message: "Hello, secure World!" };

	// Get listener. Reply with the json encoded model
	(async (sub) => {
		for await (const m of sub) {
			m.respond(json.encode({ result: { model: model }}));
		}
	})(nats.subscribe('get.example.model'));

	// Access listener. Only get access if the token is set with { foo:"bar" }
	(async (sub) => {
		for await (const m of sub) {
			let { token } = json.decode(m.data);
			if (token && token.foo === 'bar') {
				m.respond(json.encode({ result: { get: true, call: "set" }}));
			} else {
				m.respond(json.encode({ result: { get: false }}));
			}
		}
	})(nats.subscribe('access.example.model'));

	// Set listener for updating the model.message property
	(async (sub) => {
		for await (const m of sub) {
			let r = json.decode(m.data);
			let p = r.params || {};
			// Check if the message property was changed
			if (typeof p.message === 'string' && p.message !== model.message) {
				model.message = p.message;
				// The model is updated. Send a change event.
				nats.publish('event.example.model.change', json.encode({
					values: { message: p.message }
				}));
			}
			// Reply success by sending an empty result
			m.respond(json.encode({ result: null }));
		}
	})(nats.subscribe('call.example.model.set'));

	// System resets tells Resgate that the service has been (re)started.
	// Resgate will then update any cached resource from example
	nats.publish('system.reset', json.encode({ resources: [ 'example.>' ] }));

	// Don't exit until the client closes
	await nats.closed();
}
