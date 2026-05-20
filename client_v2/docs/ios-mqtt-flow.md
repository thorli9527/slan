# iOS MQTT Flow

This document describes the target iOS MQTT flow. Legacy native Swift MQTT/login/device control-plane code is intentionally out of scope.

## Ownership

- `client-core-ffi` exposes the embedded `client-core-service` JSON API to Flutter/iOS.
- Embedded `client-core-service` owns login, session persistence, device registration, MQTT credentials, MQTT5 control transport, downstream policy consumption, and upstream status publishing.
- iOS native plugin owns only platform execution: PacketTunnel start/stop, App Group config handoff, PacketTunnel stats, and system VPN permissions.
- PacketTunnel owns system packet I/O and writes runtime stats into the shared App Group store.

## Startup

1. Flutter bridge calls embedded `start`.
2. Embedded service loads persisted session from the shared client state directory.
3. If a valid session exists, embedded runtime applies it to `ClientRuntime`.
4. If the session is signed in, Flutter calls embedded `localEnsureDevice`.
5. After device registration succeeds, Flutter calls embedded `localConnectControlMqtt`.
6. Flutter starts the business-event watch loop through embedded `localBusinessEventWatch`.
7. Native iOS `start/localState/dispatch/localBusinessEventWatch/localControlStatus` must not be used for control-plane state.

## Login And Session

1. Mobile UI keeps the username/password login form.
2. Flutter dispatches `LoginWithPassword` to embedded `dispatch`.
3. Embedded service calls server login through `ControlPlaneClient`.
4. Embedded service hydrates/persists `PersistedSession`.
5. Flutter calls embedded `localEnsureDevice`.
6. After device registration succeeds, Flutter calls embedded `localConnectControlMqtt`.
7. Session includes access token, refresh token, user label, device id when available, active network id when available, virtual IP when available, and MQTT credential when returned by device/session hydration.
8. Flutter reads current state from embedded `localState` or embedded watch responses.

## Device Registration And MQTT Credential

1. Device registration is owned by embedded service, not Swift.
2. After login or signed-in startup, Flutter calls embedded `localEnsureDevice`.
3. Embedded service ensures the iOS device exists server-side and persists the returned device/session data.
4. During network config, embedded service also calls the same registration helper before requesting final platform config.
5. Server returns or refreshes MQTT credential with:
   - broker URL
   - client ID
   - username
   - password
   - topic prefix
   - expiry when available
6. Embedded service persists the credential in `PersistedSession`.
7. Embedded `localControlStatus` reports:
   - `mqttCredentialReady`
   - `controlSessionReady`
   - `ready`
   - `missing`
   - `mqttExpiresAt`
   - `activeNetworkId`
   - `deviceId`

## MQTT Connection

Target transport is MQTT 5.0 over TCP using `control-mqtt-client`.

1. MQTT connection is only attempted after `localEnsureDevice` succeeds.
2. Flutter calls embedded `localConnectControlMqtt`.
3. Embedded service loads the persisted MQTT credential from the registered device session.
4. It connects to MQTT using the persisted credential.
5. It subscribes to downstream topic:
   - `{topicPrefix}/control/down`
6. It uses QoS2 for control messages where required by the server policy.
7. If the persisted session changes, the worker exits and reconnects with the new session key.

## Downstream Message Consumption

Embedded service consumes downstream MQTT publishes and ACKs them after local ingest.

Supported downstream messages:

- `device_user_login_succeeded`
  - Applies browser login success payload into runtime/session.
- `device_ip_reassigned`
  - Verifies target `deviceId`.
  - Applies final IP through `SyncAssignedIp`.
  - Publishes `control.sync.changed`.
- `network_map_response`
  - Updates network map/connectivity state.
- `connect_plan`
  - If signed in and network enabled, schedules network rebuild when required.
  - Publishes network runtime or control sync events.
- `relay_data_plane_policy`
  - Validates target device.
  - Debounces duplicate policy.
  - Persists policy to relay policy store.
  - Applies relay data plane policy to runtime.
  - Publishes network runtime event.
- `client_message`
  - Accepts device-to-device payload.
  - Applies it to `ClientRuntime`.
  - Publishes `control.sync.changed` so Flutter can display the latest sender/body.
- normalized downstream control messages
  - Used for generic control task handling and ACK tracking.

## Upstream Publishing

Embedded service periodically builds outbound MQTT messages from runtime/session state.

Outbound categories:

- Heartbeat
  - Online state, device id, current IP, network enabled state.
- Runtime state
  - Network enabled/disabled, adapter/runtime status, error state.
- Path health
  - Relay/path health when available.
- Control ACKs
  - ACKs for downstream control tasks.

Cadence currently follows embedded/local control transport cadence:

- ACK flush: 1 second
- heartbeat: 30 seconds
- runtime state: 10 seconds
- path health: 10 minutes

## iOS Network Enable Flow

1. User taps Switch in Flutter.
2. Flutter calls embedded `localPlatformNetworkConfig`.
3. Embedded service:
   - ensures device registration
   - resolves active network
   - activates network server-side
   - persists assigned IP
   - returns final platform network config
4. Flutter calls native `iosStartPacketTunnel(config)`.
5. iOS plugin writes config to App Group shared store.
6. iOS plugin starts `NETunnelProviderManager`.
7. PacketTunnel reads config from App Group and configures:
   - virtual IP
   - prefix
   - DNS
   - routes
   - MTU
   - relay/data-plane fields
8. PacketTunnel writes stats back to App Group.
9. Flutter reads PacketTunnel stats and reports them to embedded `ingestPlatformRuntimeState`.
10. Embedded service includes runtime state in upstream MQTT publishing.

## iOS Network Disable Flow

1. User turns Switch off.
2. Flutter calls native `iosStopPacketTunnel`.
3. Flutter reports disabled runtime state to embedded service.
4. Embedded service updates runtime state and publishes heartbeat/runtime state through MQTT.
5. Embedded `localNetworkShutdown` maps to `DisableNetwork` for local state cleanup.

## Business Event Flow

1. Flutter waits on embedded `localBusinessEventWatch`.
2. Embedded service returns current business event snapshot.
3. Events originate from:
   - login/session changes
   - IP reassignment
   - network runtime changes
   - relay policy application
   - downstream control sync changes
4. Native iOS business-event queue is not part of the target flow.

## Current Gaps

- Embedded iOS FFI path has `localBusinessEventWatch` and downstream MQTT ingest, but the MQTT worker lifecycle still needs iOS foreground/background verification.
- `client_message` is surfaced through embedded business events and Flutter state; end-to-end Mac/iOS simulator delivery still needs device-pair verification.
- Relay data-plane policy persistence/apply exists in Rust worker, but iOS PacketTunnel rebuild handoff should be verified end to end through App Group config reload.
- MQTT reconnect/keepalive stability needs a mobile-focused lifecycle:
  - app foreground
  - app background
  - PacketTunnel running while app is suspended
  - credential refresh/session refresh
- iOS background constraints mean long-lived MQTT should live where execution is allowed. If the main Flutter app is suspended, control transport needed for active tunnel should be driven by PacketTunnel-safe embedded runtime or another allowed NetworkExtension path.

## Migration Rule

For iOS, any new MQTT/control-plane work must go through embedded `client-core-service` via `client-core-ffi`. Swift plugin code must not implement login, device registration, MQTT subscribe/publish, downstream control parsing, or policy selection.
