# M2 Patch Summary

This note is a commit-prep summary for the current M2 worktree.

## Main Change Areas

### 1. Desktop auth callback flow

- Desktop auth callback now prefers `deviceId` as the callback key.
- Repeated login attempts on the same device are treated as fresh callback
  attempts.
- Web auth URLs now resolve cleanly back to the root login entrypoint.
- Local browser-to-desktop callback smoke was verified for both register and
  login flows.

Primary files:

- `client/app/lib/features/auth/auth_callback_service.dart`
- `client/app/lib/features/auth/auth_page.dart`
- `client/app/lib/features/home/home_page_logic.dart`
- `server/server-ui/web/src/ui/app.component.ts`
- `server/server-ui/web/src/ui/console-session.service.ts`

### 2. Windows desktop packaging and startup

- Windows plugin now handles tunnel methods before forwarding helper calls.
- Tunnel cleanup avoids an unnecessary elevated path when no staged tunnel
  config exists.
- Startup logging writes to the system temp directory.
- Windows packaged app, installer payload, installed app launch, and startup log
  flow were all smoke-tested successfully.

Primary files:

- `client/app_core_plugin_windows/windows/slan_app_core_plugin_windows_plugin.cpp`
- `client/app/lib/infra/logging/startup_log.dart`
- `client/app/windows/runner/CMakeLists.txt`
- `client/app/windows/installer/`

### 3. Tunnel helper host path

- Desktop tunnel host gateway support was added and covered with tests.
- Helper-host error mapping and invalid-address handling were validated.

Primary files:

- `client/app/lib/application/tunnel_host_gateway.dart`
- `client/app/test/infra/tunnel_host_gateway_test.dart`

### 4. Server-biz local Go testability

- Sqlite-backed Go tests now run without CGO by using a pure Go sqlite driver.
- Service tests no longer depend on a live Redis token backend.

Primary files:

- `server/server-biz/configs/runtime_test.go`
- `server/server-biz/internal/service/impl/network_test.go`
- `server/server-biz/internal/service/impl/token_store_test.go`
- `server/server-biz/internal/service/impl/state.go`
- `server/server-biz/go.mod`
- `server/server-biz/go.sum`

## Validation Snapshot

Verified during this M2 pass:

- `flutter test`
- `npm run build` in `server/server-ui/web`
- `flutter build windows`
- Windows installer packaging and install smoke
- Browser register callback to desktop
- Browser login callback to desktop
- `cargo test` for the validated `client/app_core` crates
- `go test ./...` in `server/server-biz`

## Suggested Commit Split

### Commit 1: Desktop auth callback and web flow

- Flutter auth callback handling
- Web callback forwarding
- Auth URL/config tests
- Browser smoke docs

### Commit 2: Windows runtime and packaging

- Windows plugin tunnel dispatch
- Startup log behavior
- Installer and packaging scripts
- Windows packaging docs

### Commit 3: Server-biz testability

- Pure Go sqlite switch for tests
- In-memory token store for service tests
- Server-biz local test notes

## Suggested Exclusions Before Commit

Review before staging:

- `server/server-ui/web/.angular/cache/...`
- any other regenerated cache or lock output not intended for source control

Those files are validation artifacts, not core M2 source changes.
