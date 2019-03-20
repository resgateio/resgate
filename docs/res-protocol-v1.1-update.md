# RES Protocol v1.1 Update

## Changes from v1.0

The update only affects [Model change event](res-service-protocol.md#model-change-event).

The specification has been updated to have the changed properties contained in a `props` field:

**v1.0 model change event payload:**
```json
{
   "foo": "bar",
   "faz": 42
}
```

**v1.1 model change event payload:**
```json
{
   "props": {
      "foo": "bar",
      "faz": 42
   }
}
```

## Reason

Model change events, as defined in RES-service specification V1.0, gave no room to include meta data in the event. This is a design flaw which prevents the specification to adapt to requests such as version numbering of resources.

## Impact

The version upgrade only affect services. The RES-client protocol, and subsequently any RES client, is unaffected.

Resgate can detect service legacy (v1.0) behaviour and handle it, but will log a *Deprecated* warning with a link to this page.

The only time a legacy service might be mistakenly taken as non-legacy (v1.1), is for the following two change event payloads:
```json
{
    "props": { "rid": "example.model" }
}
```
or
```json
{
   "props": { "action": "delete" }
}
```

## Migration

Any service that sends [model change event](res-service-protocol.md#model-change-event) should be updated to follow v1.1 specification.

This can be done in partial steps, one service at a time, as Resgate detects legacy behaviour for each separate model change event.
