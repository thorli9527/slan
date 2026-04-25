# Client Core Flow

This note defines the recommended public client flow. Ops-only behavior is out
of scope here.

## New User Creates A Network

1. `POST /auth/register` or `POST /auth/login`
2. `POST /auth/refresh` when the access token expires
3. `POST /devices/register`
4. `POST /nodes/register`
5. `GET /networks/home`
6. If `ownedNetwork` is empty, call `POST /networks` with `bindDeviceId`.
   If the server already created an owned network during registration, reuse
   the `ownedNetwork` returned by `/networks/home`.
7. `POST /bootstrap` with the selected `nodeId` and `networkId`
8. Use the returned control session token to open the control WebSocket.
9. Call `POST /relay/tickets` only when direct path setup fails and a relay
   fallback is needed.

`POST /control/sessions` is still available when a client already has device,
node, and network context and only needs to refresh the control-plane session.
The normal public client flow should prefer `POST /bootstrap` because it returns
the control token, WebSocket config, device attachment view, and initial
NetworkMap together.

Implementation checkpoints:

- Flutter entry: `NetworksPage` selects the active network, while
  `HomePage` enables or disables the selected network.
- App coordinator: `AppCoreCoordinator` keeps `selectedNetworkId` and passes it
  to activate, deactivate, bootstrap, and control refresh flows.
- Native bridge: `joinNetwork`, `joinNetworkByOwnerEmail`,
  `joinNetworkByKey`, `switchNetwork`, and `activateNetwork` return
  `NetworkJoinResult`.
- Controller client: join, switch, and activate parse the server
  `member + attachment` response so the app can keep `networkId`,
  `attachmentId`, and virtual IP.

## New User Joins An Existing Network

1. `POST /auth/register` or `POST /auth/login`
2. `POST /devices/register`
3. `POST /nodes/register`
4. Use one discovery path:
   - `POST /networks/join-by-owner-email` when the user typed an owner email.
   - `POST /networks/join-by-key` when the user has an explicit join key.
5. Use the returned `networkId` and attachment result as the active network.
6. `POST /bootstrap` with the joined `networkId` and local `nodeId`.
7. Use `POST /relay/tickets` only after direct connection attempts fail.

If the user provides an alias, the app updates the joined attachment remark:

1. Use the returned `attachmentId` from the join result.
2. Call `PUT /networks/{networkId}/attachments/{attachmentId}/remark`.
3. The server accepts the update when the caller is the network owner or owns
   the device behind that attachment.
4. Refresh `GET /networks` so the member list displays the alias.

The web console follows the same rule for its owner-email and join-key entry
points: alias is optional, but when provided it is persisted through the
attachment remark API before the workspace is refreshed.

## Join Semantics

- `join-by-owner-email` discovers the target network by owner email and then
  performs the same membership and attachment work as `join`.
- `join-by-key` discovers the target network by join key and then performs the
  same membership and attachment work as `join`.
- `join` is the explicit form used when the client already knows `networkId`.
- `activate` is an idempotent reactivation path for a device that is already a
  member or needs its active attachment restored.
- All join paths should be idempotent for the same device and network: repeated
  calls should return the existing member and attachment when possible.

## Switching Networks

Switching networks is an explicit control-plane operation. It records the
target network for the current device and returns the same join result shape as
join and activate:

1. `GET /networks` loads all joined networks.
2. The app calls `POST /networks/{networkId}/switch` with the current
   `deviceId`.
3. The app stores the returned `networkId` in `selectedNetworkId` and refreshes
   `GET /networks`.
4. `Enable Network` calls `POST /networks/{networkId}/activate`, then
   `POST /bootstrap` with the same `networkId`.
5. `Disable Network` brings down the local tunnel, then calls
   `POST /networks/{networkId}/deactivate`.
6. Control sync and connection fallback must continue using the selected
   network from the current bootstrap/network map.

The web console also uses `GET /networks` for its switch list, then calls the
same switch and activate endpoints as the desktop app flow.

## Bootstrap Preconditions

`POST /bootstrap` requires:

- authenticated user
- existing `nodeId`
- existing `networkId`
- node owned by the authenticated user
- device is an active member of the network
- device has an active network attachment with a virtual IP

Expected failures:

- missing `nodeId` or `networkId`: `400 INVALID_ARGUMENT`
- unknown node or network: `404 NOT_FOUND`
- node owned by another user: `403 FORBIDDEN`
- device is not an active member or has no active attachment: `403 FORBIDDEN`

## Validation Matrix

- Server business API: `go test ./...` under `server/server-biz`.
- Server HTTP integration flow: `go test ./...` under
  `server/tests/go/server-biz-test`.
- Rust app core and bridge: `cargo test` under `client/app_core`.
- Rust integration tests: `cargo test` under `server/tests/rust/app-core-tests`.
- Flutter app join/switch adapters: `flutter test
  test/infra/bridge_app_core_api_test.dart
  test/features/networks/networks_page_desktop_test.dart
  test/infra/http_app_core_api_test.dart`.
- Web console build: `npm.cmd run build` under `server/server-ui/web`.
- Protocol drift checks:
  `go run scripts/check_protocol_contracts.go --root . --target web --target flutter --target rust-controller --target go-server --target openapi --target protobuf`.
