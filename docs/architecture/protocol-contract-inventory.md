# Protocol Contract Inventory

## Goal

This document tracks the duplicated business contracts that currently exist
across:

- `server/server-biz/api/dto` (backend source-of-truth today)
- `server/server-ui/web/src/ui` (web API contracts)
- `client/app/lib/infra` (Flutter request/response contracts and local models)
- `client/app_core/crates/controller-client` (Rust controller transport DTOs)

The goal is not to force one-step unification. The goal is to make the next
round of protocol-source extraction concrete and low-risk.

## Drift Check

The repository now includes a minimal field-level drift checker for web
transport contracts. Targets can be passed one by one or repeated in a single
run:

```bash
go run ./scripts/check_protocol_contracts.go --target web
go run ./scripts/check_protocol_contracts.go --target web --target flutter --target rust-controller --target go-server --target openapi --target protobuf --target http-routes
```

Current scope:

- Reads canonical slice drafts from `protocol/contracts/*.yaml`
- Compares matching contract names against
  `server/server-ui/web/src/ui/api-contracts.ts`
- Verifies that every field listed in the canonical contract also exists in the
  matching exported web transport type

Flutter drift check is also available:

```bash
go run ./scripts/check_protocol_contracts.go --target flutter
```

Current Flutter scope:

- Checks request contracts in
  `client/app/lib/infra/api_contracts/request_models.dart`
- Checks Flutter response DTOs that mirror backend transport shape, including
  auth, device/node, network detail/member/assignment/join, bootstrap,
  control-plane, relay, route, peer, and subnet attachment DTOs.
- App-local projection models such as `NetworkModel` can remain reduced, but
  the transport DTO layer is expected to keep the protocol fields.

Rust controller-client drift check is also available:

```bash
go run ./scripts/check_protocol_contracts.go --target rust-controller
```

Current Rust scope:

- Checks request/response DTOs in
  `client/app_core/crates/controller-client/src/dto.rs`
- Applies known DTO name aliases such as `Device -> DeviceDto`,
  `DeviceBootstrap -> BootstrapDeviceDto`, and `DNSConfig -> DnsConfigDto`
- Respects explicit serde field renames such as the control-plane endpoint
  JSON field `type`

Go server DTO drift check is also available:

```bash
go run ./scripts/check_protocol_contracts.go --target go-server
```

Current Go server scope:

- Checks JSON tags on structs in `server/server-biz/api/dto`
- Expands embedded DTO structs such as `NetworkDetail` embedding `Network`
- Verifies that backend request/response DTOs still carry every canonical
  protocol field

OpenAPI drift check is also available:

```bash
go run ./scripts/check_protocol_contracts.go --target openapi
```

Current OpenAPI scope:

- Checks component schema properties in `protocol/openapi/phase1.yaml`
- Expands `allOf` schema references such as `NetworkDetail -> Network`
- Verifies that public API documentation exposes every canonical protocol
  field

HTTP route drift check is also available:

```bash
go run ./scripts/check_protocol_contracts.go --target http-routes
```

Current HTTP route scope:

- Parses public Go route files in `server/server-biz/api/http`
- Converts Gin parameters such as `:networkId` into OpenAPI
  `{networkId}` form
- Verifies that OpenAPI paths and methods match the public router, including
  health, diagnostics, and MQTT control entries

Protobuf drift check is also available:

```bash
go run ./scripts/check_protocol_contracts.go --target protobuf
```

Current protobuf scope:

- Checks message fields in `protocol/protobuf/control.proto`
- Converts proto snake_case field names to canonical camelCase names
- Only validates contracts that already exist as protobuf control-channel
  messages, such as `NetworkMap`, `Peer`, `Route`, `RelayRegion`, and
  `RelayTicket`

See also:

- [protocol-contract-checks.md](./protocol-contract-checks.md)

## Current Sources

### Backend

- Access: `server/server-biz/api/dto/types_business_access.go`
- Registration: `server/server-biz/api/dto/types_business_registration.go`
- Network: `server/server-biz/api/dto/types_business_network.go`
- Control / bootstrap / relay: `server/server-biz/api/dto/types_business_control.go`

### Canonical Slice Drafts

- `protocol/contracts/auth-registration.yaml`
- `protocol/contracts/control-plane.yaml`
- `protocol/contracts/network.yaml`
- `protocol/contracts/system.yaml`

### Web

- API response contracts: `server/server-ui/web/src/ui/api-contracts.ts`
- UI-local types: `server/server-ui/web/src/ui/ui-models.ts`

### Flutter

- API request contracts: `client/app/lib/infra/api_contracts/request_models.dart`
- API response DTOs: `client/app/lib/infra/control_api_responses/response_dtos.dart`
- DTO -> app model mappers: `client/app/lib/infra/control_api_responses/response_mappers.dart`
- App-local runtime models: `client/app/lib/infra/app_core/models/*`

### Rust Controller Client

- Request/response DTOs: `client/app_core/crates/controller-client/src/dto.rs`
- HTTP endpoint calls: `client/app_core/crates/controller-client/src/client.rs`
- Public controller API trait: `client/app_core/crates/controller-client/src/api.rs`

## Contract Groups

### Auth

Backend:

- `RegisterRequest`
- `LoginRequest`
- `RefreshTokenRequest`
- `AuthResponse`
- `AuthCallbackStatusResponse`

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
  `server/server-biz/docs/public-error-codes.md`.
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
  `server/server-biz/api/dto`
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
