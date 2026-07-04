# Test Scripts Layout

This folder holds the categorized implementations for the project's ad-hoc
test, smoke, validation, and matrix scripts.

The legacy entrypoints in [`scripts/`](/Users/thorli/workspace/slan/slan/scripts)
are preserved as symlinks for compatibility with existing docs, Make targets,
and operator habits.

## Categories

- `android/`
  Android emulator and Android-specific validation flows.
- `ios/`
  iOS simulator / real-device validation and iOS-specific helpers.
- `linux/`
  Linux client, Docker Linux, and remote VM Linux validation.
- `macos/`
  macOS-only service/login/host inspection checks.
- `matrix/`
  Cross-platform regression wrappers such as Mac + Android, Mac + iOS, mixed
  tri-device flows, and aggregate regression entrypoints.
- `backend/`
  service-biz, ops, Web UI, local Docker, and backend-facing smoke checks.
- `wire/`
  standalone Wire-stack smoke tests written in Go.
- `ui/`
  desktop UI and browser UI smoke tests.
- `shared/`
  reusable Go helpers and shell helpers used by the higher-level tests.
- `guard/`
  API-surface and boundary guard checks for client architecture constraints.

## Conventions

- Use `scripts/<name>` when you want the stable operator-facing entrypoint.
- Use `scripts/tests/<category>/<name>` when you are working on the organized
  implementation tree directly.
- New test scripts should be added under the matching category first, then
  optionally exposed via a root-level symlink if they are intended to be
  operator-facing.
