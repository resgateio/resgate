# RES Protocol v1.2 Update

## Changes from v1.1



## Deprecation

The update deprecates the pre-defined *new call requests* and replaces it with the [Resource response](res-service-protocol.md#response).


Any useThe specification has been updated to have the changed properties contained in a `values` field:

**v1.1 successful response payload to new call request:**
```json
{
   "result": { "rid": "example.foo.42" }
}
```

**v1.2 successful response payload to new call request:**
```json
{
   "rid": "example.foo.42"
}
```

## Reason

The ability to return a resource reference as part of a call or auth is useful for situations such as n Model change events, as defined in RES-service specification V1.0, gave no room to include meta data in the event. This is a design flaw which prevents the specification to adapt to requests such as version numbering of resources.

## Impact

The version upgrade affects both the RES-service protocol and the RES-client protocol.

* A client supporting Any client should add a deprecated warning on the use of the predefined  `new

Resgate can detect service legacy (v1.1) behavior and handle it, but will log a *Deprecated* warning with a link to this page.

## Migration

Any service that handled *new call requests* should be updated to follow v1.2 specification.

This can be done in partial steps, one service at a time, as Resgate detects legacy behavior for each *new call response*.
