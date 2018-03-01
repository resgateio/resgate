# RES Protocol Overview

RES (REsource Subscription) is a protocol for creating scaleable client-server APIs consisting of resources served from stateless services connected by a messaging system. Clients subscribe to resources, and call upon their methods, through an API agnostic gateway acting as a bridge between the services and clients.

It is designed for creating modular cloud based applications where new services and functionality can be added without causing disruption to a running production environment.

The protocol consists of two subprotocols:

* RES-Client - the protocol used by the client
* RES-Service - the protocol used by the services

## Features

**Stateless**  
No client context is held by the services between requests. Each request contains all the information necessary to service the request. Session state is instead held by the client and the gateways.

**Live data**  
All resources subscribed by the clients are live. The gateways will keep state on what resources are currently being used by any of its connected clients, making sure their data are kept updated.

**Scalable**  
Multiple gateways may be used to handle massive amounts of clients.

**Resilient**  
In case of a lost connection or a gateway failure, the client can reconnect to any other gateway to update its stale data and resume operations.

**Secure**  
The client uses WebSockets as transport layer, allowing for TLS/SSL encryption. The protocol does not define the authentication methods used, but authentication tokens are stored on the gateways and not in the client.

**Access control**  
Tokens may at any time be revoked or replaces. If a token is revoked/replaced, all subscriptions will be paused for reauthorization with the new token, and resources which no longer are authorized will be unsubscribed directly.

**Hot-swapping**  
Services can be added or removed to the API without any disruption or any configuration changes. Simply just connect/disconnect the service to the messaging system.

**Caching**  
All resources are cachable by the gateways, taking load off the services.

**Resource queries**  
The protocol supports resource queries where partial or filtered resources are requested, such as for searches, filters, or pagination. Live updates are also available for query resources.

**Web resources**  
All resources may be accessed using ordinary web (http) requests. The same goes for all resource method calls.

**Simple**  
The protocol is made to be simple. Simple to create services. Simple to access resources from the clients.

# Getting started

