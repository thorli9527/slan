# Protocol Contract Inventory

## Goal

This document tracks the duplicated business contracts that currently exist
across:

- `server/server-biz/api/dto` (backend source-of-truth today)
- `server/server-ui/web/src/ui` (web API contracts)
- `client/app/lib/infra` (Flutter request/response contracts and local models)

The goal is not to force one-step unification. The goal is to make the next
round of protocol-source extraction concrete and low-risk.

## Drift Check

The repository now includes a minimal field-level drift checker for web
transport contracts:

```bash
go run ./scripts/check_protocol_contracts.go --target web
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

Current Flutter scope is intentionally narrower than web:

- Checks request contracts in
  `client/app/lib/infra/api_contracts/request_models.dart`
- Checks only the Flutter response DTOs that are intended to remain close to
  backend transport shape today:
  `AuthResponseDto`, `CompleteAuthCallbackRequestDto`,
  `AuthCallbackStatusResponseDto`, `NodeResponseDto`,
  `ControlPlaneConfigResponseDto`, and `RelayTicketResponseDto`
- Does not currently fail on reduced Flutter projection DTOs such as
  `DeviceResponseDto`, `NetworkDetailResponseDto`, or `BootstrapResponseDto`

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

### Web

- API response contracts: `server/server-ui/web/src/ui/api-contracts.ts`
- UI-local types: `server/server-ui/web/src/ui/ui-models.ts`

### Flutter

- API request contracts: `client/app/lib/infra/api_contracts/request_models.dart`
- API response DTOs: `client/app/lib/infra/control_api_responses/response_dtos.dart`
- DTO -> app model mappers: `client/app/lib/infra/control_api_responses/response_mappers.dart`
- App-local runtime models: `client/app/lib/infra/app_core/models/*`

## Contract Groups

### Auth

Backend:

- `RegisterRequest`
- `LoginRequest`
- `AuthResponse`
- `AuthCallbackStatusResponse`

Web:

- `AuthResponse`
- `AuthMode` is UI-local, not backend contract

Flutter:

- request contracts: `RegisterRequest`, `LoginRequest`
- response DTO: `AuthResponseDto`
- local model: `SessionModel`

Status:

- Request and response field names are aligned across backend, web, and
  Flutter.
- The first canonical slice draft now lives at
  `protocol/contracts/auth-registration.yaml`.

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
- `SwitchNetworkRequest`
- `DeactivateNetworkRequest`
- `JoinNetworkByOwnerEmailRequest`
- `UpdateAttachmentIPRequest`
- `CreateSubnetRequest`
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

Flutter:

- request contracts:
  - `CreateNetworkRequest`
  - `JoinNetworkRequest`
  - `BootstrapRequest` indirectly depends on network ids
- response DTOs:
  - `NetworkSummaryResponseDto`
  - `NetworkDetailResponseDto`
  - `SubnetResponseDto`
  - `NetworkMemberResponseDto`
  - `SubnetAttachmentResponseDto`
- local models:
  - `NetworkModel`
  - `NetworkMemberModel`

Status:

- A canonical slice draft now exists at `protocol/contracts/network.yaml`.
- Web now has explicit transport typings for `NetworkMember`,
  `SubnetAttachment`, `NetworkJoinResult`, and
  `NetworkJoinByOwnerEmailResult`.
- Flutter currently models a reduced network view and folds attachment IP into
  `DeviceModel` / `NetworkMemberModel`.
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

- no explicit bootstrap contract layer yet

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

- Web has no explicit contract for `AuthCallbackStatusResponse`.
- Web `NetworkDetail` contract is a reduced subset of backend `NetworkDetail`,
  which makes later feature expansion easy to forget.
- Flutter request contracts do not yet include:
  - `UpdateNetworkRequest`
  - `DeactivateNetworkRequest`
  - `JoinNetworkByOwnerEmailRequest`
  - `CreateSubnetRequest`
  - `AttachDeviceRequest`
  - `UpdateAttachmentIPRequest`
  because current mobile/desktop HTTP client does not expose those flows.

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

1. Add missing web contract types for `AuthCallbackStatusResponse`,
   `ControlPlaneConfig`, and `RelayTicket` if the web console will consume them.
2. Add explicit Flutter request contract files for network-management requests
   not yet represented.
3. Decide whether shared protocol artifacts will be:
   - generated from Go DTOs
   - manually maintained in a new repo directory such as `protocol/`
4. Once a canonical source is chosen, migrate one vertical slice first:
   `auth + registration`.
