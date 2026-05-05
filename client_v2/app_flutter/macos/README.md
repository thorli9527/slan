# macOS Tray Policy

macOS should run SLAN Client V2 as a menu bar resident app.

Policy:

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
- No mesh, DNS, route, or adapter logic belongs in Swift UI code.

Native pieces:

- `NSStatusItem` with app icon.
- Menu items: `Open SLAN Client`, `Connect/Disconnect`, `Status`, `Open Console`, `Quit`.
- The plugin assigns window delegates so close hides the window instead of terminating.
- Explicit menu bar `Quit` calls `localNetworkShutdown` before terminating.
