# Windows Plugin

Windows connects Flutter to `client-core-service` with a small MethodChannel adapter.

Channel:

- `dev.slan/client_core_v2`

Forwarded control-plane methods:

- `start`
- `state`
- `refresh`
- `dispatch`
- `localNetworkShutdown`

The plugin does not synthesize fallback responses for these methods. If
`client-core-service` is unavailable, the MethodChannel call is not implemented
and the Flutter bridge must surface the local service failure.

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

Control tasks and MQTT:

- `client-core-service` owns MQTT subscribe/publish, QoS handling, control task queueing, ACK state, heartbeat, runtime state, and downstream message ingestion.
- The Windows plugin exposes only platform execution methods; control outbox, ACK handling, and downstream ingestion stay inside `client-core-service`.
- Tasks are persisted by the service at `C:\ProgramData\SLAN\client-v2-control-tasks.xml`.

Browser login:

- `loginWithBrowser` is forwarded to `client-core-service` first.
- The service creates an `authCallbackId` in `ClientViewState`.
- The Windows plugin opens Web Console with `auth=login`, `callbackId`, and `deviceId`.
- The service polls `/auth/callback-status/{callbackId}` and applies the callback payload itself.
- After callback is ready, service registers or resolves the current device and stores the assigned IP.

UI lifetime:

- The Flutter window is not the owner of runtime networking.
- Closing UI must not imply network stop.
- Tray/menu-bar `Quit` calls `localNetworkShutdown` before exiting the Flutter shell.
- `client-core-service` keeps local session, assigned IP, runtime sync, and MQTT control handling.

Windows relay multi-peer verification:

- The packaged installer stage includes `tools\test-windows-multipeer-relay.ps1`.
- Run it after login and network enable to validate the local data plane:
  - `powershell -ExecutionPolicy Bypass -File tools\test-windows-multipeer-relay.ps1 -ExportOnFailure`
- The tool talks to `client-core-service` over the same TCP JSON-line protocol as Flutter.
- It checks relay session count, per-peer attach state, replayed frames, config-hash mismatches, oversized packets, unroutable destinations, and DNS readback.
- On failure, `-ExportOnFailure` writes a full diagnostics JSON through `localDiagnosticsExport` and prints the path.
- `verify-installation.ps1 -Json` verifies the installed app, service, adapter, uninstall entry, and packaged relay diagnosis tool. It exits with code `1` when any check fails.
- `verify-installation.ps1 -RunRelayDiagnose -AllowMissingPeerSessions` can call the same tool after install. Use `-AllowMissingPeerSessions` only when testing with peers intentionally offline.
- `verify-installation.ps1 -ExpectUninstalled -Json` verifies uninstall cleanup, including service, adapter, shortcuts, scheduled tasks, state files, and diagnostics.
- `verify-installation.ps1 -SkipRelayDiagnoseToolCheck` is only for local development runs where the app was copied without rebuilding the installer. Packaged installer verification should not use it.
