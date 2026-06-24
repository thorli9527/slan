# service-biz

`service-biz` is the rebuilt business service using a standard layered layout.

Layers:

- `internal/api`: app / web / ops route sets and concrete HTTP handlers
- `internal/app`: application composition and server bootstrap
- `internal/service`: business contracts and use-case implementations
- `internal/repository`: persistence interfaces and in-memory adapters
- `internal/model`: domain models
- `internal/pkg/mqttkit`: shared MQTT config / topic / credential helpers

Current state:

- app / web / ops instances are split
- auth / device / network / ops core flows are wired through use-cases
- repository layer currently provides an in-memory implementation
- unsupported endpoints still return `501 Not Implemented`

This project can continue filling repository implementations and extending the
remaining `501` endpoints without changing the layer boundaries.
