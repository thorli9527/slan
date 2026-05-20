# Protocol Contract Inventory

## Goal

This document tracks the duplicated business contracts that currently exist
across:

- `server/service-biz/internal/biz` (backend source-of-truth today)
- `server/service-ui/src/ui` (Web Console API contracts)
- `client_v2/app_flutter/lib/bridge` (Flutter local service contracts)
- `client_v2/rust/crates/client-core-service/src` (Rust control-plane contracts)

The goal is not to force one-step unification. The goal is to make the next
round of protocol-source extraction concrete and low-risk.

## Drift Check

The repository now includes a lightweight protocol-contract entry point:

```bash
make protocol-contract-check
```

Current scope:

- Verifies legacy `-new` project names and old business-control paths are not
  present in active source, docs, protocol metadata, or scripts.
- Reads canonical slice drafts from `protocol/contracts/*.yaml`
- Verifies required contract and OpenAPI files exist.
- Runs `go test ./...` for `server/service-biz`.
- Runs `cargo check -p client-core-service`.
- Verifies key Web Console and Flutter bridge contract files exist.

Full drift checking against generated schema is still a production hardening
item. Until that exists, full functional verification should also run
`flutter analyze`, `flutter test`, `cargo test --workspace`, Web Console
`npm run build`, and the service smoke scripts.

## Current Sources

### Backend

- Models: `server/service-biz/internal/biz/models.go`
- Public API handlers: `server/service-biz/internal/biz/server.go`
- MQTT API and auth: `server/service-biz/internal/biz/server_mqtt.go`
- Wire internal API: `server/service-biz/internal/biz/wire_server.go`
- Relay tickets: `server/service-biz/internal/biz/relay.go`

### Canonical Slice Drafts

- `protocol/contracts/auth-registration.yaml`
- `protocol/contracts/control-plane.yaml`
- `protocol/contracts/network.yaml`
- `protocol/contracts/system.yaml`

### Web

- API service: `server/service-ui/src/ui/app-api.service.ts`
- Web auth flow: `server/service-ui/src/ui/app-auth-flow.ts`
- UI models: `server/service-ui/src/ui/app.models.ts`

### Flutter

- Local service bridge: `client_v2/app_flutter/lib/bridge/client_core_bridge.dart`
- Local service API: `client_v2/app_flutter/lib/bridge/client_core_local_service.dart`
- Commands: `client_v2/app_flutter/lib/bridge/client_commands.dart`
- View state: `client_v2/app_flutter/lib/bridge/client_view_state.dart`

### Rust Controller Client

- HTTP control-plane client: `client_v2/rust/crates/client-core-service/src/control_plane.rs`
- Session persistence and renewal: `client_v2/rust/crates/client-core-service/src/session_store.rs`
- Control transport worker: `client_v2/rust/crates/client-core-service/src/control_transport_worker.rs`
- Embedded mobile entrypoint: `client_v2/rust/crates/client-core-service/src/embedded.rs`

## Contract Groups

### Auth

Backend:

- `RegisterRequest`
- `LoginRequest`
- `RefreshTokenRequest`
- `AuthResponse`

Web:

- `AuthResponse`
- `AuthMode` is UI-local, not backend contract

Flutter:

- request contracts: `RegisterRequest`, `LoginRequest`, `RefreshTokenRequest`
- response DTO: `AuthResponseDto`
- local model: `SessionModel`

Status:

- Request and response field names are aligned across backend, web, and
  Flutter.
- The first canonical slice draft now lives at
  `protocol/contracts/auth-registration.yaml`.

### System

Backend:

- `ErrorResponse`

Web:

- `ErrorResponse`

Flutter:

- response DTO: `ErrorResponseDto`

Rust controller:

- response DTO: `ErrorResponseDto`

Status:

- Public HTTP errors use the stable `code + message` shape documented in
  `server/service-biz/docs/public-error-codes.md`.
- A canonical slice draft now exists at `protocol/contracts/system.yaml`.

### Registration

Backend:

- `RegisterDeviceRequest`
- `Device`
- `RegisterNodeRequest`
- `Node`

Web:

- `Device`

Flutter:

- request contracts: `RegisterDeviceRequest`, `RegisterNodeRequest`
- response DTOs: `DeviceResponseDto`, `NodeResponseDto`
- local models: `DeviceModel`, `NodeModel`

Status:

- Device and node request/response fields are mostly aligned.
- Flutter local `DeviceModel` is intentionally not identical to backend
  `Device`. It adds `virtualIp` and drops `networkIds`.
- Web does not currently expose `Node` contract directly.

### Network

Backend:

- `CreateNetworkRequest`
- `UpdateNetworkRequest`
- `UpdateNetworkJoinKeyRequest`
- `UpdateNetworkDNSRequest`
- `SwitchNetworkRequest`
- `DeactivateNetworkRequest`
- `JoinNetworkByOwnerEmailRequest`
- `UpdateAttachmentIPRequest`
- `UpdateAttachmentRemarkRequest`
- `AttachDeviceRequest`
- `JoinNetworkRequest`
- `Network`
- `NetworkHome`
- `Subnet`
- `NetworkMember`
- `SubnetAttachment`
- `NetworkJoinResult`
- `NetworkJoinByOwnerEmailResult`
- `NetworkDetail`
- `NetworkAssignment`

Web:

- `Network`
- `NetworkHome`
- `NetworkDetail`
- `Subnet`
- `NetworkAssignment`
- `NetworkJoinResult`
- `NetworkJoinByOwnerEmailResult`

