# Client Architecture Boundary

This document defines the client-side layering rule for SLAN clients.

## Core Rule

All business HTTP/API requests must go through Rust `client-core-service`.

Flutter/Dart must not:

- call `server-biz` or `/api/app/...` directly
- maintain a second business-state source outside Rust
- add new login/session/device/network-message business flows through plugins

Flutter/Dart should only:

- render UI state exposed by Rust local API or embedded API
- dispatch user intent to Rust
- invoke platform execution methods such as VPN, PacketTunnel, WinTun, browser open
- forward platform runtime state back into Rust
- read only platform-local bootstrap defaults such as packaged `SLANControlBaseURL`
  or `SLANWebConsoleURL` when no runtime override has been persisted yet

## Ownership

Rust `client-core-service` owns:

- all control-plane HTTP access
- login, logout, renew, device registration, and session persistence
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
- opening external browser URLs through platform shell commands

Dart networking is not allowed for:

- registration
- login
- logout
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

- `client_v2/app_flutter/lib` does not reference `/api/app/...`
- `client_v2/app_flutter/lib` does not introduce direct business HTTP clients
- plugin runtime reads stay limited to bridge/diagnostics layers
- packaged default endpoint wiring stays aligned
- protocol/client contract guard still passes

The audit script is broader and prints where API and network-related usage still
exists across client code, so the remaining allowed surface stays visible during
refactors.

When changing packaged defaults or adding a new client-facing validation script:

- update `client_v2/docs/client-default-endpoints.md`
- source `scripts/lib/client_default_endpoints.sh` from new shell entrypoints
- keep `bash scripts/check_client_stack_guard.sh` green in the same change

## Login Surface

Current desktop and mobile clients log in with username and password against the
configured control-plane URL. Flutter owns the form and sends the command to
Rust; it must not derive or open a separate Web Console URL.

Legacy `openWebConsole` commands remain temporarily for released-client
compatibility. They are not part of the current UI contract and should not be
used by new entrypoints.
