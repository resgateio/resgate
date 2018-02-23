# Issues and considered features

This document is a notepad for issues and thoughts surrounding the RES protocol, until a proper issue tracker exists.

---
## Connection ID (CID) placeholder for client requests
**Issue**  
The client has no way of requesting connection specific resources.

**Suggested solution**  
RES Protocol could define a placeholder that can be used as part of a resourceID:

`get.authService.loggedUser.$cid`

Where `$cid` will be replaced by the connection ID (CID) by the gateway before passing the request to the services.

---
## Client notification on conn event
**Issue**  
When a `conn.<cid>.token` event is sent (or any other future conn events), the client gets no notification.

Services (especially an authentication service, but any service that works with the connection id (cid)) needs to have a way of passing data to a specific connection.

**Suggested solution**  
Maybe this can be solved with the $cid placeholder?
Then you can just have a resource that is connection specific:

```javascript
api.getResource('authService.loggedUser.$cid').then(loggedUser => {
	loggedUser.on('change', () => {
		// Something happened to my token
	});
});
```

 The `conn.<cid>.token` events will still not propagate any information to the client, but the service can send information to the client in a separate `event.authService.loggedUser.xxxx.change` event.

---
## Reaccess on system.reset
When a `system.reset` event is received, the gateway will currently just load the data a new.

Should a reaccess event be created, forcing each subscription to check with the service if it still has access?

---
## Delete property on change event

**Issue**  
RES protocol currently provides no way of deleting a model property once it is set.

**Suggested solution**  
A special object, eg. `{"$action":"delete"}`, could indicate deletion.
```json
{
	"foo": "New or changed value",
	"bar": {"$action":"delete"}
}
```

---
## Support for complex/linked resources

**Issue**  
To fetch a complex resource structure, you currently have to do it with multiple requests:

Example:
```javascript
Promise.all([
	api.getResource('userService.user.42'),
	api.getResource('userService.user.42.roles')
]).then(result => {
	api.getResource('orgService.org.'+result[0].orgId)
		.then(org => {
			console.log("Name : ", result[0].name);
			console.log("Roles: ", result[1]);
			console.log("Org. : ", org);
		});
});
```

**Suggested solution**  
RES protocol could define a special object that is used for resource links, eg. `{"$rid":"orgService.org.3"}`. If a property contains such a link, the gateway will fetch these resources, doing indirect subscriptions just like collections models, and return them together with the parent model in the response.

Example on `get.userService.user.42` service result response:
```json
{
	"id": 42,
	"name": "Jane Doe",
	"roles": {"$rid":"userService.user.42.roles"},
	"org": {"$rid":"orgService.org.3"}
}
```

In client:
```javascript
api.getResource('userService.user.42').then(user => {
	console.log("Name : ", user.name);
	console.log("Roles: ", user.roles);
	console.log("Org. : ", user.org);
});
```