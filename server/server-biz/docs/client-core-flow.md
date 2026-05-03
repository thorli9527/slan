# Client Core Flow

This note defines the recommended public client flow. Ops-only behavior is out
of scope here.

## New User Creates A Network

1. `POST /auth/register` or `POST /auth/login`
2. `POST /auth/refresh` when the access token expires
3. `POST /devices/register`; when MQTT is enabled the response includes the
   device MQTT `clientId`, `username`, `password`, broker URL, and topic prefix.
4. `POST /nodes/register`
5. `GET /networks/home`
6. If `ownedNetwork` is empty, call `POST /networks` with `bindDeviceId`.
   If the server already created an owned network during registration, reuse
   the `ownedNetwork` returned by `/networks/home`.
7. `POST /bootstrap` with the selected `nodeId` and `networkId`
8. In bridge/service desktop mode, `app-core-service` owns MQTT control,
   control-sync, local tunnel, local DNS, and network-state reporting. Flutter
   only asks the service to enable or disable the selected network.
9. Use the returned control session token to start the MQTT control channel.
10. Call `POST /relay/tickets` only when direct path setup fails and a relay
   fallback is needed.

`POST /control/sessions` is still available when a client already has device,
node, and network context and only needs to refresh the control-plane session.
The normal public client flow should prefer `POST /bootstrap` because it returns
the control token, MQTT config, device attachment view, and initial
NetworkMap together.

Browser login callback delivery stays on HTTP status polling before the device
exists. MQTT starts only after device registration returns a device
credential:

1. The desktop app creates or reuses a pending callback id.
2. The web login completion calls
   `POST /auth/callback-status/{callbackId}/complete`; server-biz stores the
   payload.
3. The app polls `GET /auth/callback-status/{callbackId}` and applies the
   session without a separate ACK request.
4. The app registers the device with `POST /devices/register`.
5. If the registration response contains `mqtt`, the app connects to BifroMQ
   with that device credential and subscribes to the device topic prefix.
   In production, the BifroMQ Auth Provider calls `POST /mqtt/bifromq/auth`
   for credential validation and `POST /mqtt/bifromq/check` for topic access
   checks. A successful auth check marks only the device control channel as
   reachable.

Device runtime state is split into three meanings:

1. MQTT connect success means the device control channel is reachable
   (`controlReachable=true`).
2. The device becomes network-online only after the user enables the network
   and the local tunnel is up (`networkOnline=true`, `tunnelUp=true`).
3. After MQTT is connected, the client runtime keeps reporting
   `controlReachable=true` every 15 seconds for the selected network. Before
   the tunnel is enabled this report carries `networkOnline=false`.
4. While the network is enabled, the same heartbeat reports
   `networkOnline=true`, `tunnelUp=true`, and the latest probe result. The
   server marks stale control/network state offline when no report arrives for
   more than 45 seconds.

The preferred transport for the state heartbeat is MQTT topic
`{topicPrefix}/networks/{networkId}/state`, where the credential-specific
`topicPrefix` already contains the device id. `PUT
/devices/{deviceId}/networks/{networkId}/state` remains the HTTP fallback. Both
transports write the same `DeviceNetworkState` record.

When disabling a network, the app reports `networkOnline=false` immediately and
then calls `POST /networks/{networkId}/deactivate`. If MQTT remains connected,
the app continues the 15-second control reachability heartbeat with
`networkOnline=false`.

Management views should treat `Device.networkState.networkOnline` as the
source of truth for online devices.

Path quality and relay data-plane policy use these canonical path types:

- `direct_udp`
- `relay_udp`
- `relay_tcp`
- `relay_http3`
- `relay_tls`

The current Windows local service has real data-plane support for
`direct_udp`, `relay_udp`, and `relay_tcp`. `relay_http3` and `relay_tls` are
reserved in the protocol and can be managed by policy, but they must not be
selected as reachable until the local service and relay daemon have matching
HTTP3/TLS listeners and attach logic.

In the current desktop bridge/service architecture, the 15-second heartbeat and
45-second freshness semantics are implemented below Flutter:

- Flutter persists the user's last requested network usage state and, after a
  valid login/session refresh, asks `app-core-service` to restore the last
  enabled network.
- `app-core-service` runs the recurring control-sync and network-state report
  jobs.
- `app-core-helper` owns the OS-facing tunnel and local DNS runtime.
- Flutter must not directly mutate tunnel peers or local DNS in bridge mode.

Implementation checkpoints:

- Flutter entry: `NetworksPage` selects the active network, while
  `HomePage` enables or disables the selected network.
- App coordinator: `AppCoreCoordinator` keeps `selectedNetworkId` and passes it
  to activate, deactivate, bootstrap, and control refresh flows.
- Native bridge: `joinNetwork`, `joinNetworkByKey`, `switchNetwork`, and
  `activateNetwork` return
  `NetworkJoinResult`.
- Controller client: join, switch, and activate parse the server
  `member + attachment` response so the app can keep `networkId`,
  `attachmentId`, and virtual IP.

## New User Joins An Existing Network

1. `POST /auth/register` or `POST /auth/login`
2. `POST /devices/register`
3. `POST /nodes/register`
4. Call `POST /networks/join-by-key` with the invitation code.
5. Use the returned `networkId` and attachment result as the active network.
6. `POST /bootstrap` with the joined `networkId` and local `nodeId`.
7. Use `POST /relay/tickets` only after direct connection attempts fail.

If the user provides an alias, the app updates the joined attachment remark:

1. Use the returned `attachmentId` from the join result.
2. Call `PUT /networks/{networkId}/attachments/{attachmentId}/remark`.
3. The server accepts the update when the caller is the network owner or owns
   the device behind that attachment.
4. Refresh `GET /networks` so the member list displays the alias.

The web console follows the same rule for its join-key entry point: alias is
optional, but when provided it is persisted through the
attachment remark API before the workspace is refreshed.

## Join Semantics

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
5. After the local tunnel is up, the client runtime reports network state through
   `PUT /devices/{deviceId}/networks/{networkId}/state` and starts the
   15-second heartbeat.
6. `Disable Network` asks the client runtime to report `networkOnline=false`,
   bring down the local tunnel/DNS, then call
   `POST /networks/{networkId}/deactivate`.
7. Control sync and connection fallback must continue using the selected
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

- Web console build: `npm.cmd run build` under `server/server-ui/web`.
