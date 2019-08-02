# Client Session Example

This example, written in Javascript (Node.js), shows how to create client sessions that survive client reloads and network disconnects. It also shows how to use a [connection ID tag](../../docs/res-client-protocol.md#connection-id-tag) to create connection specific resources that may contain session user data.

The server consists of three files:

**tickerService.js**
* exposes a single resource: `ticker.model`
* updates the model every second
* requires a token to access the resource

**sessionService.js**
* exposes three [authentication methods](../../docs/res-service-protocol.md#auth-request), `session.login`, `session.relogin`, and `session.logout`
	* `login` auth method creates a new session stored on the service, and issues a *reloginKey* that can be used a single time to resume an existing session
	* `relogin` auth method lets the client resume a session using a *reloginKey*. The key will be used up, and a new *reloginKey* is issued.
	* `logout` auth method disposes the session and clears the access token
* disposes sessions after 1m30s counting from last issued *reloginkey*
* exposes a connection specific user resource: `session.user.{cid}`
* requires the [connection ID](../../docs/res-protocol.md#connection-ids) to match the `{cid}` in the user resource in order to access it

**server.js**
* starts the *tickerService.js* and *sessionService.js* micro-services
* serves `/index.html` which is the example client


The client consists of a single file:

**index.html**
* sets the client's `setOnConnect` callback to try relogin if a *reloginKey* is found in *localStorage*
* when not logged in
	* lets you login with different users
* when logged in
	* displays the info from the `session.user.{cid}` model
	* gives you the option to log out
	* calls auth method `session.relogin` every minute to get a new *reloginKey* before the old one expires
	* displays the `ticker.model` counter

## Prerequisite

* Have [NATS Server](https://nats-io.github.io/docs/nats_server/installation.html) and [Resgate](https://resgate.io/docs/get-started/installation/) running
* Have [node.js](https://nodejs.org/en/download/) installed

## Install and run

Run the following commands:
```bash
npm install
npm start
```
Open the client
```
http://localhost:8086
```

## Things to try out

**Gain access**  
Log in with a username and password shown in the client to set the connection's access token and start seeing both the user info and the ticking counter.

**Reload page**  
Press F5 to reload the web page. The client will relogin to the session and resume displaying the user info and counter.

**Restart Resgate**  
Disconnect the WebSocket connection by restarting Resgate. The client will periodically try to reconnect. Once Resgate is started again, the client will relogin and resume receiving events (unless the *relogin key* has expired).

**Expire the relogin key**  
Login and then close the page. Load the page again after more than 1m30s. The *relogin key* should now have expired, causing the client to fail on relogin.

## Notes

**In-memory sessions**  
The example stores the *sessions* and *relogin keys* in memory. In real usage, this information should be persisted in a database to allow *sessionService.js* to be restarted without losing client sessions.

**Additional header security**  
It is recommended to secure the *relogin key* additionally by using a header session cookie issued by the web server that serves the client.

By storing a header cookie's session ID value in the *sessionService's* `session` object, one can validate that any *relogin* is done by a client having the same session ID in the header cookie.

**Dispose session on stolen key**  
By making a small adjustment to the client's `updateUserInfo` function, we can cause a session to be disposed in case a malicious user stole our *reloginKey* and used it. This is done by letting the client make an extra attempt to *relogin* once logged out.

If the client was forcefully logged out due to a session hijack, the *relogin key* would now have been used twice, causing the entire session to be disposed by the service.

```javascript
function updateUserInfo(user) {
   // No user ID means we are not logged in
   if (!user.id) {
      relogin();
      /* ... */
   }
   /* ... */
}
```