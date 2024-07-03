// Connect to NATS server
const nats = require('nats').connect("nats://localhost:4222");

// Predefined responses
const errorNotFound = JSON.stringify({ error: { code: "system.notFound", message: "Not found" }});
const accessGranted = JSON.stringify({ result: { get: true, call: "*" }});
const successResponse = JSON.stringify({ result: null });
const errorInvalidParams = (message) => JSON.stringify({ error: { code: "system.invalidParams", message }});

// Map of all book models
let bookModels = {
	"library.book.1": { id: 1, title: "Animal Farm", author: "George Orwell" },
	"library.book.2": { id: 2, title: "Brave New World", author: "Aldous Huxley" },
	"library.book.3": { id: 3, title: "Coraline", author: "Neil Gaiman" }
};

// Collection of books
var books = [
	{ rid: "library.book.1" },
	{ rid: "library.book.2" },
	{ rid: "library.book.3" }
];

// ID counter for book models
let nextBookID = books.length + 1;

// Validation function
function getError(field, value) {
	// Check we actually got a string
	if (typeof value !== "string") {
		return errorInvalidParams(field + " must be a string");
	}
	// Check value is not empty
	if (!value.trim()) {
		return errorInvalidParams(field + " must not be empty");
	}
}

// Access listener for all library resources. Everyone gets full access
nats.subscribe('access.library.>', (req, reply) => {
	nats.publish(reply, accessGranted);
});

// Book get listener
nats.subscribe('get.library.book.*', function(req, reply, subj) {
	let rid = subj.substring(4); // Remove "get." to get resource ID
	let model = bookModels[rid];
	if (model) {
		nats.publish(reply, JSON.stringify({ result: { model }}));
	} else {
		nats.publish(reply, errorNotFound);
	}
});

// Book set listener
nats.subscribe('call.library.book.*.set', (req, reply, subj) => {
	let rid = subj.substring(5, subj.length - 4); // Remove "call." and ".set" to get resource ID
	let model = bookModels[rid];
	if (!model) {
		nats.publish(reply, errorNotFound);
		return;
	}

	let r = JSON.parse(req);
	let p = r.params || {};
	let changed = null;

	// Check if title property was set and has changed
	if (p.title !== undefined && p.title !== model.title) {
		let err = getError("Title", p.title);
		if (err) {
			nats.publish(reply, err);
			return;
		}
		changed = Object.assign({}, changed, { title: p.title });
	}

	// Check if author property was set and has changed
	if (p.author !== undefined && p.author !== model.author) {
		let err = getError("Author", p.author);
		if (err) {
			nats.publish(reply, err);
			return;
		}
		changed = Object.assign({}, changed, { author: p.author });
	}

	// Publish update event on property changed
	if (changed) {
		Object.assign(model, changed);
		nats.publish("event." + rid + ".change", JSON.stringify({ values: changed }));
	}

	// Reply success by sending an empty result
	nats.publish(reply, successResponse);
});

// Books get listener
nats.subscribe('get.library.books', function(req, reply, subj) {
	nats.publish(reply, JSON.stringify({ result: { collection: books }}));
});

// Books new listener
nats.subscribe('call.library.books.new', (req, reply) => {
	let r = JSON.parse(req);
	let p = r.params || {};

	let title = p.title || "";
	let author = p.author || "";

	// Check if title and author was set
	if (!title || !author) {
		nats.publish(reply, errorInvalidParams("Must provide both title and author"));
		return;
	}

	let id = nextBookID++; // Book ID
	let rid = "library.book." + id; // Book's resource ID
	let book = { id, title, author }; // Book's model
	let ref = { rid }; // Reference value to the book

	bookModels[rid] = book;
	// Publish add event
	nats.publish("event.library.books.add", JSON.stringify({ value: ref, idx: books.length }));
	books.push(ref);

	// Reply success by sending an empty result
	nats.publish(reply, successResponse);
});

// Books delete listener
nats.subscribe('call.library.books.delete', (req, reply) => {
	let r = JSON.parse(req);
	let p = r.params || {};

	let id = p.id; // Book ID

	// Check if the book ID is a number
	if (typeof id !== "number") {
		nats.publish(reply, errorInvalidParams("Book ID must be a number"));
		return;
	}

	let rid = "library.book." + id; // Book's resource ID
	// Check if book exists
	if (bookModels[rid]) {
		// Delete the model and remove the reference from the collection
		delete bookModels[rid];
		let idx = books.findIndex(v => v.rid === rid);
		if (idx >= 0) {
			books.splice(idx, 1);
			// Publish remove event
			nats.publish("event.library.books.remove", JSON.stringify({ idx }));
		}
	}

	// Reply success by sending an empty result
	nats.publish(reply, successResponse);
});

// System resets tells Resgate that the service has been (re)started.
// Resgate will then update any cached resource from library
nats.publish('system.reset', JSON.stringify({ resources: [ 'library.>' ] }));

// Create a simple webserver to serve the index.html client.
const express = require('express');
let app = express();
app.use('/', express.static(__dirname));
app.listen(8083, () => {
	console.log('Client available at http://localhost:8083');
});
