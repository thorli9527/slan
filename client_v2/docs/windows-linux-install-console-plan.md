# Windows/Linux install and console bootstrap

## Target split

- Windows GUI: Inno Setup `.exe`, installs Flutter GUI, `client-core-service.exe`, Wintun, diagnostics tools, and `SLANClientV2Service`.
- Windows console: zip/staged package with the same `client-core-service.exe` plus a PowerShell bootstrap script. It does not show Web Console.
- Linux GUI: tar/deb package, installs Flutter bundle under `/opt/slan-client-v2/gui`, `client-core-service` under `/opt/slan-client-v2/bin`, a systemd service, and an optional desktop entry.
- Linux console: tar/deb package without desktop assumptions. The service runs `client-core-service`; the bootstrap script writes `/etc/slan/client-v2-console.env` and can start/restart the service.

## Console bootstrap parameters

Both Windows and Linux use the same logical parameters:

```text
--server-url URL        Server control base URL, for example http://127.0.0.1:18080
--email EMAIL           Login account. Required for password login.
--password PASSWORD     Login password. Required for password login.
--join-key KEY          32 character network invitation key.
--invite URL_OR_KEY     Alias for --join-key. If an invite URL is supplied, the script extracts joinKey/code/key.
--device-name NAME      Optional display name for future registration metadata.
--enable-network        Enable the active network after login/join succeeds.
--foreground            Run client-core-service in the current console instead of using the OS service.
```

## Intended flow

1. Ensure local stable `deviceId`.
2. Login with email/password through `client-core-service`.
3. If `--join-key` is present, call server `POST /networks/join-by-key` with `{ joinKey, deviceId }`.
4. Refresh local session/network state.
5. If `--enable-network` is present, call local `localNetworkActivate`.

Current installer scripts stage `SLAN_PENDING_JOIN_KEY` and related values so package entry points are stable first. The next core-service change should add a local method such as `consoleBootstrap` or `localJoinNetworkByKey` to consume those values atomically.

## Environment contract

The installers and startup scripts use these environment names:

```text
SLAN_CONTROL_BASE_URL
SLAN_CLIENT_CORE_SERVICE_HOST
SLAN_PENDING_JOIN_KEY
SLAN_PENDING_DEVICE_NAME
SLAN_PENDING_ENABLE_NETWORK
```

Linux stores them in `/etc/slan/client-v2-console.env`. Windows stores them in `%ProgramData%\SLAN\client-v2-console.env`.
