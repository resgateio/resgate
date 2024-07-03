# The RES-Client Protocol Specification

*Version: [1.2.3](res-protocol-semver.md)*

## Table of contents
- [Introduction](#introduction)
- [Subscriptions](#subscriptions)
  * [Direct subscription](#direct-subscription)
  * [Indirect subscription](#indirect-subscription)
  * [Resource set](#resource-set)
- [Connection ID tag](#connection-id-tag)
- [Client JSONRPC](#client-jsonrpc)
  * [Error object](#error-object)
  * [Pre-defined errors](#pre-defined-errors)
- [Requests](#requests)
  * [Request method](#request-method)
- [Request types](#request-types)
  * [Version request](#version-request)
  * [Subscribe request](#subscribe-request)
  * [Unsubscribe request](#unsubscribe-request)
  * [Get request](#get-request)
  * [Call request](#call-request)
  * [Auth request](#auth-request)
  * [New request](#new-request)
- [Events](#events)
  * [Event object](#event-object)
  * [Model change event](#model-change-event)
  * [Collection add event](#collection-add-event)
  * [Collection remove event](#collection-remove-event)
  * [Custom event](#custom-event)
  * [Unsubscribe event](#unsubscribe-event)

# Introduction

This document uses the definition of [resource](res-protocol.md#resources), [model](res-protocol.md#models), [collection](res-protocol.md#collections), [value](res-protocol.md#values), [service](res-protocol.md#services), [client](res-protocol.md#clients), and [gateway](res-protocol.md#gateways) as described in the [RES Protocol specification](res-protocol.md).

The RES-Client protocol is used in communication between the client and the gateway.

# Subscriptions

A core concept in the RES-Client protocol is the subscriptions. A client may subscribe to resources by making [subscribe requests](#subscribe-request) with the unique [resource ID](res-protocol.md#resource-ids), or by getting a resource response on a [call request](#call-request) or [auth request](#auth-request).

A resource may be subscribed to [directly](#direct-subscription) or [indirectly](#indirect-subscription). Any reference in this document to *subscription* or a resource being *subscribed* to, should be interpreted as both *direct* and *indirect* subscriptions, unless specified.

The client will receive [events](#events) on anything that happens on a subscribed resource. A subscription lasts as long as the resource has direct or indirect subscriptions, or when the connection to the gateway is closed.

## Direct subscription
The resource that is subscribed to with a [subscribe request](#subscribe-request), or returned as a resource response to a [call request](#call-request) or [auth request](#auth-request) will be considered *directly subscribed*.

It is possible to have multiple direct subscriptions on a resource. It will be considered directly subscribed until the same number of subscriptions are matched using one ore more [unsubscribe requests](#unsubscribe-request).

## Indirect subscription
A resource that is referred to with a non-soft [resource reference](res-protocol.md#resource-references) by a [directly subscribed](#direct-subscription) resource, or by an indirectly subscribed resource, will be considered *indirectly subscribed*. Cyclic references where none of the resources are directly subscribed will not be considered subscribed.


## Resource set
Any request or event resulting in new subscriptions will contain a set of resources that contains any subscribed resource previously not subscribed by the client.

The set is grouped by type, `models`, `collections`, and `errors`. Each group is represented by a key/value object where the key is the [resource ID](res-protocol.md#resource-ids), and the value is the [model](res-protocol.md#models), [collection](res-protocol.md#collections), or [error](#error-object).

**Example**
```json
{
  "models": {
    "messageService.message.1": {
      "id": 1,
      "msg": "foo"
    },
    "messageService.message.2": {
      "id": 2,
      "msg": "bar"
    }
  },
  "collections": {
    "messageService.messages": [
        { "rid": "messageService.message.1" },
        { "rid": "messageService.message.2" },
        { "rid": "messageService.message.3" }
    ]
  },
  "errors": {
    "messageService.message.3": {
      "code": "system.notFound",
      "message": "Not found"
    }
  }
}
```

# Connection ID tag

A connection ID tag is a specific string, "`{cid}`" (without the quotation marks), that may be used as part of a [resource ID](res-protocol.md#resource-ids).

The gateway will replace the tag with the client's actual [connection ID](res-protocol.md#connection-ids) before passing any request further to the services.

Any [event](#events) on a resource containing a connection ID tag will be sent to the client with the tag, never with the actual connection ID.

**Example**

`authService.user.{cid}` - Model representing the user currently logged in on the connection.

# Client JSONRPC
The client RPC protocol is a variant of the [JSONRPC 2.0 specification](http://www.jsonrpc.org/specification), with the RES gateway acting as server. It differs in the following:

* WebSockets SHOULD be used for transport
* Request object SHOULD NOT include the `jsonrpc` property
* Request object's `method` property MUST be a valid [request method](#request-method)
* Response object does NOT contain `jsonrpc` property
* Response object does NOT require the `result` property
* Error object's MUST be a valid [error object](#error-object), where the `code` property MUST be a string.
* Batch requests are NOT supported
* Client notifications are NOT supported
* Server may send [event objects](#event-object)

## Error object

An error object has following members:

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

Code | Message | Meaning
--- | --- | ---
`system.notFound` | Not found | The resource was not found
`system.invalidParams` | Invalid parameters | Invalid parameters in method call
`system.invalidQuery` | Invalid query | Invalid query or query parameters
`system.internalError` | Internal error | Internal error
`system.methodNotFound` | Method not found | Resource method not found
`system.accessDenied` | Access denied | Access to a resource or method is denied
`system.timeout` | Request timeout | Request timed out
`system.noSubscription` | No subscription | The resource has no direct subscription
`system.invalidRequest` | Invalid request | Invalid request
`system.unsupportedProtocol` | Unsupported protocol | RES protocol version is not supported


# Requests

Clients sends request to the gateway and the gateway responds with a request result or a request error. The request's `method` property contains the [request method](#request-method), and the `params` property contains the parameters defined by the services for requests of type `call` and `auth`.

## Request method

A request method is a string identifying the type of request, which resource it is made for, and in case of `call` and `auth` requests which resource method is called.   
A request method has the following structure:

`<type>.<resourceID>.<resourceMethod>`

* type - the request type. May be either `version`, `subscribe`, `unsubscribe`, `get`, `call`, `auth`, or `new`.
* resourceID - the [resource ID](res-protocol.md#resource-ids). Not used for `version` type requests.
* resourceMethod - the resource method. Only used for `call` or `auth` type requests.

Trailing separating dots (`.`) must not be included.

**Examples**  

* `version` - Version request
* `subscribe.userService.users` - Subscribe request of a collection of users
* `call.userService.user.42.set` - Call request to set properties on a user
* `new.userService.users` - New request to create a new user
* `auth.authService.login` - Authentication request to login


# Request types

## Version request

**method**  
`version`

Version requests are sent by the client to tell which RES protocol version it supports, and to get information on what protocol version the gateway supports.

The request SHOULD be the first request sent by the client after an established connection.

If not sent, or if the **protocol** property is omitted in the request, the gateway SHOULD assume version v1.1.x.

### Parameters
The request parameters are optional.  
It not omitted, the parameters object SHOULD have the following property:

**protocol**  
The RES protocol version supported by the client.  
MUST be a string in the format `"[MAJOR].[MINOR].[PATCH]"`. Eg. `"1.2.3"`.

### Result

**protocol**  
The RES protocol version supported by the gateway.  
MUST be a string in the format `"[MAJOR].[MINOR].[PATCH]"`. Eg. `"1.2.3"`.

### Error

A `system.unsupportedProtocol` error response will be sent if the gateway cannot support the client protocol version.  
A `system.invalidRequest` error response will be sent if the gateway only supports RES Protocol v1.1.1 or below, prior to the introduction of the [version request](#version-request).

## Subscribe request

**method**  
`subscribe.<resourceID>`

Subscribe requests are sent by the client to [subscribe](#subscriptions) to a resource.  
The request has no parameters.

### Result

**models**  
[Resource set](#resource-set) models.  
May be omitted if no new models were subscribed.

**collections**  
[Resource set](#resource-set) collections.  
May be omitted if no new collections were subscribed.

**errors**  
[Resource set](#resource-set) errors.  
May be omitted if no subscribed resources encountered errors.

### Error

An error response will be sent if the resource couldn't be subscribed to.  
Any [resource reference](res-protocol.md#resource-references) that fails will not lead to an error response, but the error will be added to the [resource set](#resource-set) errors.

## Unsubscribe request

Unsubscribe requests are sent by the client to unsubscribe to previous [direct subscriptions](#direct-subscription).

The resource will only be considered unsubscribed when there are no more [direct](#direct-subscription) or [indirect](#indirect-subscription) subscriptions.

If the **count** property is omitted in the request, the value of 1 is assumed.

**method**  
`unsubscribe.<resourceID>`

### Parameters
The request parameters are optional.  
It not omitted, the parameters object SHOULD have the following property:

**count**  
The number of direct subscriptions to unsubscribe to.  
MUST be a number greater than 0.

### Result
The result has no payload.

### Error

An error response with code `system.noSubscription` will be sent if the resource has no direct subscription, or if *count* exceeds the number of direct subscriptions. If so, the number of direct subscriptions will be unaffected.


## Get request

Get requests are sent by the client to get a resource without making a subscription.

**method**  
`get.<resourceID>`

### Parameters
The request has no parameters.

### Result

**models**  
[Resource set](#resource-set) models.  
May be omitted if no new models were retrieved.

**collections**  
[Resource set](#resource-set) collections.  
May be omitted if no new collections were retrieved.

**errors**  
[Resource set](#resource-set) errors.  
May be omitted if no retrieved resources encountered errors.

### Error

An error response will be sent if the resource couldn't be retrieved.  
Any [resource reference](res-protocol.md#resource-references) that fails will not lead to an error response, but the error will be added to the [resource set](#resource-set) errors.


## Call request

Call requests are sent by the client to invoke a method on the resource. The response may either contain a result payload or a resource ID.

In case of a resource ID, the resource is considered [directly subscribed](#direct-subscription).

**method**  
`call.<resourceID>.<resourceMethod>`

### Parameters
The request parameters are defined by the service.

### Result
The result is an object with the following members:

**payload**  
Result payload as defined by the service.  
MUST be omitted if **rid** is set.  
MUST NOT be omitted if **rid** is not set.

**rid**  
Resource ID of subscribed resource.  
MUST be omitted if **payload** is set.

**models**  
[Resource set](#resource-set) models.  
May be omitted if no new models were subscribed.  
MUST be omitted if **payload** is set.

**collections**  
[Resource set](#resource-set) collections.  
May be omitted if no new collections were subscribed.  
MUST be omitted if **payload** is set.

**errors**  
[Resource set](#resource-set) errors.  
May be omitted if no subscribed resources encountered errors.  
MUST be omitted if **payload** is set.

### Error
An error response will be sent if the method couldn't be called, or if the method was called, but an error was encountered.

## Auth request

Auth requests are sent by the client to authenticate the client connection. The response may either contain a result payload or a resource ID.

In case of a resource ID, the resource is considered [directly subscribed](#direct-subscription).

**method**  
`auth.<resourceID>.<resourceMethod>`

### Parameters
The request parameters are defined by the service.

### Result
The result is an object with the following members:

**payload**  
Result payload as defined by the service.  
MUST be omitted if **rid** is set.  
MUST NOT be omitted if **rid** is not set.

**rid**  
Resource ID of subscribed resource.  
MUST be omitted if **payload** is set.

**models**  
[Resource set](#resource-set) models.  
May be omitted if no new models were subscribed.  
MUST be omitted if **payload** is set.

**collections**  
[Resource set](#resource-set) collections.  
May be omitted if no new collections were subscribed.  
MUST be omitted if **payload** is set.

### Error
An error response will be sent if the method couldn't be called, or if the authentication failed.


## New request
DEPRECATED: Use [call request](#call-request) instead.

New requests are sent by the client to create a new resource.

The newly created resource will get a direct subscription, and will be sent to the client in the [resource set](#resource-set).

**method**  
`new.<resourceID>`

### Parameters
The request parameters are defined by the service.  
For new models, the parameters are usually an object containing the named properties and [values](res-protocol.md#values) of the model.  
For new collections, the parameters are usually an ordered array containing the [values](res-protocol.md#values) of the collection.

### Result
**rid**  
Resource ID of the created resource.

**models**  
[Resource set](#resource-set) models.  
May be omitted if no new models were subscribed.

**collections**  
[Resource set](#resource-set) collections.  
May be omitted if no new collections were subscribed.

**errors**  
[Resource set](#resource-set) errors.  
May be omitted if no subscribed resources encountered errors.

### Error
An error response will be sent if the resource could not be created, or if an error was encountered retrieving the newly created resource.

# Events

The gateway sends [event objects](#event-object) to describe events on resources currently subscribed to by the client.

RES protocol does not guarantee that all events sent by the service will reach the client. It only guarantees that the events sent on a resource will describe the modifications required to get the resource into the same state as on the service.

## Event object
An event object has the following members:

**event**  
Identiying which resource the event occurred on, and the type of event.  
It has the following structure:

`<resourceID>.<eventName>`

MUST be a string.

**data**  
Event data. The payload is defined by the event type.

## Model change event

Change events are sent when a [model](res-protocol.md#models)'s properties has been changed.  
Will result in new [indirect subscriptions](#indirect-subscription) if changed properties contain [resource references](res-protocol.md#resource-references) previously not subscribed.  
Change events are only sent on [models](res-protocol.md#models).

**event**  
`<resourceID>.change`

**data**  
[Change event object](#change-event-object).

### Change event object
The change event object has the following parameters:

**values**
A key/value object describing the properties that was changed. Each property contains the new [value](res-protocol.md#values) or a [delete action](#delete-action).  
Unchanged properties may be included and SHOULD be ignored.

**models**  
[Resource set](#resource-set) models.  
May be omitted if no new models were subscribed.

**collections**  
[Resource set](#resource-set) collections.  
May be omitted if no new collections were subscribed.

**errors**  
[Resource set](#resource-set) errors.  
May be omitted if no subscribed resources encountered errors.

### Example
```json
{
  "event": "myService.myModel.change",
  "data": {
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
Add events are sent when a value is added to a [collection](res-protocol.md#collections).  
Will result in one or more new [indirect subscriptions](#indirect-subscription) if added value is a [resource references](res-protocol.md#resource-references) previously not subscribed.  
Add events are only sent on [collections](res-protocol.md#collections).

**event**  
`<resourceID>.add`

**data**  
[Add event object](#add-event-object).

### Add event object
The add event object has the following parameters:

**idx**  
Zero-based index number of where the value is inserted.

**value**  
[Value](res-protocol.md#values) that is added.

**models**  
[Resource set](#resource-set) models.  
May be omitted if no new models were subscribed.

**collections**  
[Resource set](#resource-set) collections.  
May be omitted if no new collections were subscribed.

**errors**  
[Resource set](#resource-set) errors.  
May be omitted if no subscribed resources encountered errors.

### Example
```json
{
  "event": "userService.users.add",
  "data": {
    "idx": 12,
    "value": { "rid": "userService.user.42" },
    "models": {
      "userService.user.42": {
        "id": 42,
        "firstName": "Jane",
        "lastName": "Doe"
      }
    }
  }
}
```

## Collection remove event
Remove events are sent when a value is removed from a [collection](res-protocol.md#collections).  
Remove events are only sent on [collections](res-protocol.md#collections).

**event**  
`<resourceID>.remove`

**data**  
[Remove event object](#remove-event-object).

### Remove event object
The remove event object has the following parameter:

**idx**  
Zero-based index number of the value being removed.

### Example
```json
{
  "event": "userService.users.remove",
  "data": {
    "idx": 12,
  }
}
```

## Custom event

Custom events are defined by the services, and may have any event name except the following:  
`add`, `change`, `create`, `delete`, `patch`, `reset`, `reaccess`, `remove` or `unsubscribe`.  
Custom events MUST NOT be used to change the state of the resource.

**event**  
`<resourceID>.<eventName>`

**data**  
Payload is defined by the service.

## Unsubscribe event

Unsubscribe events are sent by the gateway when subcription access to a resource is revoked. Any [direct subscription](#direct-subscription) to the resource are removed.  

The resource may still have [indirect](#indirect-subscription) subscriptions, in which case the resource is still considered subscribed. Otherwise, the resource is no longer considered subscribed.

**event**  
`<resourceID>.unsubscribe`

**data**  
[Unsubscribe event object](#unsubscribe-event-object).

### Unsubscribe event object
The unsubscribe event object has the following parameter:

**reason**  
[Error object](#error-object) describing the reason for the event.

### Example
```json
{
  "event": "messageService.messages.unsubscribe",
  "data": {
    "reason": {
      "code": "system.accessDenied",
      "message": "Access denied"
    }
  }
}
```

## Delete event

Delete events are sent to the client when the service considers the resource deleted.  
The resource is still to be considered subscribed, but the client will not receive any more events on the resource.  
The event has no payload.

**event**  
`<resourceID>.delete`
