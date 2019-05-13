<p align="center"><a href="https://resgate.io" target="_blank" rel="noopener noreferrer"><img width="100" src="docs/img/resgate-logo.png" alt="Resgate logo"></a></p>


<h2 align="center"><b>Realtime API Gateway</b><br/>Synchronize Your Clients</h2>
</p>

<p align="center">
<a href="http://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License"></a>
<a href="http://goreportcard.com/report/resgateio/resgate"><img src="http://goreportcard.com/badge/github.com/resgateio/resgate" alt="Report Card"></a>
<a href="https://travis-ci.org/resgateio/resgate"><img src="https://travis-ci.org/resgateio/resgate.svg?branch=master" alt="Build Status"></a>
<a href="https://coveralls.io/github/resgateio/resgate?branch=master"><img src="https://coveralls.io/repos/github/resgateio/resgate/badge.svg?branch=master" alt="Coverage"></a>
</p>

<p align="center">Visit <a href="https://resgate.io">Resgate.io</a> for <a href="https://resgate.io/docs/get-started/introduction/">guides</a>, <a href="https://resgate.io/demo/">live demos</a>, and <a href="https://resgate.io/docs/get-started/resources/">resources</a>.</p>

---

Resgate is a [Go](http://golang.org) project implementing a realtime API gateway for the [RES protocol](docs/res-protocol.md) with [NATS server](https://nats.io/about/) as messaging system.

It is a simple server that lets you create REST, real time, and RPC APIs, where all your clients are synchronized seamlessly.

Used for building **new REST APIs** with real-time functionality, or when creating **single page applications** using reactive frameworks such as React, Vue.js, or Modapp.

![Book Collection Animation](docs/img/book-collection-anim.gif)  
*Screen capture from the [Book Collection Example](examples/book-collection/). Try out the [Live demo](https://resgate.io/demo/#book-collection-demo) version yourself.*

## How it works

Resgate handles all API requests from your clients, instead of directly exposing your micro-services (represented by *Node.js* and *Java* below). Clients will connect to Resgate, using either HTTP or WebSocket, to make requests. These requests are sent to the micro-services over NATS server, and Resgate will keep track on which resource each client has requested.

Whenever there is a change to the data, the responsible micro-service sends an event. Resgate will use this event to both update its own cache, and make sure each subscribing client is kept up-to-date.

<p align="center"><img width="480" src="docs/img/res-network.png" alt="RES network diagram"></p>

## Quickstart

### Download
The recommended way to install *Resgate* and *NATS Server* is to download one of the pre-built binaries:
* [Download](https://nats.io/download/nats-io/gnatsd/) and run NATS Server
* [Download](https://github.com/resgateio/resgate/releases/latest) and run Resgate

### Building

If you wish to build your own binaries, first make sure you have:
* [installed Go](https://golang.org/doc/install) and [set your `$GOPATH`](https://golang.org/cmd/go/#hdr-GOPATH_environment_variable)
* added `$GOPATH/bin` (where your binaries are stored) to your `PATH`
* [installed node.js](https://nodejs.org/en/download/) (for the test app)

Install and run [NATS server](https://nats.io/download/nats-io/gnatsd/) and Resgate:
```bash
go get github.com/nats-io/gnatsd
gnatsd
```
```bash
go get github.com/resgateio/resgate
resgate
```

## Examples

While Resgate may be used with any language, the examples are written in Javascript for Node.js.

| Example | Description
| --- | ---
| [Hello World](examples/hello-world/) | Simple service serving a static message.
| [Edit Text](examples/edit-text/) | Text field that can be edited by multiple clients concurrently.
| [Book Collection](examples/book-collection/) | List of book titles & authors that can be edited by many.
| [JWT Authentication](examples/jwt-authentication/) |Showing how JWT tokens can be used for authentication.
| [Password Authentication](examples/password-authentication/) | Showing authentication with user and password credentials.
| [Client Session](examples/client-session/) | Creating client sessions that survive reloads and reconnects.

> **Note**
>
> All examples are complete with both service and client.

## Protocol Specification

For more in depth information on the protocol:

* [RES protocol](docs/res-protocol.md) - Introduction and general terminology
* [RES-Service protocol](docs/res-service-protocol.md) - How to write services
* [RES-Client protocol](docs/res-client-protocol.md) - How to write client libraries, if [ResClient](https://github.com/resgateio/resclient) doesn't fit your needs

## Usage
```
resgate [options]
```
| Option | Description |
|---|---|
| `-n, --nats <url>` | NATS Server URL |
| `-i, --addr <host>` | Bind to HOST address |
| `-p, --port <port>` | Use port for clients |
| `-w, --wspath <path>` | Path to WebSocket |
| `-a, --apipath <path>` | Path to web resources |
| `-r, --reqtimeout <milliseconds>` | Timeout duration for NATS requests |
| `-u, --headauth <method>` | Resource method for header authentication |
| `    --tls` | Enable TLS |
| `    --tlscert <file>` | Server certificate file |
| `    --tlskey <file>` | Private key for server certificate |
| `    --apiencoding <type>` | Encoding for web resources: json, jsonflat |
| `-c, --config <file>` | Configuration file |
| `-h, --help` | Show usage message |


## Configuration
Configuration is a JSON encoded file. If no config file is found at the given path, a new file will be created with default values as follows.

### Properties

```javascript
{
	// URL to the NATS server
	"natsUrl": "nats://127.0.0.1:4222",
	// Timeout in milliseconds for NATS requests
	"requestTimeout": 3000,
	// Bind to HOST IPv4 or IPv6 address
	// Empty string ("") means all IPv4 and IPv6 addresses.
	// Invalid or missing IP address defaults to 0.0.0.0.
	"addr": "0.0.0.0",
	// Port for the http server to listen on.
	// If the port value is missing or 0, standard http(s) port is used.
	"port": 8080,
	// Path for accessing the RES API WebSocket
	"wsPath": "/",
	// Path for accessing web resources
	"apiPath": "/api",
	// Encoding for web resources.
	// Available encodings are:
	// * json - JSON encoding with resource reference meta data
	// * jsonflat - JSON encoding without resource reference meta data
	"apiEncoding": "json",
	// Header authentication resource method for web resources.
	// Prior to accessing the resource, this resource method will be
	// called, allowing an auth service to set a token using
	// information such as the request headers.
	// Missing value or null will disable header authentication.
	// Eg. "authService.headerLogin"
	"headerAuth": null,
	// Flag telling if tls encryption is enabled
	"tls": false,
	// Certificate file path for tls encryption
	"tlsCert": "",
	// Key file path for tls encryption
	"tlsKey": ""
}
```

## Running Resgate

By design, Resgate will exit if it fails to connect to the NATS server, or if it loses the connection.
This is to allow clients to try to reconnect to another Resgate instance and resume from there, and to give Resgate a fresh new start if something went wrong.

A simple bash script can keep it running:

```bash
#!/bin/bash
until ./resgate; do
    echo "Resgate exited with code $?.  Restarting.." >&2
    sleep 2
done
```

## Documentation

Visit [Resgate.io](https://resgate.io) for documentation and resources.

It has guides on [installation](https://resgate.io/docs/get-started/installation/), [configuration](https://resgate.io/docs/get-started/configuration/), [writing services](https://resgate.io/docs/writing-services/01hello-world/), [scaling](https://resgate.io/docs/advanced-topics/scaling/), [queries](https://resgate.io/docs/advanced-topics/query-resources/), and other useful things. It also contains guides for [ResClient](https://resgate.io/docs/writing-clients/resclient/) when working with frameworks such as [React](https://resgate.io/docs/writing-clients/using-react/), [Vue.js](https://resgate.io/docs/writing-clients/using-vuejs/), and [Modapp](https://resgate.io/docs/writing-clients/using-modapp/).

## Support Resgate

Resgate is an MIT-licensed open source project where development is made possible through community support.

If you'd like help out, please consider:

- [Make a one-time donation via PayPal](https://paypal.me/jirenius)
- [Become a backer on Patreon](https://www.patreon.com/jirenius)


## Contribution

Any feedback on the protocol and its implementation is highly appreciated!

If you find any issues with the protocol or the gateway, feel free to [report them](https://github.com/resgateio/resgate/issues/new).

If you have created a service library, a client library, or some other tool or utility, please contact me to have it added to [the list of resources](https://resgate.io/docs/get-started/resources/).
