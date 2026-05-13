# server-wire

`server-wire` is an isolated control service for the WireGuard-shaped network
model. It does not reuse `server-biz` internals so protocol
and path logic can evolve without mixing with the legacy control plane.

For the matching UDP-only relay data plane, use the separate
`server/server-wire-relay` project.

## Scope

The service keeps the path model intentionally small:

- `lan_udp`
- `ipv6_udp`
- `direct_udp`
- `relay_udp`
- `derp_tcp_tls_443`

Supported control behaviors:

- path probing and scoring
- fast path reselection
- MTU probing policy
- IPv6 preference with safe fallback
- LAN-first direct preference
- endpoint roaming
- relay ticket renewal planning
- per-peer keepalive policy
- DERP fallback planning

## Run

```bash
go run ./cmd/server-wire
```

Default listen address:

- `127.0.0.1:29100`

Remote Docker:

```bash
cd ../..
sh scripts/setup_remote_docker_context.sh
.tmp/remote-deploy/deploy_to_47.245.40.231.sh
```

Container environment:

- `SLAN_WIRE_LISTEN_ADDR=:29100`
- `SLAN_WIRE_BIZ_INTERNAL_URL=http://server-biz:8080`
- `SLAN_INTERNAL_WIRE_TOKEN=change-me-wire-internal-token`
- `SLAN_WIRE_TICKET_SECRET=change-me-wire-ticket-secret`
- `SLAN_WIRE_TICKET_SECRETS=change-me-wire-ticket-secret`
- `SLAN_WIRE_POSTGRES_DSN=postgres://postgres:change-me-postgres-password@postgres:5432/slan?sslmode=disable`

When `SLAN_WIRE_BIZ_INTERNAL_URL` is set, peer registration is authorized
through `server-biz /internal/wire/peers/{peerId}/authz`. The business control
plane response overrides client-provided `networkId`, `nodeId`, `virtualIps`,
and `allowedIps`.

When `SLAN_WIRE_POSTGRES_DSN` is set, runtime state is stored in Postgres.
If Postgres is unavailable at startup, `server-wire` falls back to the in-memory
store for isolated development. The remote Docker stack sets the DSN by default.

Ticket key rotation:

- `SLAN_WIRE_TICKET_SECRET` is the current signing key.
- `SLAN_WIRE_TICKET_SECRETS` is a comma-separated verification key ring.
- During rotation, put the new key first and keep previous keys until all
  outstanding relay / DERP tickets expire.

## Endpoints

- `GET /healthz`
- `POST /v1/peers/register`
- `POST /v1/peers/endpoints`
- `POST /v1/peers/path-health`
- `POST /v1/peers/active-path`
- `POST /v1/relay/tickets`
- `GET /v1/derp/map`
- `POST /v1/derp/tickets`
- `POST /v1/path-plan`

`/v1/path-plan` uses the stored peer state and returns the preferred path,
fallback order, keepalive policy, MTU plan, roaming policy, and relay ticket
renewal hints.

## Suggested Flow

1. Register peer capabilities and identity with `/v1/peers/register`
2. Push current endpoint view to `/v1/peers/endpoints`
3. Push path probes to `/v1/peers/path-health`
4. Ask `/v1/path-plan` for the selected path order
5. Update the chosen active path through `/v1/peers/active-path`
6. Issue or renew relay fallback credentials with `/v1/relay/tickets`
