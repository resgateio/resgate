# Hello World Example

This is a Hello World example written in javascript (Node.js).
* It exposes a single resource: `example.model`
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

## Files

**server.js**
```javascript
const nats = require('nats').connect('nats://localhost:4222');

nats.subscribe('get.example.model', (req, reply) => {
  nats.publish(reply, JSON.stringify({ result: {
    model: { message: "Hello, world!" }
  }}));
});

nats.subscribe('access.example.model', (req, reply) => {
  nats.publish(reply, JSON.stringify({ result: {
    get: true
  }}));
});
```

**index.html**
```html
<!DOCTYPE html>
<html>
  <head>
     <meta charset="UTF-8" />
     <title>Resgate - Hello World example</title>
     <script src="https://unpkg.com/resclient@latest/dist/resclient.min.js"></script>
  </head>
  <body>
     <script>
         const ResClient = resclient.default;
         let client = new ResClient('ws://localhost:8080');

         client.get('example.model').then(model => {
            document.body.textContent = model.message;
         }).catch(err => {
            document.body.textContent = "Error getting model. Are NATS Server and Resgate running?";
         });
     </script>
  </body>
</html>
```


## Things to try out

**View message**  
Open the client to see the message:
```text
http://localhost:8081
```

**Get model with REST**  
Resources can be retrieved using ordinary HTTP GET requests:
```text
http://localhost:8080/api/example/model
```
