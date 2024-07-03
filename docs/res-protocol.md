# RES Protocol

*Version: [1.2.3](res-protocol-semver.md)*

## Table of contents
- [Introduction](#introduction)
- [Terminology](#terminology)
  * [Resources](#resources)
  * [Resource IDs](#resource-ids)
  * [Models](#models)
  * [Collections](#collections)
  * [Values](#values)
  * [Primitives](#primitives)
  * [Resource references](#resource-references)
  * [Data values](#data-values)
  * [Messaging system](#messaging-system)
  * [Services](#services)
  * [Gateways](#gateways)
  * [Clients](#clients)
  * [Connection IDs](#connection-ids)

# Introduction

RES (REsource Subscription) is a protocol for creating scalable client-server Push APIs consisting of resources served from stateless services connected by a messaging system. Clients subscribe to resources, and call upon their methods, through an API agnostic gateway acting as a bridge between the services and clients.

It is designed for creating modular cloud based applications where new services and functionality can be added without causing disruption to a running production environment.

The protocol consists of two subprotocols:

* [RES-Client protocol](res-client-protocol.md) - the protocol used by the clients
* [RES-Service protocol](res-service-protocol.md)  - the protocol used by the services

This document gives an overview of the protocol and its features, and describes the common concepts used in the RES-Client and RES-Service protocols.

# Terminology

## Resources

RES protocol is built around a concept of resources. A resource may be either be a [*model*](#models) or a [*collection*](#collections). Each resource (model or collection) is identified by a unique [*resource ID*](#resource-ids), also called *rid* for short.

## Resource IDs
 A *resource ID* is a string that consist of a *resource name* and an optional *query*.

**resource name**  
A *resource name* is case-sensitive and must be non-empty alphanumeric strings with no embedded whitespace, and part-delimited using the dot character (`.`).  
The first part SHOULD be the name of the service owning the resource. The following parts describes and identifies the specific resource.

**query**  
The *query* is separated from the resource name by a question mark (`?`). The format of the query is not enforced, but it is recommended to use URI queries in case the resources are to be accessed through web requests.  
May be omitted. If omitted, then the question mark separator MUST also be omitted.

**Examples**

* `example.users` - A collection representing a list of users
* `example.user.42` - A model representing a user
* `example.user.42.roles` - A collection of roles held by a user
* `chat.messages?start=0&limit=25` - A collection representing the first 25 messages of a list
* `example.users?q=Jane` - A collection of users with the name Jane

## Models

A model is an unordered set of named properties and [values](#values) represented by a JSON object.

**Example**
```json
{
    "id": 42,
    "name": "Jane Doe",
    "roles": { "rid": "example.user.42.roles" }
}
```

## Collections

A collection is an ordered list of [values](#values) represented by a JSON array.

**Example**
```json
[ "admin", "tester", "developer" ]
```

## Values

A value is either a [primitive](#primitives), a [resource reference](#resource-references), or a [data value](#data-values).  

### Example
```javascript
"foo" // string
42    // number
true  // boolean true
false // boolean false
null  // null

{ "rid": "example.user.42" }             // resource reference
{ "rid": "example.page.2", "soft":true } // soft reference
{ "data": { "foo": [ "bar" ] }}          // data value
{ "data": 42 }                           // data value interchangeable with the primitive 42
```

## Primitives

A primitive is either a JSON `string`, `number`, `true`, `false`, or `null` value.

## Resource references

A resource reference is a link to a resource. A *soft reference* is a resource reference which will not automatically be followed by the gateway. The resource reference is a JSON object with the following parameters:

**rid**  
Resource ID of the referenced resource.  
MUST be a valid [resource ID](#resource-ids).

**soft**  
Flag telling if the reference is a soft resource reference.  
May be omitted if the reference is not a soft reference.  
MUST be a boolean.

## Data values

A data value contains any JSON value, including nested objects and arrays.  
If the contained value can be expressed as a [primitive](#primitives), the data value is interchangeable with the primitive.  
The data value is a JSON object with the following parameter:

**data**  
The contained JSON value.  

## Messaging system

The messaging system handles the communication between [services](#services) and [gateways](#gateways). It MUST provide the following functionality:
* support publish-subscribe pattern
* support request-response pattern
* support subscriptions to requests and messages using wild cards
* guarantee that the messages and responses sent by any one service will arrive in the same order as they were sent
* guarantee any service or gateway is notified on connection loss, or any other disturbance that might have caused any message or response to not having been delivered.

Resgate, the gateway implementation of the RES protocol, uses [NATS server](https://nats.io/about/) as messaging system.

## Services

A service, or microservice, is a server application that replies to requests sent by [gateways](#gateways) or other services, and emits [events](res-service-protocol.md#events) over the [messaging system](#messaging-system). The service that replies to [get requests](res-service-protocol.md#get-request) for a specific resource is called the *owner* of that resource. Only one service per messaging system may have ownership of a specific resource at any time.

There is no protocol limitation on how many services may be connected to a [messaging system](#messaging-system) as long as there is no conflict in resource ownership.

A service uses the [RES-service protocol](res-service-protocol.md) for communication.

## Gateways

A gateway acts as a bridge between [services](#services) and [clients](#clients). It communicates with services through the [messaging system](#messaging-system) using the [RES-service protocol](res-service-protocol.md), and lets clients connect with WebSocket to communicate using the [RES-client protocol](res-client-protocol.md). It may also act as a web server to allow access to the API resources using HTTP requests.

Each gateway keeps track on which resources each client, that has connected to it, is currently subscribing to. It also stores any access token that has been issued to the client connections, blocking any request or event which the client does not have access to.

Gateways also handles caching of resources, keeping its internal cache up to date by intercepting any resource event which describes any resource modification.

There is no protocol limitation on how many gateways may be connected to a [messaging system](#messaging-system).

If a gateway loses the connection to a client, it will make no attempt at recovering the connection, but will clear any state it held for that connection. If a gateway loses the connection to the messaging system, it SHOULD disconnect all of its clients to allow them to reconnect to another gateway.

## Clients

A client is the application that accesses the API by connecting to a gateway's WebSocket. While it may be possible to access the API resources through HTTP requests, any reference in these documentations to *client* implies a client using the WebSocket.

A client uses the [RES-client protocol](res-client-protocol.md) for communication.

## Connection IDs

A connection ID, or *cid* for short, is a unique ID generated by the [gateways](#gateways) for every new client connection.  
Connection IDs are never sent to the clients.  
Any reconnect will result in a new connection ID.
