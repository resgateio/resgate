# RES Protocol

## Table of contents
- [Introduction](#introduction)
  * [Features](#features)
- [Getting started](#getting-started)
- [Resources](#resources)
  * [Resource IDs](#resource-ids)
  * [Models](#models)
  * [Collections](#collections)
- [Values](#values)
- [Messaging system](#messaging-system)
- [Services](#services)
- [Gateways](#gateways)
- [Clients](#clients)
- [Architecture](#architecture)


# Introduction

RES (REsource Subscription) is a protocol for creating scaleable client-server Push APIs consisting of resources served from stateless services connected by a messaging system. Clients subscribe to resources, and call upon their methods, through an API agnostic gateway acting as a bridge between the services and clients.

It is designed for creating modular cloud based applications where new services and functionality can be added without causing disruption to a running production environment.

The protocol consists of two subprotocols:

* [RES-Client protocol](res-client-protocol.md) - the protocol used by the clients
* [RES-Service protocol](res-service-protocol.md)  - the protocol used by the services

This document gives an overview of the protocol and its features, and describes the common concepts used in the RES-Client and RES-Service protocols.

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
All resources are cachable by the gateways, taking load off the services. The gateway keeps its cache up-to-date using the events emitted from the services.

**Resource queries**  
The protocol supports resource queries where partial or filtered resources are requested, such as for searches, filters, or pagination. Just like any other resource, query resources are also live.

**Web resources**  
All resources may be accessed using ordinary web (http) requests, in a RESTful manner. The same goes for all resource method calls.

**Simple**  
The protocol is made to be simple. Simple to create services. Simple to use in clients. The gateway takes care of the complex synchronization parts.


# Getting started

The best place to start is to [install resgate](https://github.com/jirenius/resgateway), a RES gateway implementation written in [Go](http://golang.org), using [NATS server](https://nats.io/documentation/server/gnatsd-intro/) as messaging system. The resgate README contains information on how to get started, and how to make a basic *Hello world* example service and client.

# Resources

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
MUST be a valid [resource ID](#resource-ids).

# Messaging system

The messaging system handles the communication between [services](#services) and [gateways](#gateways). It MUST provide the following functionality:
* support publish-subscribe pattern
* support request-response pattern
* support subscriptions to requests and messages using wild cards
* guarantee that the messages and responses sent by any one service will arrive in the same order as they were sent
* guarantee any service or gateway is notified on connection loss, or any other disturbance that might have caused any message or response to not having been delivered.

Resgate, the gateway implementation of the RES protocol, uses [NATS server](https://nats.io/documentation/server/gnatsd-intro/) as messaging system.

# Services

A service, or microservice, is a server application that replies to requests sent by [gateways](#gateways) or other services, and emits [events](res-service-protocol.md#events) over the [messaging system](#messaging-system). The service that replies to [get requests](res-service-protocol.md#get-requests) for a specific resource is called the *owner* of that resource. Only one service per messaging system may have ownership of a specific resource at any time.

There is no protocol limitation on how many services may be connected to a [messaging system](#messaging-system) as long as there is no conflict in resource ownership.

A service uses the [RES-service protocol](#res-service-protocol.md) for communication.

# Gateways

A gateway acts as a bridge between [services](#services) and [clients](#clients). It communicates with services through the [messaging system](#messaging-system) using the [RES-service protocol](#res-service-protocol.md), and lets clients connect with WebSocket to communicate using the [RES-client protocol](#res-client-protocol.md). It may also act as a web server to allow access to the API resources using HTTP requests.

Each gateway keeps track on which resources each client, that has connected to it, is currently subscribing to. It also stores any access token that has been issued to the client connections, blocking any request or event which the client does not have access to.

Gateways also handles caching of resources, keeping its internal cache up to date by intercepting any resource event which describes any resource modification.

There is no protocol limitation on how many gateways may be connected to a [messaging system](#messaging-system).

If a gateway loses the connection to a client, it will make no attempt at recovering the connection, but will clear any state it held for that connection. If a gateway loses the connection to the messaging system, it SHOULD disconnect all of its clients to allow them to reconnect to another gateway.

# Clients

A client is the application that accesses the API by connecting to a gateway's WebSocket. While it may be possible to access the API resources through HTTP requests, any reference in these documentations to *client* implies a client using the WebSocket.

A client uses the [RES-service protocol](#res-service-protocol.md) for communication.

# Architecture

![Diagram of a simple resgate architecture](img/res-network.svg)

An example setup consisting of three services and two resgates with a load balancer.

For additional scaling and high availability, the setup may be replicated and distributed geographically as long as each service has a way of synchronizing with the same services in other replicas.
