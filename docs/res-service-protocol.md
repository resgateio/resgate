# The RES-Service Protocol Specification v1.1

## Table of contents
- [Introduction](#introduction)
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
- [Pre-defined call methods](#pre-defined-call-methods)
  * [Set call request](#set-call-request)
  * [New call request](#new-call-request)
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

This document uses the definition of [resource](res-protocol.md#resources), [model](res-protocol.md#models), [collection](res-protocol.md#collections), [value](res-protocol.md#values), [messaging system](res-protocol.md#messaging-system), [service](res-protocol.md#services), [client](res-protocol.md#clients), and [gateway](res-protocol.md#gateways) as described in the [RES Protocol specification](res-protocol.md).

The RES-Service protocol is used in communication between the services and the gateways.

# Requests
Services listens to requests published by the gateways and services over the messaging system. A request consists of a [subject](#request-subject) (also called *topic*) and a [payload](#request-payload).

## Request subject

A request subject is a string identifying the type of request, which resource it is made for, and in case of `call` and `auth` requests, which method is called.   
It has the following structure:

`<type>.<resourceName>.<method>`

* type - the request type. May be either `access`, `get`, `call`, or `auth`.
* resourceName - the resource name of the [resource ID](res-protocol.md#resource-ids).
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

When a service receives a request, and before a response is sent, the service may send a pre-response. A service should only send a pre-response when the time it takes to handle the request might exceed the default timeout limit of the requester.  
The pre-response is a UTF-8 encoded key:"value" string without any leading white space.  
It should contain the following key:

**timeout**  
Sets the request timeout. The value is the new timeout in milliseconds calculated from when the requester receives the pre-response. The requester should honor this timeout.  
Example payload (15 second timeout):  
`timeout:"15000"`


# Request types

## Access request

**Subject**  
`access.<resourceName>`

Access requests are sent to determine what kind of access a client has to a resource. The service handling the access request may be different from the service providing the resource.  
The request payload has the following parameters:

**cid**  
[Connection ID](res-protocol.md#connection-ids) of the client connection requesting connection.  
The value is generated by the gateway for every new connection.  
MUST be a string.

**token**  
Access token that MAY be omitted if the connection has no token.  
The value is defined by the service issuing the token.

**query**  
Query part of the [resource ID](res-protocol.md#resource-ids) without the question mark separator.  
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
Query part of the [resource ID](res-protocol.md#resource-ids) without the question mark separator.  
MUST be omitted if the resource ID has no query.  
MUST be a string.

### Result

**model**  
An object containing the named properties and [values](res-protocol.md#values) of the model.  
MUST be omitted if *collection* is provided.

**collection**  
An ordered array containing the [values](res-protocol.md#values) of the collection.  
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
[Connection ID](res-protocol.md#connection-ids) of the client connection requesting connection.  
The value is generated by the gateway for every new connection.  
MUST be a string.

**token**  
Access token that MAY be omitted if the connection has no token.  
The value is defined by the service issuing the token.

**query**  
Query part of the [resource ID](res-protocol.md#resource-ids) without the question mark separator.  
MUST be omitted if the resource ID has no query.  
MUST be a string.

**params**  
Method parameters as defined by the service or by the appropriate [pre-defined call method](#pre-defined-call-methods).  
MAY be omitted.

### Result

The result is defined by the service, or by the appropriate [pre-defined call method](#pre-defined-call-methods). The result may be null.

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
[Connection ID](res-protocol.md#connection-ids) of the client connection requesting connection.  
The value is generated by the gateway for every new connection.  
MUST be a string.

**token**  
Access token that MAY be omitted if the connection has no token.  
The value is defined by the service issuing the token.

**query**  
Query part of the [resource ID](res-protocol.md#resource-ids) without the question mark separator.  
MUST be omitted if the resource ID has no query.  
MUST be a string.

**params**  
Method parameters as defined by the service.  
MAY be omitted.

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


# Pre-defined call methods

There are a set of [call request](#call-request) methods are predefined. A service may implement any of these methods as long as it conforms to this specification.  
The parameters described for each call method refers to the `params` parameter of the call request.

## Set call request

**Subject**  
`call.<resourceName>.set`

A set request is used to update or delete a model's properties.

**Parameters**  
The parameters SHOULD be a key/value object describing the properties to be changed. Each property should have a new [value](res-protocol.md#values) or a [delete action](#delete-action). Unchanged properties SHOULD NOT be included.  
If any of the model properties are changed, a [model change event](#model-change-event) MUST be sent prior to sending the response.  
MUST NOT be sent on [collections](res-protocol.md#collections).

## New call request

**Subject**  
`call.<resourceName>.new`

A new request is used to create new resources.

**Params**  

For new models, the parameters SHOULD be an object containing the named properties and [values](res-protocol.md#values) of the model.  
For new collections, the parameters SHOULD be an ordered array containing the [values](res-protocol.md#values) of the collection.

**Result**  
MUST be a [resource reference](res-protocol.md#values) to the new resource.


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

Change events are sent when a [model](res-protocol.md#models)'s properties has been changed.  
MUST NOT be sent on [collections](res-protocol.md#collections).  
The event payload has the following parameter:

**values**  
A key/value object describing the properties that was changed.  
Each property should have a new [value](res-protocol.md#values) or a [delete action](#delete-action).  
Unchanged properties SHOULD NOT be included.  

**Example payload**
```json
{
  "values": {
    "myProperty": "New value",
    "unusedProperty": { "action": "delete" }
  }
}
```

### Delete action
A delete action is a JSON object used when a property has been deleted from a model. It has the following signature:  
```json
{ "action": "delete" }
```

## Collection add event

**Subject**  
`event.<resourceName>.add`

Add events are sent when a value is added to a [collection](res-protocol.md#collections).  
MUST NOT be sent on [models](res-protocol.md#models).  
The event payload has the following parameters:

**value**  
[Value](res-protocol.md#values) that is added.

**idx**  
Zero-based index number of where the value is inserted.  
MUST be a number that is zero or greater and less than the length of the collection.

**Example payload**
```json
{
  "value": "foo",
  "idx": 2
}
```

## Collection remove event

**Subject**  
`event.<resourceName>.remove`

Remove events are sent when a value is removed from a [collection](res-protocol.md#collections).  
MUST NOT be sent on [models](res-protocol.md#models).  
The event payload has the following parameter:

**idx**  
Zero-based index number of where the value was prior to removal.  
MUST be a number that is zero or greater and less than the length of the collection prior to removal.

**Example payload**
```json
{ "idx": 2 }
```

## Reaccess event

**Subject**  
`event.<resourceName>.reaccess`

Reaccess events are sent when a resource's access permissions has changed. It will invalidate any previous access response received for the resource.  
The event has no payload.

## Custom event

**Subject**  
`event.<resourceName>.<eventName>`

Custom events are used to send information that does not affect the state of the resource.  
The event name is case-sensitive and MUST be a non-empty alphanumeric string with no embedded whitespace. It MUST NOT be any of the following reserved event names:  
`change`, `delete`, `add`, `remove`, `patch`, `reaccess` or `unsubscribe`.


Payload is defined by the service, and will be passed to the client without alteration.


# Connection events

Connection events are sent for specific [connection ID's (cid)](#res-protocol.md#connection-ids), and are listened to by the gateways. These events allow for the services to control the state of the connections.  

## Connection token event

**Subject**  
`conn.<cid>.token`

Sets the connection's access token, discarding any previously set token.  
A change of token will invalidate any previous access response received using the old token.  
The event payload has the following parameter:

**token**  
Access token.
A `null` token clears any previously set token.

**Example payload**
```json
{
  "token": {
    "username": "foo",
    "role": "admin",
  }
}
```


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

**Example payload**
```json
{
  "resources": [ "userService.users", "userService.user.*" ],
  "access": [ "userService.>" ]
}
```

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

**Example payload**
```json
{
  "subject": "_REQUEST_SUBJECT_12345678"
}
```

## Query request

**Subject**  
Subject received from the [query event](#query-event).

Query requests are sent in response to a [query event](#query-event). The service should respond with a list of events to be applied to the query resource. These events must be based on the state of the underlaying data at the time when the query event was sent. This requires the service to keep track of the changes made to a query resource's underlaying data for as long as the temporary request subject is being subscribed to.  
The request payload has the following parameters:

**query**  
Normalized query received in the response to the get request for the query resource.  
MUST be a string.

**Example payload**
```json
{
  "query": "limit=25&start=0"
}
```

### Result

**events**  
An array of events for the query resource.  
MUST be an array of [event query objects](#event-query-object)  
May be omitted if there are no events.

**Example result payload**
```json
{
  "events": [
    { "event": "remove", "data": { "idx": 24 }},
    { "event": "add", "data": { "value": "foo", "idx": 0 }},
  ]
}
```

### Event query object

An event query object has the following members:

**event**  
Event name as described in [resource events](#resource-events).  
MUST be a string.

**data**  
Payload data as described in [resource events](#resource-events).  
May be omitted if the event requires no payload.

