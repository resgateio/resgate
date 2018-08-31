# Book Collection example

This is an example of how to create a RES service with collections and resource references written in Javascript (Node.js).
* It exposes a collection, `bookService.books`, containing book model references.
* It allows setting the books' title and author property through the `set` method.
* It allows creating new books that are added to the collection with the `new` method.
* It allows deleting existing books from the collection with the `delete` method.
* It verifies that a title and author is always set.

## Prerequisite

* Have NATS Server and Resgate running
* Have node.js installed

## Running the example

Run the following commands:
```bash
npm install
npm start
```

Open your web browser at http://localhost:8082

### Web resource

Book collection can be retrieved using ordinary HTTP GET requests:

**GET**  
```
http://localhost:8080/api/bookService/books
```

Books can be added using HTTP POST requests:

**POST**  
```
http://localhost:8080/api/bookService/books/add
```
*Body*  
```
{ "title": "Dracula", "author": "Bram Stoker" }
```

Books can be deleted using HTTP POST requests:

**POST**  
```
http://localhost:8080/api/bookService/books/delete
```
*Body*  
```
{ "id": 2 }
```