The best place to start is to [install resgate](https://github.com/jirenius/resgateway), a RES gateway implementation written in [Go](http://golang.org), using [NATS server](https://nats.io/documentation/server/gnatsd-intro/) as messaging system. The resgate README contains information on how to get started, and how to make a basic *Hello world* example service and client.

# Introduction

## Resources

RES protocol is built around a concept of resources: model resources and collection resources. A model is a single object that may have simple properties and methods. A collection is an ordered list of models.  

### Resource ID
Each resource (model or collection) is identified by a unique *resource ID* string. A *resource ID* consist of a *resource name* and an optional *query*.  
MUST be a string.

**resource name**  
A dot-separated string where the first part should be the name of the service owning the resource. The following parts should describe and identify the specific resource.

**query**  
The query is separated from the resource name by a question mark (`?`). The format of the query is not enforced, but it is recommended to use URI queries in case the resources are to be accessed through web requests. May be omitted.  
May be omitted, and the question mark separator MUST then also be omitted.

**Examples**

* `userService.users` - A collection representing a list of users
* `userService.user.42` - A model representing a user
* `userService.user.42.roles`- A collection of roles held by a user
* `messageService.messages?start=0&limit=25` - A collection representing the first 25 messages of a list
* `userService.users?q=Jane` - A collection of users with the name Jane

## Model

A model is a resource represented by a single JSON object. Models contain key/value pairs where the value may be anything except an object or array. For nested objects or arrays, a model may hold the resource ID's to other resources, but must not hold the actual resources.

## Collection

A collection is an array of model resources. A collection must not contain other collection resources or primitives.



# RES-Service protocol

A RES service communicates over a publish-subscribe messaging system such as a NATS server, using JSON (RFC 4627) as message encoding.

## Requests
Services listens to requests published by the gateways on on behalf of the clients. A request consists of a [subject](#request-subject) (also called *topic*) and a [payload](#request-payload).

### Request subject

A request subject is a string identiying the request. It has the following structure:

`<type>.<resourceName>.<method>`

* type - the request type. May be either `access`, `get`, `call`, or `auth`.
* resourceName - the resource ID for the resource, without the query part.
* method - the request method. Only used for `call` or `auth` type requests.

### Request payload

The payload is a JSON object containing the request parameters.  
If no parameters are provided, the payload may be empty (0 bytes).

The content of the payload depends on the subject type.

## Response
When a request is received by a service, it should send a response as a JSON object with following members:

**result**  
Is REQUIRED on success.  
Will be ignored on error.  
The value is determined by the request subject.  

**error**  
Is REQUIRED on error.  
MUST NOT exist on success.  
The value MUST be an error object as defined in the [Error object](#error-object) section.  

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

Code                    | Message            | Meaning
----------------------- | ------------------ | ----------------------------------------
`system.notFound`       | Not found          | The resource was not found
`system.invalidParams`  | Invalid parameters | Invalid parameters in method call
`system.internalError`  | Internal error     | Internal error
`system.methodNotFound` | Method not found   | Resource method not found
`system.accessDenied`   | Access denied      | Access to a resource or method is denied

---

## Access request

**Topic**  
`access.<resourceName>`

Access requests are sent to determine what kind of access a client has to a resource. The service handling the access request may be different from the service providing the resource.  
The request payload has the following parameters:

**cid**  
Connection ID of the client connection requesting connection.  
The value is generated by the gateway for every new connection.  
MUST be a string.

**token**  
Authorization token that MAY be omitted if the connection has no token.  
The value is defined by the service issuing the token.

**query**  
Query parameters without the question mark separator.  
MUST be omitted if the resource has no query.  
MUST be a string.

### Result

**get**  
Flag if the client has access to get (read) the resource.  
May be omitted if client has no get access.  
MUST be a boolean

**call**  
A comma separated list of methods that the client can call. Eg. `"set,foo,bar"`.  
May be omitted if client is not allowed to call any methods.  
Value may be a single asterisk character (`"*"`) if client is allowed to call any method.  

### Error

Any error response will be treated as if the client has no access to the resource.  
A `system.notFound` error MAY be sent if the resourceID doesn't exist, or if the *query* parameter is provided for a non-query resource.

## Get request

**Topic**  
`get.<resourceName>`

Get requests are sent to get the JSON representation of a resource.  
The request payload has the following parameter:

**query**  
Query parameters without the question mark separator.  
MUST be omitted if the resource has no query.  
MUST be a string.

### Result

**model**  
Object containing the models data.  
The object properties is defined by the service.  
MUST NOT exist if *collection* is provided.

**collection**  
An ordered array containing the resource IDs of the collection model.  
MUST NOT exist if *model* is provided.  
MUST be an array of strings.

### Error

Any error response will be treated as if the resource is currently unavailable.  
A `system.notFound` error SHOULD be sent if the resource ID doesn't exist, or if the *query* parameter is provided for a non-query resource.

## Call request

**Topic**  
`call.<resourceName>.<method>`

Call requests are sent to invoke a method on the resource.  
The request payload has the following parameter:

**cid**  
Connection ID of the client connection requesting connection.  
The value is generated by the gateway for every new connection.  
MUST be a string.

**token**  
Authorization token that MAY be omitted if the connection has no token.  
The value is defined by the service issuing the token.

**query**  
Query parameters without the question mark separator.  
MUST be omitted if the resource has no query.  
MUST be a string.

### Result

The result is defined by the service, and may be null.

### Error

Any error response indicates that the method call failed and had no effect.  
A `system.notFound` error SHOULD be sent if the resource ID does not exist, or if the *query* parameter is provided for a non-query resource.  
A `system.methodNotFound` error SHOULD be sent if the method does not exist.  
A `system.invalidParams` error SHOULD be sent if any required parameter is missing, or any parameter is invalid.

## Auth request

**Topic**  
`auth.<resourceName>.<method>`

Auth requests are sent to invoke an authentication method on the resource.  
It behaves in a similar way as the [call request](#call-request), but does not require access. Auth requests also includes additional parameters.  
The request payload has the following parameter:

**cid**  
Connection ID of the client connection requesting connection.  
The value is generated by the gateway for every new connection.  
MUST be a string.

**token**  
Authorization token that MAY be omitted if the connection has no token.  
The value is defined by the service issuing the token.

**query**  
Query parameters without the question mark separator.  
MUST be omitted if the resource has no query.  
MUST be a string.

**header**  
HTTP headers used on client connection. May be omitted.  
MUST be a key/value object, where the key is the canonical format of the MIME header, and the value is an array of strings associated with the key.

**host**
The host on which the URL is sought by the client. Per RFC 2616, this is either the value of the "Host" header or the host name given in the URL itself.  
May be omitted.  
MUST be a string.

**remoteAddr**
The network address of the client that sent the request.  
The format is not specified, and it may be omitted.  
MUST be a string.

**uri**
The unmodified Request-URI of the Request-Line (RFC 2616, Section 5.1) as sent by the client when connecting to the gateway.  
May be omitted.  
MUST be a string.

### Result

The result is defined by the service, and may be null.  
A successful request MAY trigger a [connection token event](#connection-token-event). If a token event is triggered, it MUST be sent prior to sending the response.

### Error

Any error response indicates that the authentication failed and had no effect. A failed authentication SHOULD NOT trigger a [connection token event](#connection-token-event).  

A `system.notFound` error SHOULD be sent if the resource ID does not exist, or if the *query* parameter is provided for a non-query resource.  
A `system.methodNotFound` error SHOULD be sent if the method does not exist.  
A `system.invalidParams` error SHOULD be sent if any required parameter is missing, or any parameter is invalid.

---

## Events

Services may send events to the messaging system that may be listen to by any gateway or service. Events are not persisted in the system, and any event that was not subscribed to when it was sent will not be retrievable. There are three types of events:

* resource events - affects a single resource
* connection events - affects a client connection
* system events - affects the system

## Resource events

Resource events for a given resource MUST be sent by the same service that handles the resource's requests, to ensure all responses and events are sent in chronological order.

Events and responses from two different resources may be sent in non-chronological order, even if the resources are handled by the same service, or if a resource is a model of another collection resource.

When a resource is modified, the service MUST send the defined events that describe the changes made. If a service fails to do so, maybe due to a program crash or a service loading stale data on restart, it MUST send a [System reset event](#system-reset-event) for the affected resources.

### Model change event

**Topic**  
`event.<resourceName>.change`

Change events are sent when a [model](#model)'s properties has been changed.  
The event payload is a key/value object describing the property that was change, and the new value.  
Unchanged properties SHOULD NOT be included.

### Custom event

**Topic**  
`event.<resourceName>.<eventName>`

Custom events are used to send information that does not affect the state of the resource.  
A custom event MUST NOT use a resource event name already defined by this document.  
Payload is defined by the service, and will be passed to the client without alteration.