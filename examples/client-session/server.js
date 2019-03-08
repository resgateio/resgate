const express = require('express');

// Load sessionService.js and tickerService.js
// Both may run as independent micro-services
require("./sessionService.js");
require("./tickerService.js");

// Create a simple webserver to serve the client.
let app = express();

// Serve index.html and start listening
app.use('/', express.static(__dirname));
app.listen(8085, () => {
	console.log('Client available at http://localhost:8085');
});