Flutter:

- request contracts:
  - `CreateNetworkRequest`
  - `UpdateNetworkRequest`
  - `UpdateNetworkDNSRequest`
  - `SwitchNetworkRequest`
  - `DeactivateNetworkRequest`
  - `JoinNetworkByOwnerEmailRequest`
  - `JoinNetworkRequest`
  - `UpdateAttachmentRemarkRequest`
  - `BootstrapRequest` indirectly depends on network ids
- response DTOs:
  - `NetworkSummaryResponseDto`
  - `NetworkDetailResponseDto`
  - `SubnetResponseDto`
  - `NetworkMemberResponseDto`
  - `SubnetAttachmentResponseDto`
  - `NetworkAssignmentResponseDto`
  - `NetworkJoinResultResponseDto`
  - `NetworkJoinByOwnerEmailResultResponseDto`
- local models:
  - `NetworkModel`
  - `NetworkMemberModel`

Status:

- A canonical slice draft now exists at `protocol/contracts/network.yaml`.
- Web now has explicit transport typings for `NetworkMember`,
  `SubnetAttachment`, `NetworkJoinResult`, and
  `NetworkJoinByOwnerEmailResult`.
- Flutter now keeps transport DTOs aligned for join, switch, activate,
  deactivate, assignment, and owner-email join flows; UI-facing models remain
  intentionally smaller projections.
- Backend `NetworkDetail` includes `subnets` and `members`; web currently keeps
  those in separate fetches from `/subnets` and `/assignments` as well.

### Bootstrap / Control Plane

Backend:

- `CreateControlSessionRequest`
- `BootstrapRequest`
- `DeviceBootstrap`
- `ControlPlaneConfig`
- `BootstrapResponse`
- `ControlSessionResponse`
- `RelayTicket`

Web:

- explicit transport types exist for `ControlPlaneConfig`,
  `BootstrapResponse`, `RelayTicket`, and related runtime DTOs, although the
  console does not yet consume all of them as first-class runtime flows.

Flutter:

- request contracts:
  - `BootstrapRequest`
  - `RelayTicketRequest`
- response DTOs:
  - `BootstrapResponseDto`
  - `DeviceBootstrapResponseDto`
  - `ControlPlaneConfigResponseDto`
  - `RelayTicketResponseDto`
  - `NetworkMapResponseDto`
- local models:
  - `BootstrapModel`
  - `ControlPlaneConfigModel`
  - `RelayTicketModel`

Status:

- Flutter mirrors bootstrap-related backend DTOs much more closely than web.
- Web now has explicit transport type definitions for `ControlPlaneConfig`,
  `BootstrapResponse`, and `RelayTicket`, even though the console does not yet
  consume all of them as first-class runtime flows.

### Relay Topology

Backend:

- `RelayConfig`
- `RelayCountry`
- `RelayCity`
- `RelayCluster`
- `RelayNode`

Web:

- no explicit relay topology contracts today

Flutter:

- response DTOs:
  - `RelayConfigResponseDto`
  - `RelayCountryResponseDto`
  - `RelayCityResponseDto`
  - `RelayClusterResponseDto`
  - `RelayNodeResponseDto`
- local models:
  - `RelayConfigModel`
  - `RelayCountryModel`
  - `RelayCityModel`
  - `RelayClusterModel`
  - `RelayNodeModel`

Status:

- Flutter already has a nearly 1:1 relay contract mirror.
- Web has no shared relay contract layer yet.

## Confirmed Drift

### Intentional Drift

- Flutter `DeviceModel` adds `virtualIp` for local UI/runtime convenience.
- Flutter `NetworkModel` is a reduced projection, not backend `NetworkDetail`.
- Web `AuthMode` is UI-local and should not be promoted into shared protocol.

### Unintentional or Risky Drift

- Web still treats some owner administration responses as refresh-driven UI
  updates even though the transport layer now parses returned entities.
- Flutter `NetworkModel` intentionally remains a reduced app projection; keep
  expanding `*ResponseDto` transport coverage instead of adding backend-only
  fields directly to UI models.

## Recommended Unification Order

### Phase 1

Make backend DTOs the canonical schema inventory without changing runtime code.

- Create a generated or manually curated protocol index from
  `server/service-biz/internal/biz/models.go` and handler request/response
  shapes in `server/service-biz/internal/biz/server.go`
- Keep frontend-local projections separate

### Phase 2

Unify request contracts first.

- Auth
- Registration
- Network create/join/activate/deactivate
- Bootstrap
- Relay ticket

Reason:

- Request contracts are smaller and more stable than response projections.

### Phase 3

Unify transport DTOs second.

- `AuthResponse`
- `Device`
- `Node`
- `Network`
- `NetworkHome`
- `BootstrapResponse`
- `RelayTicket`

Reason:

- These are used across multiple clients and are close to backend DTOs already.

### Phase 4

Keep local runtime/view models separate.

- Flutter `SessionModel`, `DeviceModel`, `NetworkModel`, `BootstrapModel`
- web UI-local state types

Reason:

- These are projections and should not be forced into backend shape.

## Next Concrete Tasks

1. Decide whether shared protocol artifacts will be:
   - generated from Go DTOs
   - manually maintained in a new repo directory such as `protocol/`
2. Once a canonical source is chosen, migrate one vertical slice first:
   `auth + registration`.
3. Keep adding protocol drift coverage when a new public DTO becomes part of
   the app, app-core, web, OpenAPI, or protobuf boundary.
