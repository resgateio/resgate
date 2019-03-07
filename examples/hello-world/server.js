const nats = require('nats').connect('nats://localhost:4222');

nats.subscribe('get.example.model', (req, reply) => {
	nats.publish(reply, JSON.stringify({ result: { model: { message: "Hello, World!" }}}));
});

nats.subscribe('access.example.model', (req, reply) => {
	nats.publish(reply, JSON.stringify({ result: { get: true }}));
});
