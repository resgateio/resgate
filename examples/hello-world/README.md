# Hello World example

This is an example of a simple Hello World RES service written in Javascript (Node.js).
* It exposes a single resource: `exampleService.myModel`.
* It allows setting the resource's Message property through the `set` method.

## Prerequisite

* Have NATS Server and Resgate running
* Have node.js installed

## Install and run

Run the following commands:
```bash
npm install
npm start
```
### Open the client
```
http://localhost:8081
```

### Web resource

Resources can be retrieved using ordinary HTTP GET requests:

**GET**  
```
http://localhost:8080/api/exampleService/myModel
```

Methods can be called using HTTP POST requests:

**POST**  
```
http://localhost:8080/api/exampleService/myModel/set
```
*Body*  
```
{ "message": "Updated through HTTP" }
```