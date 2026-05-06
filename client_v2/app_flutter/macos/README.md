# macOS Tray Policy

macOS should run SLAN Client V2 as a menu bar resident app.

Policy:

- launchd label: `dev.slan.client-core-service`.
- Installed service binary: `/Library/Application Support/SLAN/client-core-service`.
- App-bundled service binary: `slan_client_v2.app/Contents/MacOS/client-core-service`.
- State directory: `/Library/Application Support/SLAN`.
- Logs: `/Library/Logs/SLAN`.
- Closing the main window hides it.
- The menu bar item owns `Open`, `Connect/Disconnect`, `Status`, `Open Console`, and `Quit`.
- `Quit` calls `localNetworkShutdown` and then exits the Flutter shell.
- `localNetworkShutdown` uses `SLAN_CLIENT_CORE_SERVICE_HOST` when provided.
- MethodChannel calls are forwarded to `client-core-service`; Swift keeps only a fallback state for service-unavailable startup.
- `client-core-service` owns runtime networking and should continue independently when installed as a launchd service.
- Menu state follows the same model as the Flutter UI: it listens to `localStateWatch` and only updates when the service state revision changes.

Implementation boundary:

- AppKit status item lives in the macOS runner or macOS plugin.
- Flutter UI only sends `ClientCommand`.
- Network enable/disable still goes through `client-core-service`.
- `Connect/Disconnect` only sends the local service command; it must not force a UI refresh.
- `Open Console` first requests `consoleLoginKey` from `client-core-service`, matching the Windows tray behavior.
- Web Console URL resolution matches Windows: `SLAN_WEB_CONSOLE_URL`, then `SLAN_CONTROL_BASE_URL`, then `http://127.0.0.1:24200`.
- No mesh, DNS, route, or adapter logic belongs in Swift UI code.

Native pieces:

- `NSStatusItem` with app icon.
- Menu items: `Open SLAN Client`, `Connect/Disconnect`, `Status`, `Open Console`, `Quit`.
- The plugin assigns window delegates so close hides the window instead of terminating.
- Explicit menu bar `Quit` calls `localNetworkShutdown` before terminating.

Build check:

- `make client-macos-build`
- Smoke: `make client-macos-service-smoke`

Service scripts:

- Install: `scripts/install_macos_service.sh --app client_v2/app_flutter/build/macos/Build/Products/Release/slan_client_v2.app`
- Status: `scripts/status_macos_service.sh`
- Uninstall: `scripts/uninstall_macos_service.sh`
- Install/uninstall write `/Library/*` paths and require an interactive sudo/root terminal.
- Service binary self-description: `client-core-service --service-info`.
- Re-run install after every rebuilt service binary; `make client-macos-service-smoke` fails when installed and bundled service hashes differ.
- Upgrade smoke: `make client-macos-service-upgrade-smoke`.
