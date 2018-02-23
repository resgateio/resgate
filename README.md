# resgate - A RES gateway
A [Go](http://golang.org) project implementing the RES protocol.

## Quickstart

If you just want to start using resgate, and you have:
* [installed Go](https://golang.org/doc/install) and [set your `$GOPATH`](https://golang.org/cmd/go/#GOPATH_environment_variable)
* Added `$GOPATH/bin` (where your binaries ends up) to your `PATH`
* [installed NATS server](https://nats.io/download/nats-io/gnatsd/) and set it to listen on port 4222

Install and run resgate:

```
go get github.com/jirenius/resgate
resgate
```

## Hello world example

A simple example of a service and client application. For more in depth examples, see the [RES protocol documentation](https://github.com/jirenius/resgate/blob/master/resprotocol.md).

### Service (Node.js)

There is currently no public library for the RES-Service protocol, but because of the simplicity of the protocol, no library is required.

Create an empty folder and install the *nats* client:

```
npm install nats
```

Create file `service.js` :

```javascript
const nats = require('nats').connect("nats://localhost:4222");

let myModel = {message: "Hello world"};

// Access listener. Everyone gets read access and access to call the set-method
nats.subscribe('access.exampleService.myModel', (request, replyTo, subject) => {
	nats.publish(replyTo, JSON.stringify({result: {get: true, call: "set"}}));
});

// Get listener. Reply with the json encoded model
nats.subscribe('get.exampleService.myModel', (request, replyTo, subject) => {
	nats.publish(replyTo, JSON.stringify({result: {model: myModel}}));
});

// Set listener for updating the myModel.message property
nats.subscribe('call.exampleService.myModel.set', (request, replyTo, subject) => {
	let req = JSON.parse(request);
	let p = req.params || {};
	// Check if the message property was changed
	if (typeof p.message === 'string' && p.message !== myModel.message) {
		myModel.message = p.message;
		nats.publish('event.exampleService.myModel.change', JSON.stringify({data: {message: p.message}}));
	}
	// Reply success by sending an empty result
	nats.publish(replyTo, JSON.stringify({result: null}));
});
```

Start the service:

```
node service.js
```

### Client

Javascript client.  
Copy the code to [requirebin.com](http://requirebin.com/) and try it out from there.  
Try running it in two separate tabs!

```javascript
let ResClient = require('resclient').default;
let eventBus = require('modapp/eventBus').default;

const client = new ResClient(eventBus, 'ws://localhost:8181/ws');

client.getResource('exampleService.myModel').then(model => {
	// Create an input element
	let input = document.createElement('input');
	input.value = model.message;
	document.body.appendChild(input);

	// Call set to update the remote model
	input.addEventListener('input', () => {
		model.set({message: input.value});
	});

	// Listen for model change events.
	// The model will be unsubscribed after calling model.off
	model.on('change', () => {
		input.value = model.message;
	});
});
```

### Web  resource

Resources can be retrieved using an ordinary HTTP GET request:

```
http://localhost:8080/api/exampleService/myModel
```

## Usage
```
resgate [options]
```
#### Options
- `-conf=<file path>` File path to configuration-file (default "config.json")

## Configuration
Configuration is a JSON encoded file. If no config file is found, a new file will be created with default values as follows.

### Properties

```javascript
{
	// URL to the NATS server
	"natsUrl": "nats://127.0.0.1:4222",
	// Port for the http server to listen on.
	// If the port value is missing or 0, standard http(s) port is used.
	"port": 8080,
	// Flag telling if tls encryption is used
	"tls": false,
	// Certificate file path for tls encryption
	"certFile": "/etc/ssl/certs/ssl-cert-snakeoil.pem",
	// Key file path for tls encryption
	"keyFile": "/etc/ssl/private/ssl-cert-snakeoil.key",
	// Path for accessing web resources
	"apiPath": "/api/",
	// Header authentication resource method for web resources.
	// Missing value or null will disable header authentication.
	// Eg. "authService.headerLogin"
	"headerAuth": null,
	// Timeout in seconds for message queue requests
	"requestTimeout": 5
}
```

## Contributing

The RES protocol and resgate is still under development, but is currently at a state where the protocol is starting to settle.

While it is not recommended as of yet to use the gateway in a production environment, testing it out and giving feedback on the protocol and its implementation is highly appreciated!

If you find any issues with the protocol or the gateway, feel free to [report them](https://github.com/jirenius/resgate/issues/new) as an Issue.
