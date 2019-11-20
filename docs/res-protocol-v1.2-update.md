# RES Protocol v1.2 Update

[Full changelog](CHANGELOG.md)

## Changes from v1.1

The update deprecates the pre-defined *new call requests*. The same behaviour can now be achieved with the [Resource response](res-service-protocol.md#response).

## Migration

Any service that handled *new call requests* should be updated to follow the v1.2 specification.

**v1.1 successful response payload for new call request:**
```json
{
   "result": { "rid": "example.foo.42" }
}
```

**v1.2 successful response payload for new call request:**
```json
{
   "rid": "example.foo.42"
}
```

Migration can be done in partial steps, one service at a time, as Resgate detects legacy behavior for each *new call response*.

## Impact

The update affects both the RES-service and RES-client protocol, but Resgate can detect and handle legacy (v1.1) behavior for both services and clients.

Legacy support will be available until 2021-11-30.