# Password Authentication Example

This example, written in Javascript (Node.js), shows how authentication can be done using a password. It also shows how access to events are revoked as soon as the client is logged out. The server consists of three files:

**tickerService.js**
* exposes a single resource: `ticker.model`
* updates the model every second
* requires a token to access the resource

**passwdService.js**
* exposes two [authentication methods](../../docs/res-service-protocol.md#auth-request), `passwd.login` and `passwd.logout`
* `login` auth method verifies the *password* parameter and sets a [connection token](docs/res-service-protocol.md#connection-token-event)
* `logout` auth method clears any connection token, by setting it to *null*

**server.js**
* starts the *tickerService.js* and *passwdService.js* micro-services
* serves `/index.html` which is the example client

## Prerequisite

* Have [NATS Server](https://nats-io.github.io/docs/nats_server/installation.html) and [Resgate](https://resgate.io/docs/get-started/installation/) running
* Have [node.js](https://nodejs.org/en/download/) installed

## Install and run

Run the following commands:
```bash
npm install
npm start
```
Open the client
```
http://localhost:8085
```

## Things to try out

**Gain access**  
Log in with the password `secret` to set the client's access token and start seeing the ticking counter.

**Remove access**  
Click on the *Logout* button to clear the client's access token. As the `ticker.model` resource requires an access token, Resgate will force the client to unsubscribe to the resource.

**Regain access**  
Logging in again will allow the client to resume getting updates. There might be a slight delay before the updates start, as *ResClient* will periodically try to resubscribe to resources still being listened to.

> **Note**
>
> This example does not handle disconnects or Resgate restarts.
>
> Look at [Client Session example](../client-session/) or [JWT Authentication example](../jwt-authentication/) to learn more about session handling.

