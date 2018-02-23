# RES Protocol Overview

RES (REsource Subscription) is a protocol for creating scaleable client-server APIs consisting of resources served from stateless services connected by a message system. Clients subscribe to resources, and call upon their methods, through an API agnostic gateway acting as a bridge between the services and clients.

It is designed for creating modular cloud based applications where new services and functionality can be added without causing disruption to a running production environment.

The protocol consists of two subprotocols:

* RES-C (RES-Client)
* RES-S (RES-Service)

## Features

**Stateless**  
No client context is held by the services between requests. Each request from contains all the information necessary to service the request. Session state is instead held by the client and the gateway.

**Live data**  
All resources subscribed by the clients are live. The gateways will keep state on what resources are currently being used by any of its connected clients, making sure their data are kept updated.

**Scalable**  
Multiple gateways may be used to handle massive amounts of clients.

**Resilient**  
In case of a lost connection or a gateway failure, the client can reconnect to any other gateway to update its stale data and resume operations.

**Secure**  
The client uses WebSockets as transport layer, allowing for TLS/SSL encryption. The protocol enable all kinds of authentication methods, but authentication tokens are stored on the gateway and not in the client.

**Access control**  
Tokens may at any time be revoked or replaces. If a token is revoked/replaced, all subscriptions will be paused for reauthorization with the new token, and resources which no longer are authorized will be unsubscribed directly.

**Hot-swapping**  
Services can be added or removed to the API without any disruption or any configuration changes. Simply just connect/disconnect the service to the message queue.

**Caching**  
The gateways may cache all resources, taking load off the services.

**Resource queries**  
The protocol supports resource queries where partial or filtered resources are requested, such as for searches, filters, or pagination. Such query resources may also be live.

**Web resources**  
All resources may be accessed using ordinary web (http) requests. The same goes for all resource method calls.

**Simple**  
The protocol is made to be simple. Simple to create services serving resources. Simple to access them from the clients.

# Getting started

The best place to start is to [install RESGateway](https://github.com/jirenius/resgateway), a RES implementation written in [Go](http://golang.org), using [NATS server](https://nats.io/documentation/server/gnatsd-intro/) as message queue. The README contains information on how to get started, and how to make a basic *Hello world* example service.

# Introduction

## Resources

RES protocol is built around a concept of resources: model resources and collection resources. A model is a single object that may have simple properties and methods. A collection is an ordered list of models.
Each resource (model or collection) is identified by a unique *resource ID*. A *resource ID* is a dot-separated string where the first part should be the name of the service owning the resource. The following parts should describe and identify the specific resource:

**Examples**

* `userService.users` - A collection representing a list of users
* `userService.user.42` - A model representing a user
* `userService.user.42.roles`- A collection of roles held by a user

### Query resources

In addition, a resource ID may have a query part, separated from the resource name by a question mark (?). The query format is not enforced, but it is recommended to use URI queries in case the resources are to be accessed through HTTP requests:

**Examples**

* `messageService.messages?start=0&limit=25` - A collection representing the first 25 messages of a list
* `userService.users?q=Jane` - A collection of users with the name Jane

More information can be found in the Resource Queries section.

# RES-Service protocol

A RES service communicates over a topic based publish-subscribe message system such as a NATS server, using JSON (RFC 4627) as message encoding.

## Requests
Services listens to requests published by the gateways on behalf of the clients. The payload will be an object containing the request parameters, or empty if no parameters are provided.

Example below for a call to kick a user from a chat room:

**Topic**  
`call.chatroomService.room.12.kick`

**Payload**  
```
{
	userId: 42
}
```

## Response

When a request is received by a service, it should send a response as a JSON object with following members:

**result**  
Is REQUIRED on success.  
Will be ignored on error.  
The value is determined by the request topic.  

**error**  
Is REQUIRED on error.  
MUST NOT exist on success.  
The value MUST be an error object as defined in the Error object section.  

### Error object

On error, the error member contains a value that is an object with the following members:

**code**  
A dot-separated string identifying the error.  
Custom errors SHOULD begin with the service name.  
MUST be a string.

**message**  
A simple error sentence describing the error.  
MUST be a string.  

**data**  
Additional data that may be omitted.  
The value is defined by the service.  
It can be used to hold values for replacing placeholders in the message.  

### Pre-defined errors

There are a number of predefined errors.

Code | Message | Meaning
--- | --- | ---
`system.notFound` | Not found | The resource was not found
`system.invalidParams` | Invalid parameters | Invalid parameters in method call
`system.internalError` | Internal error | Internal error
`system.methodNotFound` | Method not found | Resource method not found
`system.accessDenied` | Access denied | Access to a resource or method is denied

Success response
```javascript
{
	"result": {
		"resourceId": ""
	}
}
```
Where `<object>` is the the successful result data.

On error, the response payload must be an object with the following pattern:
```javascript
{
	"error": {
		"code": <string>,
		"message": <string>,
		"data": <object>
	}
}
```
Where `code` is a str


#### Access request
Access requests are sent by the gateway when a client wish to get a resource. The services responds whether the client should have access to the resource or not.

**Topic:**  
`access.<resourceName>`

**Request:**  
```javascript
{
	// Token held by the gateways
	// client connection, or null if
	// no token is held.
	// Token format is not defined by
	// the protocol.
	"token": null,
	// Connection ID is a unique ID
	// assigned by the gateway to a
	// client connection.
	"cid": "",
	// The query part of the resource ID
	// without the separating question mark (?).
	// Will be omitted if the resource ID
	// has no query.
	"query": "start=0&limit=25"
}
```

**Success response:**
```javascript
{
	"result": {
		"read": true
	}
}
```

#### `get.<resourceName>`
#### `call.<resourceName>.<action>`
#### `auth.<resourceName>.<action>`