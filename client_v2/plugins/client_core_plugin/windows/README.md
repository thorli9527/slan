# Windows Plugin

Windows connects Flutter to `client-core-service` with a small MethodChannel adapter.

Channel:

- `dev.slan/client_core_v2`

Methods:

- `start`
- `state`
- `refresh`
- `dispatch`
- `enqueueControlTask`
- `enqueueDownstreamControlTask`
- `ingestDownstreamControlMessage`
- `controlTransportPlan`
- `controlTransportCadence`
- `controlTransportTickPlan`
- `controlTransportOutbox`
- `pendingControlAcks`
- `markControlAcked`
- `markTransportPublished`
- `shutdownNetwork`

Service host:

- Default: `127.0.0.1:46392`
- Override: `SLAN_CLIENT_CORE_SERVICE_HOST`
- Tray/menu-bar quit uses the same service host override.
- Native quit resolves the host with `getaddrinfo`, so `localhost:46392` and IP literals both work.

Web Console URL:

- Default: `http://127.0.0.1:24200`
- Override: `SLAN_WEB_CONSOLE_URL`
- Fallback derive source: `SLAN_CONTROL_BASE_URL`

The Windows plugin must not implement mesh, adapter, DNS, route, MQTT, or device registration business logic. It only forwards UI commands to the local service and returns `ClientViewState`.

The Dart facade also has a local TCP fallback for platforms without a native MethodChannel handler.

Windows runtime chain:

1. Flutter calls `dev.slan/client_core_v2`.
2. The plugin forwards to `client-core-service`.
3. `client-core-service` owns the client state machine.
4. `client-core-service` performs privileged Windows network operations.

Current helper operations:

- Validate the `SLAN LAN Adapter` exists.
- Enable/disable the adapter.
- Set/remove IPv4 address.
- Set/reset DNS servers.
- Add explicit routes when provided.
- Read adapter status and current IPv4 address.

If no assigned virtual IP exists, enable must fail clearly instead of showing a fake pending IP.

Session and assigned IP:

- `client-core-service` persists session at `C:\ProgramData\SLAN\client-v2-session.json`.
- The session stores access token, device id, active network id, and assigned virtual IP.
- `syncAssignedIp` updates the assigned IP in service state and session storage.
- When enabling network, service can recover assigned IP from local session even if UI no longer shows the current adapter IP.
- Before local enable, service calls `/networks/{networkId}/activate` and uses the returned assignment IP.
- Before local disable, service calls `/networks/{networkId}/deactivate`.
- After local enable/disable, service reports runtime state with `/devices/{deviceId}/networks/{networkId}/state`.
- Independently of UI clicks, service periodically reads helper runtime state and reports it to the control plane.
- Device heartbeat/control reachability belongs to service and must not depend on whether network is enabled.

Control task queue:

- UI switch does not execute network changes directly.
- UI sends `enqueueControlTask` with `enableNetwork` or `disableNetwork`; these are upstream tasks and may call control-plane activate/deactivate.
- MQTT/Web control messages should call `enqueueDownstreamControlTask`; these are downstream tasks and only apply the server-decided local network action.
- MQTT QoS2 receivers should call `ingestDownstreamControlMessage` and ACK only after that call returns successfully.
- MQTT app-level ACK publishing reads `pendingControlAcks`; after the ACK publish succeeds, call `markControlAcked`.
- MQTT publish loops can use `controlTransportOutbox` to get publish-ready heartbeat, runtime state, and control ACK messages with QoS already assigned.
- `controlTransportOutbox` supports `includeHeartbeat`, `includeRuntimeState`, and `includeControlAcks` flags for separate publish cadences.
- `controlTransportCadence` owns the default ACK flush, heartbeat, and runtime state intervals.
- `controlTransportTickPlan` maps worker cursor timestamps to the next `controlTransportOutbox` flags.
- After publish success, call `markTransportPublished` with the outbox message `id`; only control ACK messages are mapped back to their XML task.
- Tasks are persisted at `C:\ProgramData\SLAN\client-v2-control-tasks.xml`.
- The XML file separates `<upstreamTasks>` and `<downstreamTasks>`, and downstream pending tasks run first.
- The service worker marks tasks as `pending`, `running`, `succeeded`, or `failed`.
- If `requireUiRefresh=true`, the service drains pending tasks in queue order before returning the latest UI state.

Browser login:

- `loginWithBrowser` is forwarded to `client-core-service` first.
- The service creates an `authCallbackId` in `ClientViewState`.
- The Windows plugin opens Web Console with `auth=login`, `callbackId`, and `deviceId`.
- The service polls `/auth/callback-status/{callbackId}` and applies the callback payload itself.
- After callback is ready, service registers or resolves the current device and stores the assigned IP.

UI lifetime:

- The Flutter window is not the owner of runtime networking.
- Closing UI must not imply network stop.
- Tray/menu-bar `Quit` calls `shutdownNetwork` before exiting the Flutter shell.
- `client-core-service` keeps local session, assigned IP, runtime sync, and MQTT control handling.

Windows relay multi-peer verification:

- The packaged installer stage includes `tools\test-windows-multipeer-relay.ps1`.
- Run it after login and network enable to validate the local data plane:
  - `powershell -ExecutionPolicy Bypass -File tools\test-windows-multipeer-relay.ps1 -ExportOnFailure`
- The tool talks to `client-core-service` over the same TCP JSON-line protocol as Flutter.
- It checks relay session count, per-peer attach state, replayed frames, config-hash mismatches, oversized packets, unroutable destinations, and DNS readback.
- On failure, `-ExportOnFailure` writes a full diagnostics JSON through `exportDiagnostics` and prints the path.
- `verify-installation.ps1 -Json` verifies the installed app, service, adapter, uninstall entry, and packaged relay diagnosis tool. It exits with code `1` when any check fails.
- `verify-installation.ps1 -RunRelayDiagnose -AllowMissingPeerSessions` can call the same tool after install. Use `-AllowMissingPeerSessions` only when testing with peers intentionally offline.
- `verify-installation.ps1 -ExpectUninstalled -Json` verifies uninstall cleanup, including service, adapter, shortcuts, scheduled tasks, state files, and diagnostics.
- `verify-installation.ps1 -SkipRelayDiagnoseToolCheck` is only for local development runs where the app was copied without rebuilding the installer. Packaged installer verification should not use it.
