// Connect to NATS server
const nats = require('nats').connect("nats://localhost:4222");
const crypto = require('crypto');

// List of users and their passwords
const users = {
	admin: { id: 1, name: "Administrator", role: "admin", password: "admin" },
	guest: { id: 2, name: "Guest", role: "guest", password: "guest" },
	jane: { id: 3, name: "Jane Doe", role: "user", password: "jane" },
	john: { id: 4, name: "John Doe", role: "user", password: "john" }
};

// Relogin key expires after 2 min. Should be renewed after 1 min.
// In a real environment, this should be longer.
const EXPIRE_DURATION = 2 * 60 * 1000;

// Session lookup objects
let sessionsByKey = {};
let sessionsBySID = {};
let sessionsByCID = {};

// Helper function that generates a random ID string
// This examples ignores the minimal chance of duplicates IDs.
function generateID() {
	return crypto.randomBytes(24).toString('base64');
};

// Sets a session's current connection and sends an access token to the connection.
function setSessionCID(session, newCid) {
	let { cid, sid, user } = session;

	// Check if we have an old connection ID
	if (cid) {
		// Clear token on the old connection just to be sure
		// We don't want two connections on the same session
		nats.publish('conn.' + cid + '.token', JSON.stringify({ token: null }));
		delete sessionsByCID[cid];
	}

	// Associate the session with the new connection ID
	session.cid = newCid;
	// Store the session on the new connection ID
	sessionsByCID[newCid] = session;

	// Send a new access token to the connection.
	// Only Resgate and services will have access to this information
	nats.publish('conn.' + newCid + '.token', JSON.stringify({ token:
		{ sid, userId: user.id, role: user.role }
	}));

	// Update the connection's user model
	// The client may access this information
	nats.publish('event.session.user.' + newCid + '.change', JSON.stringify({
		values: { id: user.id, name: user.name, role: user.role }
	}));
}

// Issues a new relogin key to the session and resets the session expire timeout
function issueReloginKey(session) {
	// Clear last expire timeout and set a new
	clearTimeout(session.expireId);
	session.expireId = setTimeout(() => disposeSession(session), EXPIRE_DURATION);

	// Create a new relogin key
	let key = generateID();

	// Add the key to the end of the list of keys issued to the session
	session.keys.push(key);

	// Store the session on the key
	sessionsByKey[key] = session;

	return key;
};

// Disposes the session and clears the connection's access token and user model
function disposeSession(session) {
	let { cid, keys, sid, expireId } = session;

	// Clear the relogin key's expire timeout
	clearTimeout(expireId);

	// Clear the connection's user model
	nats.publish('event.session.user.' + cid + '.change', JSON.stringify({
		values: { id: null, name: null, role: null }
	}));
	// Clear token from current connection
	nats.publish('conn.' + cid + '.token', JSON.stringify({ token: null }));

	// Delete references to session
	keys.map(key => delete sessionsByKey[key]);
	delete sessionsByCID[cid];
	delete sessionsBySID[sid];
};

// Password login auth listener
nats.subscribe('auth.session.login', function(req, reply) {
	let { cid, params, token } = JSON.parse(req);

	// If client already has an access token, respond with an error
	if (token) {
		nats.publish(reply, JSON.stringify({ error:
			{ code: 'session.alreadyLoggedIn', message: "Already logged in" }
		}));
		return;
	}

	// Validate we have both a username and a password
	if (!params ||
		typeof params.username !== 'string' ||
		typeof params.password !== 'string'
	) {
		nats.publish(reply, JSON.stringify({ error:
			{ code: 'system.invalidParams', message: "Invalid parameters" }
		}));
		return;
	}

	// Get user and validate the password
	let user = users[params.username.toLowerCase()];
	if (!user || user.password !== params.password) {
		nats.publish(reply, JSON.stringify({ error:
			{ code: 'session.wrongUsernamePassword', message: "Wrong username or password" }
		}));
		return;
	}

	// Create a new session and store relevant information
	let sid = generateID();
	let session = { sid, user, created: new Date(), keys: [] };
	sessionsBySID[sid] = session;

	// Set the cid to the session and send an access token
	setSessionCID(session, cid);

	// Issue a new relogin key for the session
	let reloginKey = issueReloginKey(session);

	// Respond with the relogin key
	nats.publish(reply, JSON.stringify({ result: { reloginKey }}));
});

// Relogin auth listener
nats.subscribe('auth.session.relogin', (req, reply) => {
	let { params, cid } = JSON.parse(req);
	let key = params && params.reloginKey;

	// Validate we have a reloginKey
	if (typeof key !== 'string' ) {
		nats.publish(reply, JSON.stringify({ error:
			{ code: 'system.invalidParams', message: "Invalid parameters" }
		}));
		return;
	}

	// Get session and validate the key matches the last issued relogin key
	let session = sessionsByKey[key];
	if (!session || session.keys[session.keys.length - 1] !== key) {
		if (session) {
			// The relogin key has been used two times and might be a stolen key.
			// Let's dispose the session all together, just to be sure.
			disposeSession(session);
		}

		nats.publish(reply, JSON.stringify({ error:
			{ code: 'session.invalidReloginKey', message: "Invalid relogin key" }
		}));
		return;
	}

	// If the client has relogged in from a new connection,
	// set the new cid to the session and send an access token
	if (session.cid != cid) {
		setSessionCID(session, cid);
	}

	// Issue a new relogin key
	let reloginKey = issueReloginKey(session);

	// Respond with the relogin key
	nats.publish(reply, JSON.stringify({ result: { reloginKey }}));
});

// Logout auth listener
nats.subscribe('auth.session.logout', function(req, reply) {
	let { cid } = JSON.parse(req);
	// Dispose the session if we have one on the connection
	let session = sessionsByCID[cid];
	if (session) {
		disposeSession(session);
	}
	nats.publish(reply, JSON.stringify({ result: null }));
});

// User access listener for model access.session.user.{cid}
// Access is only granted if the cid matches that of the resource
nats.subscribe('access.session.user.*', (req, reply, subject) => {
	let { cid } = JSON.parse(req);
	nats.publish(reply, JSON.stringify({ result: {
		get: cid == subject.substr(20) // Remove 'access.session.user.' from subject
	}}));
});

// User get listener for model access.session.user.{cid}
nats.subscribe('get.session.user.*', (req, reply, subject) => {
	let cid = subject.substr(17); // Remove 'get.session.user.' from subject

	// Check if the connection is logged in to a session
	let session = sessionsByCID[cid];
	if (session) {
		// Return a model with the session's user information
		let { user } = session;
		nats.publish(reply, JSON.stringify({ result: { model:
			{ id: user.id, name: user.name, role: user.role }
		}}));
	} else {
		// Return a model with all properties set to null
		nats.publish(reply, JSON.stringify({ result:
			{ model: { id: null, name: null, role: null }}
		}));
	}
});
