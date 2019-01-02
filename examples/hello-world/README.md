# Hello World example

This is an example of a simple Hello World RES service written in javascript (node.js).
* It exposes a single resource: `example.mymodel`.
* It allows setting the resource's `message` property through the `set` method.
* It resets the model on server restart.
* It serves a web client at http://localhost:8081

## Prerequisite

* Have [NATS Server](https://nats.io/download/nats-io/gnatsd/) and [Resgate](https://github.com/jirenius/resgate) running
* Have [node.js](https://nodejs.org/en/download/) installed

## Install and run

Run the following commands:
```bash
npm install
npm start
```
Open the client
```
http://localhost:8081
```

## Things to try out

**Realtime updates**  
Run the client in two separate tabs, edit the message in one tab, and observe realtime updates in both.

## Web resources

Resources can be retrieved using ordinary HTTP GET requests, and methods can be called using HTTP POST requests.

### Get model
```
GET http://localhost:8080/api/example/mymodel
```

### Update model
```
POST http://localhost:8080/api/example/mymodel/set
```
*Body*  
```
{ "message": "Updated through HTTP" }
```