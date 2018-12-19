# JWT Authentication example

This example, written in javascript (node.js), shows how jwt tokens can be used to authenticate the client. The server consists of three files:

**exampleService.js**
* It exposes a single resource, `exampleService.myModel`.
* It requires a token to access the resource and its `set` method.

**authService.js**
* It exposes an [authentication method](../../docs/res-service-protocol.md#auth-request), `authService.jwtHeader`.
* The `jwtHeader` auth method verifies the jwt token and sets it as [connection token](docs/res-service-protocol.md#connection-token-event).

**server.js**
* It starts the *exampleService.js* and *authService.js* micro-services
* It serves `/index.html` which is the example client
* It serves `/login` which sets the jwt token cookie
* It serves `/logout` which clears the jwt token cookie

## Running Resgate

To access the resource with HTTP GET requests, Resgate needs to be configured with the header authentication method to use. Start Resgate with the following flag:

```bash
resgate --headauth authService.jwtHeader
```
## Prerequisite

* Have [NATS Server](https://nats.io/download/nats-io/gnatsd/) and [Resgate](https://github.com/jirenius/resgate) (with the *headauth* flag) running
* Have [node.js](https://nodejs.org/en/download/) installed

## Install and run

Run the following commands:
```bash
npm install
npm start
```
Open the client
```
http://localhost:8083
```

## Things to try out

**Access denied**  
When loading the client without a token set, the client should not be able to access the model, instead showing the *Access denied* error message.

**Gain access**  
Go to `http://localhost:8083/login`, to set the jwt token, and then return back to the client. The editable input box should now show with the model's message.

**Remove access**  
Go to `http://localhost:8083/logout`, to clear the jwt token, and then return back to the client. The message *Access denied* should show again.

**Accessing via REST**  
Try accessing the model as web resource (REST), both with the jwt token set or cleared.

## Web resources

### Get model
```
GET http://localhost:8080/api/exampleService/myModel
```

### Update model
```
POST http://localhost:8080/api/exampleService/myModel/set
```
*Body*  
```
{ "message": "Updated through HTTP" }
```