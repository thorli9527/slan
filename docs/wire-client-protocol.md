# SLAN Wire Client Protocol

This document is the client-facing contract for the new WireGuard-shaped
networking stack.

## Services

- `service-biz`: business identity, device, node, network, and IP ownership
- `server-wire`: runtime control plane and path planning
- `server-wire-relay`: UDP relay fallback for `relay_udp`
- `server-wire-derp`: TCP/TLS 443 final fallback for `derp_tcp_tls_443`

## Path Types

Clients must treat path order as a control-plane recommendation, not as a
hardcoded constant. The default order is:

- `lan_udp`
- `ipv6_udp`
- `direct_udp`
- `relay_udp`
- `derp_tcp_tls_443`

`derp_tcp_tls_443` is the final fallback path. It should be selected only when
direct and UDP relay paths are unavailable or unhealthy.

## server-wire HTTP

### Register Peer

`POST /v1/peers/register`

Request:

```json
{
  "peer": {
    "peerId": "peer-a",
    "networkId": "net-a",
    "nodeId": "node-a",
    "publicKey": "base64-or-hex-public-key",
    "virtualIps": ["100.64.0.10"],
    "allowedIps": ["100.64.0.10/32"],
    "supportsLanDirect": true,
    "supportsIpv6Direct": true,
    "supportsDirectUdp": true,
    "supportsRelayUdp": true,
    "supportsDerpTcpTls443": true,
    "preferLan": true,
    "preferIpv6": true,
    "allowEndpointRoaming": true,
    "allowFastReselection": true,
    "allowRelayTicketRenewal": true
  }
}
```

Response:

```json
{
  "peer": {
    "peerId": "peer-a",
    "networkId": "net-a",
    "nodeId": "node-a",
    "supportsRelayUdp": true,
    "supportsDerpTcpTls443": true,
    "updatedAt": 1777968000000
  }
}
```

### Report Endpoints

`POST /v1/peers/endpoints`

Request:

```json
{
  "peerId": "peer-a",
  "endpoints": [
    {
      "kind": "lan",
      "address": "192.168.1.10",
      "port": 51820,
      "reachable": true,
      "observedAt": 1777968000000
    },
    {
      "kind": "ipv6",
      "address": "2408::1",
      "port": 51820,
      "reachable": true,
      "observedAt": 1777968000000
    }
  ]
}
```

### Report Path Health

`POST /v1/peers/path-health`

Request:

```json
{
  "peerId": "peer-a",
  "probes": [
    {
      "path": "lan_udp",
      "reachable": true,
      "rttMs": 3,
      "lossPpm": 0,
      "jitterMs": 1,
      "mtu": 1420,
      "observedAt": 1777968000000
    },
    {
      "path": "derp_tcp_tls_443",
      "reachable": true,
      "rttMs": 90,
      "lossPpm": 0,
      "jitterMs": 8,
      "mtu": 1240,
      "observedAt": 1777968000000
    }
  ]
}
```

### Report DERP Health

`POST /v1/peers/derp-health`

Request:

```json
{
  "peerId": "peer-a",
  "samples": [
    {
      "regionId": "cn-east",
      "nodeId": "derp-cn-east-1",
      "reachable": true,
      "rttMs": 60,
      "observedAt": 1777968000000
    }
  ]
}
```

### Get Path Plan

`POST /v1/path-plan`

Request:

```json
{
  "peerId": "peer-a"
}
```

Response:

```json
{
  "preferredPath": "lan_udp",
  "fallbackOrder": [
    "lan_udp",
    "ipv6_udp",
    "direct_udp",
    "relay_udp",
    "derp_tcp_tls_443"
  ],
  "scoredPaths": [
    {
      "path": "lan_udp",
      "score": 3,
      "reason": "lan direct preferred",
      "mtu": 1420,
      "primary": true
    }
  ],
  "keepalive": {
    "intervalSecs": 0,
    "mode": "idle"
  },
  "mtu": {
    "probeRequired": false,
    "targetMtu": 1420
  },
  "roaming": {
    "apply": false
  },
  "relayTicket": {
    "renewRequired": false
  },
  "derpCandidates": [
    {
      "regionId": "cn-east",
      "nodeId": "derp-cn-east-1",
      "host": "derp-cn-east-1.slan.local",
      "port": 443
    }
  ],
  "fastReselection": false,
  "ipv6Preferred": true,
  "lanDirectPreferred": true
}
```

### Issue UDP Relay Ticket

`POST /v1/relay/tickets`

Request:

```json
{
  "peerId": "peer-a",
  "ttlSeconds": 300,
  "renewAfterMs": 60000
}
```

Response:

```json
{
  "ticket": {
    "ticketId": "relay-peer-a-1777968000000",
    "peerId": "peer-a",
    "sessionId": "relay-session-peer-a-1777968000000",
    "path": "relay_udp",
    "present": true,
    "expiresAt": "2026-05-05T07:10:00Z",
    "expiresInMs": 300000,
    "renewAfterMs": 60000,
    "signature": "hex-hmac-sha256"
  }
}
```

