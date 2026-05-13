# server-wire-derp

`server-wire-derp` is the isolated DERP-style final fallback data plane for the
WireGuard-shaped stack.

It provides a long-lived TCP control/data channel today and is intended to sit
behind TLS on port 443 in production.

## Run

```bash
go run ./cmd/server-wire-derp
```

Default listen addresses:

- data: `127.0.0.1:29120`
- admin: `127.0.0.1:29121`

Remote Docker:

```bash
cd ../..
sh scripts/setup_remote_docker_context.sh
.tmp/remote-deploy/deploy_to_47.245.40.231.sh
```

Container environment:

- `SLAN_WIRE_DERP_LISTEN_ADDR=:29120`
- `SLAN_WIRE_DERP_ADMIN_LISTEN_ADDR=:29121`
- `SLAN_BIZ_URL=http://server-biz:8080`
- `SLAN_INTERNAL_WIRE_TOKEN=...`
- `SLAN_WIRE_DERP_REGION_ID=local`
- `SLAN_WIRE_DERP_NODE_ID=derp-local`
- `SLAN_WIRE_DERP_HEARTBEAT_SECONDS=30`
- `SLAN_WIRE_TICKET_SECRET=change-me-wire-ticket-secret`
- `SLAN_WIRE_TICKET_SECRETS=change-me-wire-ticket-secret`

When `SLAN_BIZ_URL` and `SLAN_INTERNAL_WIRE_TOKEN` are configured, the node
registers itself with `server-biz` and sends periodic heartbeats. Registration
and heartbeat failures are logged with bounded exponential backoff.

## Admin HTTP

- `GET /healthz`
- `GET /v1/connections`
- `GET /v1/connections/{peerId}`
- `GET /v1/sessions`
- `GET /v1/sessions/{sessionId}`
- `GET /v1/regions`
- `GET /v1/ticket-key-status`
- `GET /metrics`

`/v1/ticket-key-status` exposes key ring configuration state and a consistency
ID only. It never returns ticket secrets.
