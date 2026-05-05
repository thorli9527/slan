# server-wire-relay

`server-wire-relay` is the isolated UDP-only relay data plane for the
WireGuard-shaped stack.

It intentionally supports only one transport:

- `relay_udp`

## Protocol

UDP JSON control messages:

- `ping`
- `attach`
- `forward`
- `detach`

UDP JSON responses:

- `pong`
- `attached`
- `forwarded`
- `packet`
- `detached`
- `error`

## Run

```bash
go run ./cmd/server-wire-relay
```

Default bind address:

- `127.0.0.1:29110`

Default admin address:

- `127.0.0.1:29111`

Docker:

```bash
docker compose -f ../../docker-compose.local.yml up --build server-wire-relay
```

Container environment:

- `SLAN_WIRE_RELAY_LISTEN_ADDR=:29110`
- `SLAN_WIRE_RELAY_ADMIN_LISTEN_ADDR=:29111`
- `SLAN_BIZ_URL=http://server-biz:8080`
- `SLAN_INTERNAL_WIRE_TOKEN=...`
- `SLAN_WIRE_RELAY_REGION_ID=local`
- `SLAN_WIRE_RELAY_NODE_ID=relay-local`
- `SLAN_WIRE_RELAY_HEARTBEAT_SECONDS=30`
- `SLAN_WIRE_TICKET_SECRET=change-me-wire-ticket-secret`
- `SLAN_WIRE_TICKET_SECRETS=change-me-wire-ticket-secret`

When `SLAN_BIZ_URL` and `SLAN_INTERNAL_WIRE_TOKEN` are configured, the node
registers itself with `server-biz` and sends periodic heartbeats. Registration
and heartbeat failures are logged with bounded exponential backoff.

## Admin HTTP

- `GET /healthz`
- `GET /v1/sessions`
- `GET /v1/sessions/{sessionId}`
- `GET /v1/ticket-key-status`
- `GET /metrics`

`/v1/ticket-key-status` exposes key ring configuration state and a consistency
ID only. It never returns ticket secrets.
