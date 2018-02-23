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

### Service

Node.js demo service. Requires *nats* client:

```
npm install nats
```

```javascript
// Node.js demo service
const nats = require('nats').connect("nats://localhost:4222");

let myModel = {message: "Hello world"};

// Access listener. Everyone gets access
nats.subscribe('access.exampleService.>', (request, replyTo, subject) => {
	nats.publish(replyTo, JSON.stringify({result: {read: true}}));
});

// Get listener
nats.subscribe('get.exampleService.myModel', (request, replyTo, subject) => {
	nats.publish(replyTo, JSON.stringify({result: {model: myModel}}));
});
```

### Client

Javascript client. Requires *resclient* and *modapp*:
```
npm install resclient
npm install modapp
```

```javascript
import ResClient from 'resclient';
import eventBus from 'modapp/eventBus';

const client = new ResClient(eventBus, 'ws://localhost:8080/ws');

client.getResource('exampleService.myModel').then(model => {
	alert(model.message);

	// Listen to changes for 5 seconds, eventually unsubscribing
	let onChange = () => alert("Updated message: " + model.message);
	model.on('change', onChange);
	setTimeout(() => model.off('change', onChange), 5000);
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
