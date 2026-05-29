# SLAN Client V2 Windows installer

The Windows installer uses an explicit reinstall boundary:

- `InitializeSetup` kills stale `slan_client_v2.exe`, `client-core-service.exe`, and `client-core-helper.exe` processes.
- It stops and deletes the old `SLANClientV2Service` Windows Service, then waits until the Service Control Manager no longer reports it.
- It deletes legacy scheduled tasks before the new payload is installed.
- During post-install it initializes the stable device ID, prepares the Wintun adapter, creates `SLANClientV2Service` again, and starts it.
- During uninstall it stops the runtime, deletes the Windows Service and scheduled tasks, removes the adapter, and clears client state/install artifacts.

Build:

```powershell
powershell -ExecutionPolicy Bypass -File .\client_v2\install\windows\package-installer.ps1
```

Console bootstrap defaults to `http://47.245.40.231:28080` and can be overridden with `-ServerUrl`:

```powershell
powershell -ExecutionPolicy Bypass -File .\client_v2\install\windows\slan-console.ps1 `
  -Email user@example.test `
  -Password password
```
