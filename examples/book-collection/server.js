// Connect to NATS server
const nats = require('nats').connect("nats://localhost:4222");

// Predefined responses
const errorNotFound = JSON.stringify({ error: { code: "system.notFound", message: "Not found" }});
const accessGranted = JSON.stringify({ result: { get: true, call: "*" }});
const successResponse = JSON.stringify({ result: null });
const errorInvalidParams = (message) => JSON.stringify({ error: { code: "system.invalidParams", message }});

// Map of all book models
let bookModels = {
	"bookService.book.1": { id: 1, title: "Animal Farm", author: "George Orwell" },
	"bookService.book.2": { id: 2, title: "Brave New World", author: "Aldous Huxley" },
	"bookService.book.3": { id: 3, title: "Coraline", author: "Neil Gaiman" }
};

// Collection of books
var books = [
	{ rid: "bookService.book.1" },
	{ rid: "bookService.book.2" },
	{ rid: "bookService.book.3" }
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

// Access listener for all bookService resources. Everyone gets full access
nats.subscribe('access.bookService.>', (req, reply) => {
	nats.publish(reply, accessGranted);
});

// Book get listener
nats.subscribe('get.bookService.book.*', function(req, reply, subj) {
	let rid = subj.substring(4); // Remove "get." to get resource ID
	let model = bookModels[rid];
	if (model) {
		nats.publish(reply, JSON.stringify({ result: { model }}));
	} else {
		nats.publish(reply, errorNotFound);
	}
});

// Book set listener
nats.subscribe('call.bookService.book.*.set', (req, reply, subj) => {
	let rid = subj.substring(5, subj.length - 4); // Remove "call." and ".set" to get resource ID
	let model = bookModels[rid];
	if (!model) {
		nats.publish(reply, errorNotFound);
		return;
	}

	let r = JSON.parse(req);
	let p = r.params || {};
	let changed = null;

	// Check if title property was set
	if (p.title !== undefined) {
		let err = getError("Title", p.title);
		if (err) {
			nats.publish(reply, err);
			return;
		}
		changed = Object.assign({}, changed, { title: p.title });
	}

	// Check if author property was set
	if (p.title !== undefined) {
		let err = getError("Author", p.author);
		if (err) {
			nats.publish(reply, err);
			return;
		}
		changed = Object.assign({}, changed, { author: p.author });
	}

	// Publish update event on property changed
	if (changed) {
		nats.publish("event." + rid + ".change", JSON.stringify(changed));
	}

	// Reply success by sending an empty result
	nats.publish(reply, successResponse);
});

// Books get listener
nats.subscribe('get.bookService.books', function(req, reply, subj) {
	nats.publish(reply, JSON.stringify({ result: { collection: books }}));
});

// Books new listener
nats.subscribe('call.bookService.books.new', (req, reply) => {
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
	let rid = "bookService.book." + id; // Book's resource ID
	let book = { id, title, author }; // Book's model
	let ref = { rid }; // Reference value to the book

	bookModels[rid] = book;
	// Publish add event
	nats.publish("event.bookService.books.add", JSON.stringify({ value: ref, idx: books.length }));
	books.push(ref);

	// Reply success by sending an empty result
	nats.publish(reply, successResponse);
});

// Books delete listener
nats.subscribe('call.bookService.books.delete', (req, reply) => {
	let r = JSON.parse(req);
	let p = r.params || {};

	let id = p.id; // Book ID

	// Check if the book ID is a number
	if (typeof id !== "number")  {
		nats.publish(reply, errorInvalidParams("Book ID must be a number"));
		return;
	}

	let rid = "bookService.book." + id; // Book's resource ID
	// Check if book exists
	if (bookModels[rid]) {
		// Delete the model and remove the reference from the collection
		delete bookModels[rid];
		let idx = books.findIndex(v => v.rid === rid);
		if (idx >= 0) {
			books.splice(idx, 1);
			// Publish remove event
			nats.publish("event.bookService.books.remove", JSON.stringify({ idx }));
		}
	}

	// Reply success by sending an empty result
	nats.publish(reply, successResponse);
});

// System resets tells Resgate that the service has been (re)started.
// Resgate will then update any cached resource from bookService
nats.publish('system.reset', JSON.stringify({ resources: [ 'bookService.>' ]}));


// Run a simple webserver to serve the client.
// This is only for the purpose of making the example easier to run
const connect = require('connect');
const serveStatic = require('serve-static');
connect().use(serveStatic(__dirname)).listen(8082, function(){
    console.log('Client available at http://localhost:8082');
});
