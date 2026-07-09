# SLAN Client V2

`client_v2` is the next client architecture for SLAN. It keeps Flutter thin and moves durable client business logic into Rust.

## Boundary

Flutter owns:

- Login entry and browser-to-client MQTT handoff
- Current account, current IP, network enabled/syncing/error UI
- Enable/disable intent from the switch
- Web Console button

Rust `client-core-service` owns:

- Device identity, registration, heartbeat, and server session state
- MQTT QoS2 control task consumption
- MQTT credential persistence from the control-plane device DTO
- Durable control task queue
- Network enable/disable state machine
- Mesh config generation
- Runtime monitoring and `networkState` reporting
- A small status/command API for Flutter

Rust `client-core-service` owns:

- Privileged OS network operations
- Virtual adapter create/delete/configure
- IP, DNS, and route operations
- System network inspection

Flutter must not directly generate mesh config, consume MQTT tasks, read helper internals, or manipulate DNS/routes/adapters.

## Layout

- `app_flutter/`: UI shell and bridge-facing client screens.
- `plugins/client_core_plugin/`: Flutter platform plugin facade.
- `rust/crates/client-core/`: platform-neutral client state machine and ports.
- `rust/crates/client-core-service/`: local service process.
- `rust/crates/client-core-service/`: local service and privileged network runtime.
- `rust/crates/client-core-platform/`: OS-specific helper implementations.
- `rust/crates/client-core-ffi/`: FFI surface used by desktop/mobile plugin layers.

## Migration Rule

Move behavior from the old Flutter client only when the target owner is clear:

- UI interaction stays in Flutter.
- Persistent state and network workflows go to `client-core-service`.
- Admin/system operations go to `client-core-service`.

## Tray Policy

- Windows: tray is enabled by default. Closing the window hides it to tray.
- macOS: menu bar mode is enabled by default. Closing the window hides it; the plugin forwards commands to `client-core-service`.
- Linux: tray is optional at install time; service-only installs are valid.

The tray owns UI shell lifetime only. Runtime networking belongs to `client-core-service`.
Desktop tray/menu-bar UI is intentionally icon-only and exposes only `Open`, a checked `Network` switch item, and `Quit`.
The `Network` switch item is disabled until the local service reports a signed-in user.
The tray icon changes between network enabled and disabled/signed-out states.
On Windows and macOS, explicit tray/menu-bar Quit calls `localNetworkShutdown` before exiting the shell.

If a platform has no native plugin handler yet, the Dart plugin facade falls back to the same local JSON-line TCP API exposed by `client-core-service`.

For macOS/iOS/Android multi-device development on one Mac, see [`../docs/client-v2-multidevice-dev.md`](../docs/client-v2-multidevice-dev.md).

For device ID generation and install-time reset behavior, see [`../docs/client-v2-device-id.md`](../docs/client-v2-device-id.md).

## Control Transport

The service stores MQTT credentials in the local session after login/device registration. Flutter does not read or manage these credentials.

- Heartbeat and runtime state messages use QoS 0.
- Control/config/task messages use QoS 2.
- A QoS 2 downstream message is acknowledged only after the service has durably written the task/update and reached a recoverable processing point.
- Re-delivery is deduplicated with the server `messageId`/local `deliveryId` rather than by UI action name.
- MQTT workers should feed received QoS 2 control payloads into `ingestDownstreamControlMessage`; the method only returns after the downstream XML task is durably accepted.
- After a downstream task reaches `succeeded` or `failed`, MQTT workers read `localPendingControlAcks`, publish the app-level ACK on the QoS 2 ack topic, then call `localMarkControlAcked`.
- MQTT workers can read `localControlOutbox` to get already-shaped publish messages: heartbeat/runtime state use QoS 0, control ACK uses QoS 2.
- `localControlOutbox` accepts `includeHeartbeat`, `includeRuntimeState`, and `includeControlAcks` flags so workers can request ACKs frequently without resending heartbeat/runtime state.
- After each outbox message is published, workers call `localMarkTransportPublished` with the message `id`; only `controlAck` messages update XML `acknowledgedAtMs`.
- XML control tasks are split into `upstreamTasks` and `downstreamTasks`.
  - `upstreamTasks`: local/UI intents or client-originated requests, such as clicking the switch. These may call control-plane activate/deactivate before local network changes.
  - `downstreamTasks`: server/Web/MQTT commands delivered to the client, such as remote enable/disable. These only execute the local network state machine and must not call back into control-plane activate/deactivate.
  - Downstream pending tasks run before upstream pending tasks so Web/admin disable wins over stale local switch intents.
  - Completed downstream tasks keep an `acknowledgedAtMs` marker so worker restarts can resend missing ACKs safely.
  - Periodic control-plane sync is treated like downstream input: if the server says the device is disabled or has no assigned IP, the service only disables the local network and reports runtime state.

`localControlStatus` is an internal service API for diagnostics and future workers. It reports whether MQTT credentials and the local control session are both ready. UI should continue to use the simplified client state.
`localControlPlan` exposes the normalized MQTT topic/QoS plan for service workers and diagnostics; Flutter UI should not depend on it.
`localControlCadence` exposes service-owned publish intervals for ACK flush, heartbeat, and runtime state ticks.
`localControlTickPlan` turns worker cursor timestamps into `localControlOutbox` flags, keeping publish cadence decisions in the service.
`localControlOutbox` exposes publish-ready messages for the MQTT worker; publishing success is reported with `localMarkTransportPublished`.

The service owns a control-transport supervisor. Once MQTT credentials and the control session are ready, it starts one worker per current session key. If the session, device, network, MQTT client id, or topic prefix changes, the worker exits and reconnects with backoff.

Worker loop contract:

1. Wait until `localControlStatus.ready` is true.
2. Subscribe to `localControlPlan.downstreamControlTopic` with QoS 2.
3. For every received control payload, call `ingestDownstreamControlMessage`; only then complete the MQTT QoS 2 receive handshake.
4. Call `localControlTickPlan` with the worker's last publish timestamps, then query `localControlOutbox` with the returned flags:
   - Frequent ACK flush: `includeHeartbeat=false`, `includeRuntimeState=false`, `includeControlAcks=true`.
   - Heartbeat tick: `includeHeartbeat=true`, `includeRuntimeState=false`, `includeControlAcks=true`.
   - Runtime tick: `includeHeartbeat=false`, `includeRuntimeState=true`, `includeControlAcks=true`.
5. Publish each outbox message with its provided QoS.
6. After publish success, call `localMarkTransportPublished` with the outbox message `id`.

The current worker uses the local v2 `control-mqtt-client` crate as a thin raw publish/subscribe facade. It does not use the old node envelope workflow.

## Runtime State Paths

- Windows: `%ProgramData%\SLAN`
- macOS: `/Library/Application Support/SLAN`
- Linux: `${SLAN_STATE_DIR:-/var/lib}/SLAN`
