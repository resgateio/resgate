# The RES-Service Protocol Specification

## Table of contents
- [Introduction](#introduction)
  * [Features](#features)
- [Getting started](#getting-started)
- [Resources](#resources)
  * [Resource IDs](#resource-ids)
  * [Models](#models)
  * [Collections](#collections)
- [Values](#values)
- [Requests](#requests)
  * [Request subject](#request-subject)
  * [Request payload](#request-payload)
  * [Response](#response)
  * [Error object](#error-object)
  * [Pre-defined errors](#pre-defined-errors)
  * [Pre-response](#pre-response)
- [Request types](#request-types)
  * [Access request](#access-request)
  * [Get request](#get-request)
  * [Call request](#call-request)
  * [Auth request](#auth-request)
- [Events](#events)
- [Resource events](#resource-events)
  * [Model change event](#model-change-event)
  * [Collection add event](#collection-add-event)
  * [Collection remove event](#collection-remove-event)
  * [Reaccess event](#reaccess-event)
  * [Custom event](#custom-event)
- [Connection events](#connection-events)
  * [Connection token event](#connection-token-event)
- [System events](#system-events)
  * [System reset event](#system-reset-event)
- [Query resources](#query-resources)
  * [Query event](#query-event)
  * [Query request](#query-request)


# Introduction

RES (REsource Subscription) is a protocol for creating scaleable client-server APIs consisting of resources served from stateless services connected by a messaging system. Clients subscribe to resources, and call upon their methods, through an API agnostic gateway acting as a bridge between the services and clients.

It is designed for creating modular cloud based applications where new services and functionality can be added without causing disruption to a running production environment.

The protocol consists of two subprotocols:

* RES-Client - the protocol used by the clients
* RES-Service - the protocol used by the services

## Features

**Stateless**  
No client context is held by the services between requests. Each request contains all the information necessary to service the request. Session state is instead held by the client and the gateway.

**Live data**  
All resources subscribed by the clients are live. The gateways will keep state on what resources are currently being used by any of its connected clients, making sure their data is updated.

**Scalable**  
Multiple gateways may be connecteded to the same messaging system to handle large amounts of clients. Multiple clusters of messaging systems, gateways, and services, may be used for further scaling.

**Resilient**  
In case of a lost connection or a gateway failure, the client can reconnect to any other gateway to resynchronize its stale data and resume operations. In case of a service failure, the gateways can resynchronize its data once the service is available again.

**Secure**  
The client uses WebSockets as transport layer, allowing for TLS/SSL encryption. Any access token issued by a service is stored on the gateway and not in the client.

**Access control**  
Access tokens may at any time be revoked or replaces. If a token is revoked/replaced, all subscriptions will be paused for reauthorization with the new token, and resources which no longer are authorized will be unsubscribed without delay.

**Hot-adding**  
Services can be added or removed to the API without any disruption or any configuration changes. Simply just connect/disconnect the service to the messaging system.

**Caching**  
All resources are cachable by the gateways, taking load off the services.

**Resource queries**  
The protocol supports resource queries where partial or filtered resources are requested, such as for searches, filters, or pagination. Just like any other resource, query resources are also live.

**Web resources**  
All resources may be accessed using ordinary web (http) requests, in a RESTful manner. The same goes for all resource method calls.

**Simple**  
The protocol is made to be simple. Simple to create services. Simple to access resources from the clients.


# Getting started

The best place to start is to [install resgate](https://github.com/jirenius/resgateway), a RES gateway implementation written in [Go](http://golang.org), using [NATS server](https://nats.io/documentation/server/gnatsd-intro/) as messaging system. The resgate README contains information on how to get started, and how to make a basic *Hello world* example service and client.


# Resources

RES protocol is built around a concept of resources: *model* resources and *collection* resources. A model is a single object with properties and values. A collection is an ordered list of values.  

## Resource IDs
Each resource (model or collection) is identified by a unique *resource ID* string, also called *rid* for short. A *resource ID* consist of a *resource name* and an optional *query*.  
MUST be a string.

**resource name**  
Resource names are case-sensitive and must be non-empty alphanumeric strings with no embedded whitespace, and part-delimited using the dot character (`.`).  
The first part SHOULD be the name of the service owning the resource. The following parts describes and identifies the specific resource.

**query**  
The query is separated from the resource name by a question mark (`?`). The format of the query is not enforced, but it is recommended to use URI queries in case the resources are to be accessed through web requests.  
May be omitted. If omitted, then the question mark separator MUST also be omitted.

**Examples**

* `userService.users` - A collection representing a list of users
* `userService.user.42` - A model representing a user
* `userService.user.42.roles` - A collection of roles held by a user
* `messageService.messages?start=0&limit=25` - A collection representing the first 25 messages of a list
* `userService.users?q=Jane` - A collection of users with the name Jane

## Models

A model is an unordered set of named properties and [values](#values) represented by a JSON object.

**Example**
```
{
    "id": 42,
    "name": "Jane Doe",
    "roles": { rid: "userService.user.42.roles" }
}
```

## Collections

A collection is an ordered list of [values](#values) represented by a JSON array.

**Example**
```
[ "admin", "tester", "developer" ]
```

# Values

A value is either a *primitive* or a *resource reference*. Primitives are either a JSON `string`, `number`, `true`, `false`, or `null` value. A resource reference is a JSON object with the following parameter:

**rid**  
Resource ID of the referenced resource.  
MUST be a [resource ID](#resource-ids).


# Requests
Services listens to requests published by the gateways on on behalf of the clients. A request consists of a [subject](#request-subject) (also called *topic*) and a [payload](#request-payload).

## Request subject

A request subject is a string identiying the type of request, which resource it is made for, and in case of `call` and `auth` requests, which method is called.   
It has the following structure:

`<type>.<resourceName>.<method>`

* type - the request type. May be either `access`, `get`, `call`, or `auth`.
* resourceName - the resource name of the [resource ID](#resource-ids).
* method - the request method. Only used for `call` or `auth` type requests.

## Request payload

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
MUST be omitted on success.  
The value MUST be an error object as defined in the [Error object](#error-object) section.  

## Error object

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

## Pre-defined errors

There are a number of predefined errors.

Code                    | Message            | Meaning
----------------------- | ------------------ | ----------------------------------------
`system.notFound`       | Not found          | The resource was not found
`system.invalidParams`  | Invalid parameters | Invalid parameters in method call
`system.internalError`  | Internal error     | Internal error
`system.methodNotFound` | Method not found   | Resource method not found
`system.accessDenied`   | Access denied      | Access to a resource or method is denied
`system.timeout`        | Request timeout    | Request timed out

## Pre-response

When a service recieves a request, and before a response is sent, the service may send a pre-response. A service should only send a pre-response when the time it takes to handle the request might exceed the default timeout limit of the requester.  
The pre-response is a UTF-8 encoded key:"value" string without any leading white space.  
It should contain the following key:

**timeout**  
Sets the request timeout. The value is the new timeout in milliseconds calculated from when the requester receives the pre-response. The requester should honor this timeout.  
Example payload (15 second timeout):  
`timeout:"15000"`

---


# Request types

## Access request

**Subject**  
`access.<resourceName>`

Access requests are sent to determine what kind of access a client has to a resource. The service handling the access request may be different from the service providing the resource.  
The request payload has the following parameters:

**cid**  
Connection ID of the client connection requesting connection.  
The value is generated by the gateway for every new connection.  
MUST be a string.

**token**  
Access token that MAY be omitted if the connection has no token.  
The value is defined by the service issuing the token.

**query**  
Query part of the [resource ID](#resource-ids) without the question mark separator.  
MUST be omitted if the resource ID has no query.  
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
A `system.notFound` error MAY be sent if the resource ID doesn't exist.

## Get request

**Subject**  
`get.<resourceName>`

Get requests are sent to get the JSON representation of a resource.  
The request payload may have the following parameter:

**query**  
Query part of the [resource ID](#resource-ids) without the question mark separator.  
MUST be omitted if the resource ID has no query.  
MUST be a string.

### Result

**model**  
Object containing the models data.  
The object properties is defined by the service.  
MUST be omitted if *collection* is provided.

**collection**  
An ordered array containing the [values](#values) of the collection.  
MUST be omitted if *model* is provided.  
MUST be an array of strings.

**query**  
Normalized query without the question mark separator.  
Different queries (eg. `a=1&b=2` and `b=2&a=1`) that results in the same [query resource](#query-resources) should have the same normalized query (eg. `a=1&b=2`). The normalized query will be used by the gateway in [query requests](#query-request), and in get requests triggered by a [system reset event](#system-reset-event).  
MUST be omitted if the request had no *query* parameter.  
MUST be omitted if the resource is not a [query resource](#query-resources).  
MUST NOT be omitted if the resource is a [query resource](#query-resources).  
MUST be a string.

### Error

Any error response will be treated as if the resource is currently unavailable.  
A `system.notFound` error SHOULD be sent if the resource ID doesn't exist.

## Call request

**Subject**  
`call.<resourceName>.<method>`

Call requests are sent to invoke a method on the resource.  
The request payload has the following parameter:

**cid**  
Connection ID of the client connection requesting connection.  
The value is generated by the gateway for every new connection.  
MUST be a string.

**token**  
Access token that MAY be omitted if the connection has no token.  
The value is defined by the service issuing the token.

**query**  
Query part of the [resource ID](#resource-ids) without the question mark separator.  
MUST be omitted if the resource ID has no query.  
MUST be a string.

### Result

The result is defined by the service, and may be null.

### Error

Any error response indicates that the method call failed and had no effect.  
A `system.notFound` error SHOULD be sent if the resource ID does not exist.  
A `system.methodNotFound` error SHOULD be sent if the method does not exist.  
A `system.invalidParams` error SHOULD be sent if any required parameter is missing, or any parameter is invalid.

## Auth request

**Subject**  
`auth.<resourceName>.<method>`

Auth requests are sent to invoke an authentication method on the resource.  
It behaves in a similar way as the [call request](#call-request), but does not require access. Auth requests also includes additional parameters.  
The request payload has the following parameter:

**cid**  
Connection ID of the client connection requesting connection.  
The value is generated by the gateway for every new connection.  
MUST be a string.

**token**  
Access token that MAY be omitted if the connection has no token.  
The value is defined by the service issuing the token.

**query**  
Query part of the [resource ID](#resource-ids) without the question mark separator.  
MUST be omitted if the resource ID has no query.  
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

A `system.notFound` error SHOULD be sent if the resource ID does not exist.  
A `system.methodNotFound` error SHOULD be sent if the method does not exist.  
A `system.invalidParams` error SHOULD be sent if any required parameter is missing, or any parameter is invalid.

---


# Events

Services may send events to the messaging system that may be received by any gateway or service. Events are not persisted in the system, and any event that was not subscribed to when it was sent will not be retrievable. There are three types of events:

* resource events - affects a single resource
* connection events - affects a client connection
* system events - affects the system


# Resource events

**Subject**
`event.<resourceName>.<eventName>`

Resource events are sent for a given resource, and MUST be sent by the same service that handles the resource's [get](#get-request) and [call](#call-request) requests. This is to ensure all responses and events for a resource are sent in chronological order.

Events and responses from different resources may be sent in non-chronological order in respect to one another, even if the resources are handled by the same service, or if the resources has references to each other.

When a resource is modified, the service MUST send the defined events that describe the changes made. If a service fails to do so, maybe due to a program crash or a service loading stale data on restart, it MUST send a [System reset event](#system-reset-event) for the affected resources.

## Model change event

**Subject**  
`event.<resourceName>.change`

Change events are sent when a [model](#model)'s properties has been changed.  
The event payload is a key/value object describing the properties that was change. Each property should have a new [value](#values) or a [delete action](#delete-action).  
Unchanged properties SHOULD NOT be included.  
MUST NOT be sent on [collections](#collection).

**Example payload**
```
  {
    "myProperty": "New value",
    "unusedProperty": { "action": "delete" }
  }
```

### Delete action
A delete action is a JSON object used when a property has been deleted from a model. It has the following signature:  
```
{ "action": "delete" }
```

## Collection add event

**Subject**  
`event.<resourceName>.add`

Add events are sent when a value is added to a [collection](#collection).  
MUST NOT be sent on [models](#model).  
The event payload has the following parameters:

**value**  
[Value](#values) that is added.

**idx**  
Zero-based index number of where the value is inserted.  
MUST be a number that is zero or greater and less than the length of the collection.

## Collection remove event

**Subject**  
`event.<resourceName>.remove`

Remove events are sent when a value is removed from a [collection](#collection).  
MUST NOT be sent on [models](#model).  
The event payload has the following parameter:

**idx**  
Zero-based index number of where the value was prior to removal.  
MUST be a number that is zero or greater and less than the length of the collection prior to removal.

## Reaccess event

**Subject**  
`event.<resourceName>.reaccess`

Reaccess events are sent when a resource's access permissions has changed. It will invalidate any previous access response recieved for the resource.  
The event has no payload.

## Custom event

**Subject**  
`event.<resourceName>.<eventName>`

Custom events are used to send information that does not affect the state of the resource.  
A custom event MUST NOT use a resource event name already defined by this document.  
Payload is defined by the service, and will be passed to the client without alteration.


# Connection events

Connection events are sent for specific connection ID's (cid), and are listened to by the gateways. These events allow for the services to control the state of the connections.  

## Connection token event

**Subject**  
`conn.<cid>.token`

Sets the connection's access token, discarding any previously set token.  
A change of token will invalidate any previous access response recieved using the old token.  
The event payload has the following parameter:

**token**  
Access token.
A `null` token clears any previously set token.


# System events

System events are used to send information having a system wide effect.

## System reset event

**Subject**  
`system.reset`

Signals that some resources are no longer to be considered up to date, or that previous access requests may no longer be valid.  
A service MUST send a system reset event if it no longer can guarantee that it has sent the defined [resource events](#resource-events) that describe the changes made to its resources, or if access to the resources might have changed without the service having sent the appropriate [reaccess events](#reaccess-event). This may be due to a service crashing between persisting a change and sending the event describing the change, or by restarting a service that only persisted its resource or access state in memory.  
The event payload has the following parameters:

**resources**  
JSON array of [resource name patterns](#resource-name-pattern).  
Any service or gateway subscribing to a matching resource should send a new [get request](#get-request) to get an up-to-date version.  
May be omitted.

**access**  
JSON array of [resource name patterns](#resource-name-pattern).  
Any gateway with clients subscribing to a matching resource should send new [access requests](#access-request) for each client subscription.  
May be omitted.

### Resource name pattern
A resource name pattern is a string used for matching resource names.  
The pattern may use the following wild cards:  
* The asterisk (`*`) matches any part at any level of the resource name.  
Eg. `userService.user.*.roles` - Pattern that matches the roles collection of all users.
* The greater than symbol (`>`) matches one or more parts at the end of a resource name, and must be the last part.  
Eg. `messageService.>` - Pattern that matches all resources owned by *messageService*.  




# Query resources

A query resource is a resource where its model properties or collection values may vary based on the query. It is used to request partial or filtered resources, such as for searches, sorting, or pagination.

## Query event

**Subject**  
`event.<resourceName>.query`

Query events are sent when a [query resource](#query-resources) might have been modified. This happens when any of the data that the query resource is based upon is modified.

Prior to sending the event, the service must generate a temporary inbox subject and subscribe to it. The inbox subject is sent as part of the event payload, and any subscriber receiving the event should send a [query request](#query-request) on that subject for each query they subscribe to on the given resource.

The event payload has the following parameter:

**subject**  
A subject string to which a (#query-request) may be sent.  
MUST be a string.

## Query request

**Subject**  
Subject sent in a [query event](#query-event).

Query requests are sent in response to a [query event](#query-event). The service should respond with a list of events to be applied to the query resource. These events must be based on the state of the underlaying data at the time when the query event was sent. This requires the service to keep track of the changes made to a query resource's underlaying data for as long as the temporary request subject is being subscribed to.  
The request payload has the following parameters:

**query**  
Normalized query recieved in the response to the get request for the query resource.  
MUST be a string.

### Result

**events**  
An array of events for the query resource.  
MUST be an array of [event query objects](#event-query-object)  
May be omitted if there are no events.

### Event query object

An event query object has the following members:

**event**  
Event name as described in [resource events](#resource-events).  
MUST be a string.

**data**  
Payload data as described in [resource events](#resource-events).  
May be omitted if the event requires no payload.

