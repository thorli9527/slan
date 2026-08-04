# Client Architecture Boundary

This document defines the client-side layering rule for SLAN clients.

## Core Rule

All business HTTP/API requests must go through Rust `client-core-service`.

Flutter/Dart must not:

- call `server-biz` or `/api/app/...` directly
- maintain a second business-state source outside Rust
- add device activation/session/network-message business flows through plugins

Flutter/Dart should only:

- render UI state exposed by Rust local API or embedded API
- dispatch user intent to Rust
- invoke platform execution methods such as VPN, PacketTunnel, and WinTun
- forward platform runtime state back into Rust
- read only platform-local bootstrap defaults such as packaged `SLANControlBaseURL`
  when no runtime override has been persisted yet

## Ownership

Rust `client-core-service` owns:

- all control-plane HTTP access
- device activation, deactivation, token renewal, and session persistence
- MQTT/control transport
- network config generation
- business-state snapshots and business-event stream
- runtime-to-server reporting

Flutter bridge owns:

- selecting local daemon vs embedded Rust entrypoint
- mapping Rust state/event output into UI state
- invoking platform plugins

Platform plugins own:

- Android `VpnService`
- iOS `PacketTunnel`
- macOS/Windows/Linux platform execution
- runtime-state observation from the platform side

## Allowed Dart Networking

Dart networking is only allowed for:

- local daemon transport to `client-core-service`
- explicit test socket traffic for UDP/TCP verification

Dart networking is not allowed for:

- registration
- device activation or deactivation
- token renew
- device runtime reporting
- DNS/ACL/device/network business APIs

## State Source Rule

Business state must come from Rust only:

- `localState`
- `localBusinessEventWatch`
- `localControlStatus`
- `localPlatformNetworkConfig`

Plugin direct reads such as `androidRuntimeState()` and `iosRuntimeState()` are
platform-runtime diagnostics only. They must not become a second business-state
source.

## Testing Rule

Integration and UI tests should prefer the same boundary as production:

- use bridge local API requests
- use Rust local/embedded service methods

Tests should not introduce new direct `/api/app/...` calls in Dart unless the
test is explicitly validating an HTTP client outside Flutter.

## Guard Script

Use the boundary guard before merging client changes:

```bash
bash scripts/check_client_stack_guard.sh
```

The guard currently verifies:

- `client/app_flutter/lib` does not reference `/api/app/...`
- `client/app_flutter/lib` does not introduce direct business HTTP clients
- plugin runtime reads stay limited to bridge/diagnostics layers
- packaged default endpoint wiring stays aligned
- protocol/client contract guard still passes

The audit script is broader and prints where API and network-related usage still
exists across client code, so the remaining allowed surface stays visible during
refactors.

When changing packaged defaults or adding a new client-facing validation script:

- update `client/docs/client-default-endpoints.md`
- source `scripts/lib/client_default_endpoints.sh` from new shell entrypoints
- keep `bash scripts/check_client_stack_guard.sh` green in the same change
