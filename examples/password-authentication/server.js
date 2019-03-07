const express = require('express');

// Load passwdService.js and tickerService.js
// Both may run as independent micro-services
require("./passwdService.js");
require("./tickerService.js");

// Create a simple webserver to serve the client.
let app = express();

// Serve index.html and start listening
app.use('/', express.static(__dirname));
app.listen(8084, () => {
	console.log('Client available at http://localhost:8084');
});
