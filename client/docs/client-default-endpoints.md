# Client Default Endpoints

This file is the single written source of truth for packaged client default
endpoint values.

## Production Defaults

- control plane base URL: `http://47.245.40.231:28080`

## Expected Consumers

These defaults are expected to stay aligned in the main client entrypoints:

- `client/rust/crates/client-core-service/src/control_plane.rs`
- `client/app_flutter/lib/bridge/client_core_bridge.dart`
- `client/app_flutter/ios/Runner/Info.plist`
- `client/install/linux/lib/slan-linux-install.sh`
- `client/install/windows/SlanWindowsInstall.psm1`
- `scripts/lib/client_default_endpoints.sh`

## Notes

- Tests may still override these defaults explicitly.
- Shell-based validation and publish scripts should prefer
  `scripts/lib/client_default_endpoints.sh` instead of repeating raw endpoint
  literals.
- New shell entrypoints that consume client-facing control/Ops defaults
  should source `scripts/lib/client_default_endpoints.sh` first, then layer any
  localhost or remote-special-case overrides on top.
- New packaged client entrypoints should update this document and
  `scripts/audit_client_api_surface.sh` in the same change so the written rule
  and the guard stay aligned.
- For release or refactor checks, prefer `bash scripts/check_client_stack_guard.sh`
  so boundary, endpoint, and protocol-facing client guards run together.
- Rust remains the owner of `/api/app/...` business API access; these values are
  packaged defaults only.
