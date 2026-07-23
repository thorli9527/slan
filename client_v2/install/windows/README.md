# SLAN Client V2 Windows installer

The Windows installer uses an explicit reinstall boundary:

- `InitializeSetup` kills stale `slan_client_v2.exe`, `client-core-service.exe`, and `client-core-helper.exe` processes.
- It stops and deletes the old `SLANClientV2Service` Windows Service, then waits until the Service Control Manager no longer reports it.
- It deletes legacy scheduled tasks before the new payload is installed.
- During post-install it initializes the stable device ID, prepares the Wintun adapter, creates `SLANClientV2Service` again, and starts it.
- During uninstall it stops the runtime, deletes the Windows Service and scheduled tasks, removes the adapter, and clears client state/install artifacts.

Desktop behavior is aligned with macOS:

- Closing the window keeps the privileged service and active network running.
- The tray icon refreshes service, login, and network state automatically.
- The tray menu shows the current user and disables network actions while a transition is in progress.
- `Open` restores the single application window; explicit `Quit` shuts down the local network first.
- The tray icon is restored automatically when Windows Explorer restarts.

Build:

```powershell
powershell -ExecutionPolicy Bypass -File .\client_v2\install\windows\package-installer.ps1
```

Installer constants shared by the packaging, console bootstrap, and verification
scripts live in `SlanWindowsInstall.psm1`. Keep service names, install paths,
state cleanup files, packaged tools, and the default control URL there first.

Console bootstrap defaults to `http://47.245.40.231:28080` and can be overridden with `-ServerUrl`:

```powershell
powershell -ExecutionPolicy Bypass -File .\client_v2\install\windows\slan-console.ps1 `
  -Email user@example.test `
  -Password password
```
