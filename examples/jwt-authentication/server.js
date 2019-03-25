const express = require('express');
const jwt = require('jsonwebtoken');

// Load authService.js and exampleService.js
// Both may run as independent micro-services
require("./authService.js");
require("./exampleService.js");

const mySecret = 'shhhhh';
const jwtCookieName = 'access-token';

// Create a simple webserver to serve the client.
let app = express();

// Accessing /login will set a JWT token in a cookie
app.get('/login', (req, res) => {
	let token = jwt.sign({ foo: 'bar' }, mySecret);
	res.cookie(jwtCookieName, token);
	res.send('The access-token cookie is now set. <a href="/">Go back</a>');
});

// Accessing /logout will clear the JWT token cookie
app.get('/logout', (req, res) => {
	res.clearCookie(jwtCookieName);
	res.send('The access-token cookie is now cleared. <a href="/">Go back</a>');
});

// Serve index.html and start listening
app.use('/', express.static(__dirname));
app.listen(8084, () => {
	console.log('Client available at http://localhost:8084');
});
