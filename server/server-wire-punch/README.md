# server-wire-punch

`server-wire-punch` is the short-lived NAT traversal coordinator for SLAN direct UDP paths.

It is intentionally separate from `service-biz`: biz owns users, devices, ACLs, billing, and durable network state; punch owns temporary endpoint/session state used to coordinate direct UDP probing.

## Ports

- UDP `29130`: endpoint probe/report. The service replies with the observed reflexive endpoint.
- HTTP `29131`: health, endpoint lookup/report, and connect-session creation.

## UDP protocol

Endpoint probe:

```json
{"kind":"endpoint_probe","networkId":"network-id","nodeId":"node-id"}
```

Response:

```json
{
  "kind": "endpoint_reflexive",
  "endpoint": {
    "networkId": "network-id",
    "nodeId": "node-id",
    "type": "reflexive",
    "address": "203.0.113.10:40000",
    "reflexive": "203.0.113.10:40000",
    "natType": "unknown"
  }
}
```

## HTTP API

- `GET /healthz`
- `POST /v1/endpoints`
- `GET /v1/endpoints/{networkId}/{nodeId}`
- `POST /v1/connect-sessions`
- `GET /v1/connect-sessions/{sessionId}`
- `GET /v1/stats`

Mutating HTTP requests use `X-Slan-Internal-Token` when `SLAN_INTERNAL_WIRE_TOKEN` is configured with a non-dev value.

Creating a connect session also sends a UDP `connect_session` message to both stored endpoints when both peers are currently visible:

```json
{
  "kind": "connect_session",
  "sessionId": "session-id",
  "networkId": "network-id",
  "nodeId": "local-node-id",
  "peer": {
    "nodeId": "peer-node-id",
    "address": "203.0.113.11:40001",
    "reflexive": "203.0.113.11:40001"
  }
}
```

## Runtime

Current storage is in-memory TTL state:

- endpoint TTL: `SLAN_WIRE_PUNCH_ENDPOINT_TTL_SECONDS`, default `120`
- connect session TTL: `SLAN_WIRE_PUNCH_SESSION_TTL_SECONDS`, default `60`

Redis can be added behind the same store boundary when multi-instance punch coordination is needed.
