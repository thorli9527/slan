# Local Auth Callback Smoke

This note records the local browser-to-desktop auth callback smoke that was
verified on Windows on April 22, 2026.

## Preconditions

- Local Docker stack was up from `docker-compose.local.yml`.
- `server-biz` health check returned `ok` from `http://127.0.0.1:28080/healthz`.
- Windows `hosts` contained:

```text
127.0.0.1 slan.localhost
127.0.0.1 web.slan.localhost
127.0.0.1 ops.slan.localhost
```

- Desktop app was started from the packaged install output and configured with
  `flutter.slan.server_host = slan.localhost`.

## Verified Flows

### 1. Browser register callback to desktop

- Opened the real local web entry:
  `https://web.slan.localhost:18443/?auth=register&callbackId=<id>&deviceId=<clientMachineId>`
- Submitted a real registration in Chrome.
- Server callback status later showed `received=true`.
- Desktop `shared_preferences.json` contained a real `flutter.slan.session`.

Observed session payload:

- `userLabel = smoke.20260422214856@example.com`
- `deviceId = dev-37df8963442e`

### 2. Browser login callback to desktop

- Cleared the local desktop session while keeping `slan.localhost` and a new
  pending callback id.
- Opened the real local web entry:
  `https://web.slan.localhost:18443/?auth=login&callbackId=<id>&deviceId=<clientMachineId>`
- Submitted a real login in Chrome with the same smoke account.
- Server callback status later showed `received=true`.
- Desktop `shared_preferences.json` was updated with a fresh session token set.

## Evidence Used

- `GET /auth/callback-status/<callbackId>`
- Desktop persisted preferences:
  `C:\Users\thorl\AppData\Roaming\com.example\slan_app\shared_preferences.json`
- Desktop startup/runtime log:
  `%TEMP%\slan_app_startup.log`

## Notes

- The local HTTPS entry is backed by Caddy with an internal dev certificate.
- Browser automation reached the business flow, but the Chrome DevTools driver
  script itself timed out during teardown more than once.
- The smoke result should therefore be judged from server callback status and
  desktop session persistence, not from the browser automation process exit
  code alone.
- The related `server-biz` local Go testability notes live in
  [server-biz-local-test-notes.md](./server-biz-local-test-notes.md).
