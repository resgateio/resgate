# RES Protocol semantic versioning

Each protocol version is given a version number MAJOR.MINOR.PATCH, using semantic versioning with the modified meaning:

### MAJOR version
Incremented when the protocol is no longer backwards compatible. Any gateway implementation should always target the same major version. Services written for one major version will not work for a different major version.

### MINOR version
Incremented when the protocol deprecates previously valid behavior. Any gateway implementation should still handle services written for previous minor versions for a period of time (~1 year), but should log warnings in case legacy behavior is detected.

During the deprecation period, service developers need to upgrade their services. A document describing these changes, and how to upgrade a service, is provided. An example is the [RES Protocol v1.1 Update](res-protocol-v1.1-update.md) document.

### PATCH version
Incremented when backward compatible features are added to the protocol. Services written for previous patch versions will continue to work without modifications.

New patch versions may include features like:
* New optional error codes
* New resource types
* New resource events
* New optional event properties
* New optional response properties

## Notes

* The current RES Protocol version is stated at the top of the [RES Protocol Specification](res-protocol.md) document.
* The same version number is used for both the [RES Service protocol](res-service-protocol.md) and [RES Client protocol](res-client-protocol.md), even if some version changes only affect one part of the protocol.
* Language updates, clarifications, or added examples, does not justify a version change.
