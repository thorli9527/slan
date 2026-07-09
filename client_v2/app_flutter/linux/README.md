# Linux Tray Policy

Linux tray support is optional at install time.

Reason:

- Desktop environments differ in AppIndicator/system tray behavior.
- Server-style Linux installs may run only `client-core-service`.

Install option:

- `trayMode=enabled`: install desktop shell with tray integration.
- `trayMode=disabled`: install service/helper only, no tray.

The policy writer lives at `client_v2/install/linux/install.sh`.

Runtime policy:

- If enabled, closing the window hides it to tray.
- Tray owns only `Open`, a checked `Network` switch item, and `Quit`.
- The `Network` switch item is disabled until the local service reports a signed-in user.
- The tray icon changes between network enabled and disabled/signed-out states.
- If tray is enabled, `Quit` should call `localNetworkShutdown` before exiting the Flutter shell.
- `client-core-service` keeps networking, task queue, heartbeat, and runtime sync.

Implementation boundary:

- AppIndicator/libayatana integration belongs in the Linux runner or platform plugin.
- Flutter UI remains a thin state/command shell.
- Network operations remain in `client-core-service`.