`signature` is HMAC-SHA256 over `ticketId|peerId|sessionId|path|expiresAt`.
`server-wire`, `server-wire-relay`, and `server-wire-derp` must share the
same ticket key ring. `SLAN_WIRE_TICKET_SECRET` is the signing key and
`SLAN_WIRE_TICKET_SECRETS` is the accepted validation key ring. During
rotation, put the new key first and set `SLAN_WIRE_TICKET_SECRET` to that
same first key. Keep previous keys until all short-lived tickets have expired,
then remove retired keys from the ring.

### Get DERP Map

`GET /v1/derp/map`

Response:

```json
{
  "map": {
    "preferredRegionId": "cn-east",
    "regions": [
      {
        "regionId": "cn-east",
        "name": "China East",
        "nodes": [
          {
            "regionId": "cn-east",
            "nodeId": "derp-cn-east-1",
            "host": "derp-cn-east-1.slan.local",
            "port": 443
          }
        ]
      }
    ]
  }
}
```

### Issue DERP Ticket

`POST /v1/derp/tickets`

Request:

```json
{
  "peerId": "peer-a",
  "regionId": "cn-east",
  "nodeId": "derp-cn-east-1",
  "ttlSeconds": 300,
  "renewAfterMs": 60000
}
```

Response:

```json
{
  "ticket": {
    "ticketId": "derp-peer-a-1777968000000",
    "peerId": "peer-a",
    "networkId": "net-a",
    "path": "derp_tcp_tls_443",
    "regionId": "cn-east",
    "nodeId": "derp-cn-east-1",
    "expiresAt": "2026-05-05T07:10:00Z",
    "expiresInMs": 300000,
    "renewAfterMs": 60000,
    "signature": "hex-hmac-sha256"
  }
}
```

`signature` is HMAC-SHA256 over
`ticketId|peerId|networkId|path|regionId|nodeId|expiresAt`.

## server-wire-relay UDP

`server-wire-relay` uses UDP JSON datagrams.

### Attach

Request:

```json
{
  "kind": "attach",
  "participantId": "node-a",
  "transport": "relay_udp",
  "ticket": {
    "ticketId": "relay-peer-a-1777968000000",
    "peerId": "peer-a",
    "sessionId": "session-a-b",
    "path": "relay_udp",
    "expiresAt": "2026-05-05T07:10:00Z",
    "signature": "hex-hmac-sha256"
  }
}
```

Response:

```json
{
  "kind": "attached",
  "sessionId": "session-a-b",
  "participantId": "node-a",
  "peerParticipantId": "node-b"
}
```

### Forward

Request:

```json
{
  "kind": "forward",
  "sessionId": "session-a-b",
  "participantId": "node-a",
  "payload": "base64-encoded-by-json"
}
```

Peer response:

```json
{
  "kind": "packet",
  "sessionId": "session-a-b",
  "participantId": "node-a",
  "payload": "base64-encoded-by-json"
}
```

Sender response:

```json
{
  "kind": "forwarded",
  "sessionId": "session-a-b",
  "participantId": "node-b",
  "bytesForwarded": 1200
}
```

## server-wire-derp TCP

`server-wire-derp` uses newline-delimited JSON frames over a long-lived TCP
connection. Production deployment should place this service behind TLS on port
443.

### Connect

Request:

```json
{
  "kind": "connect",
  "peerId": "peer-a",
  "nodeId": "node-a",
  "regionId": "cn-east",
  "ticket": {
    "ticketId": "derp-peer-a-1777968000000",
    "peerId": "peer-a",
    "networkId": "net-a",
    "path": "derp_tcp_tls_443",
    "regionId": "cn-east",
    "nodeId": "derp-cn-east-1",
    "expiresAt": "2026-05-05T07:10:00Z",
    "signature": "hex-hmac-sha256"
  }
}
```

Response:

```json
{
  "kind": "connected",
  "sessionId": "derp-peer-a",
  "peerId": "peer-a",
  "regionId": "cn-east",
  "nodeId": "node-a",
  "renewAfterMs": 150000
}
```

### Send

Request:

```json
{
  "kind": "send",
  "sessionId": "derp-peer-a",
  "targetPeerId": "peer-b",
  "payload": "base64-encoded-by-json"
}
```

Target peer receives:

```json
{
  "kind": "recv",
  "sessionId": "derp-peer-a",
  "sourcePeerId": "peer-a",
  "payload": "base64-encoded-by-json"
}
```

Sender receives:

```json
{
  "kind": "sent",
  "sessionId": "derp-peer-a",
  "bytesForwarded": 1200
}
```

## Admin Endpoints

`server-wire-relay` admin:

- `GET /healthz`
- `GET /v1/sessions`
- `GET /v1/sessions/{sessionId}`
- `GET /metrics`

`server-wire-derp` admin:

- `GET /healthz`
- `GET /v1/connections`
- `GET /v1/connections/{peerId}`
- `GET /v1/sessions`
- `GET /v1/sessions/{sessionId}`
- `GET /v1/regions`
- `GET /metrics`
