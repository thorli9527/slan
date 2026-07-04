# Web Console URL Resolution

This file is the single written source of truth for client-side Web Console URL
resolution across Flutter, macOS, Windows, Linux, and iOS packaged defaults.

## Resolution Order

1. Use explicit `SLAN_WEB_CONSOLE_URL` when present.
2. Otherwise derive from `SLAN_CONTROL_BASE_URL`.
3. Otherwise fall back to `http://47.245.40.231:24200`.

## Derivation Rules

- `api.dev.staticlss.com` maps to `web.dev.staticlss.com`
- `api.slan.localhost` maps to `web.slan.localhost`
- `slan.localhost` maps to `web.slan.localhost`
- loopback hosts (`127.0.0.1`, `localhost`, `::1`) map to the same host on port `24200`
- control URLs using port `28080` map to port `24200`
- the original URL scheme should be preserved during host mapping

## Expected Implementations

The following client entrypoints are expected to follow this rule:

- `client_v2/app_flutter/lib/bridge/client_core_bridge.dart`
- `client_v2/plugins/client_core_plugin/macos/Classes/ClientCorePlugin.swift`
- `client_v2/plugins/client_core_plugin/windows/client_core_plugin.cpp`
- `client_v2/plugins/client_core_plugin/linux/client_core_plugin.cc`

## Notes

- iOS `Info.plist` packaged defaults may still contain explicit control and web
  URLs; those are packaged defaults, not alternate derivation logic.
- The current packaged defaults should align with the common production fallback:
  `http://47.245.40.231:28080` for control and
  `http://47.245.40.231:24200` for Web Console.
- This rule governs browser-open behavior only. It does not change the Rust
  control-plane owner model for `/api/app/...`.
