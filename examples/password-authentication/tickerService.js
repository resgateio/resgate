// Connect to NATS server
const nats = require('nats').connect("nats://localhost:4222");

let model = { seconds: 0 };

// Get listener. Reply with the json encoded model
nats.subscribe('get.ticker.model', function(req, reply) {
	nats.publish(reply, JSON.stringify({ result: { model: model }}));
});

// Access listener. Only get access if the token is { foo: "bar" }
nats.subscribe('access.ticker.model', (req, reply) => {
	let { token } = JSON.parse(req);
	if (token && token.foo === 'bar') {
		nats.publish(reply, JSON.stringify({ result: { get: true }}));
	} else {
		nats.publish(reply, JSON.stringify({ result: { get: false }}));
	}
});

// Recursive counter that updates the model and sends a change event every second
let count = function() {
	setTimeout(() => {
		model.seconds++;
		nats.publish('event.ticker.model.change', JSON.stringify({
			values: { seconds: model.seconds }
		}));
		count();
	}, 1000);
};
count();

// System resets tells Resgate that the service has been (re)started.
// Resgate will then update any cached resource from ticker
nats.publish('system.reset', JSON.stringify({ resources: [ 'ticker.>' ] }));
