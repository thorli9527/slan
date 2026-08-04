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
  Includes `desktop_local_dns_smoke.sh`, which verifies the desktop
  `client-core-service` local DNS listener, localhost system-DNS handoff, and a
  real UDP DNS lookup against the managed record set after the desktop client is
  signed in and the network is enabled.
- `guard/`
  API-surface and boundary guard checks for client architecture constraints.
  Includes `audit_ui_template_methods.sh`, which scans `vm.*` calls used by
  Angular/Flutter-style HTML templates and fails when a referenced view-model
  method is missing from the matching TypeScript surface.

## Conventions

- Use `scripts/<name>` when you want the stable operator-facing entrypoint.
- Use `scripts/tests/<category>/<name>` when you are working on the organized
  implementation tree directly.
- New test scripts should be added under the matching category first, then
  optionally exposed via a root-level symlink if they are intended to be
  operator-facing.

## Current Stable Matrix

These are the currently verified quick-entry flows that should be preferred for
routine regression runs.

- Dual Android full business chain:
  `bash scripts/tests/android/android_dual_fast_check.sh --full-stable`
  Coverage: login, MQTT, bidirectional client message, DNS, ACL, UDP echo, TCP
  echo.
- Dual iOS simulator business chain:
  `bash scripts/tests/ios/ios_dual_fast_check.sh --full-stable`
  Coverage: DNS, ACL, MQTT, bidirectional Flutter message.
- Mixed Mac + Android real packet chain:
  `bash scripts/tests/matrix/mac_android_fast_check.sh --full-stable`
  Coverage: login, MQTT, DNS route propagation, Mac-hosted UDP echo, Mac-hosted
  TCP echo, Android -> Mac real packet send over the tunnel.
- Mixed Mac + iOS simulator business chain:
  `bash scripts/tests/matrix/mac_ios_integration_check.sh`
  Coverage: Mac service login, iOS app DNS/ACL smoke, MQTT, bidirectional Mac
  <-> iOS client message.
- Mixed Linux Docker + Mac real packet chain:
  `bash scripts/tests/linux/linux_docker_mac_integration.sh`
  Coverage: Linux Docker install/bootstrap, login, MQTT, DNS, ACL,
  bidirectional client message, Mac -> Linux UDP/TCP, Linux -> Mac UDP/TCP.
- Mixed remote Linux + Mac real packet chain:
  `bash scripts/mac_remote_linux_fast_check.sh`
  Coverage: remote Linux install/bootstrap, Mac service login, remote Linux
  login, DNS, ACL, bidirectional client message, Mac -> Linux UDP/TCP,
  Linux -> Mac UDP/TCP.
- Remote x86 Linux package + install preflight:
  `bash scripts/tests/linux/linux_remote_install_check.sh`
  Coverage: remote `amd64` package presence/build, remote install script,
  installed files, systemd service, local API availability.
- Android + remote x86 Linux mixed chain:
  `bash scripts/tests/matrix/android_remote_linux_integration.sh`
  Coverage: remote Linux install preflight, Android app login, remote Linux
  login, DNS, ACL, bidirectional client message, Android -> Linux UDP/TCP,
  Linux -> Android UDP/TCP.
- Aggregate current feasible matrix:
  `bash scripts/current_client_regression.sh`
  Coverage: current machine-feasible cross-client regression set, including a
  split dual-iOS quick/message steps, optional Android phase2-only message
  step, a split Linux Docker control-plane vs UDP/TCP packet path, and a
  standalone macOS local DNS smoke step.
  Optional remote mixed chain:
  `SLAN_RUN_CURRENT_MAC_REMOTE_LINUX=1 bash scripts/current_client_regression.sh`
  Optional local DNS toggle:
  `SLAN_RUN_CURRENT_MAC_LOCAL_DNS_SMOKE=0 bash scripts/current_client_regression.sh`
  Optional Android phase2-only toggle:
  `SLAN_RUN_CURRENT_ANDROID_DUAL_PHASE2_ONLY=1`
  Optional iOS split toggles:
  `SLAN_RUN_CURRENT_IOS_DUAL_QUICK=0`
  `SLAN_RUN_CURRENT_IOS_DUAL_FLUTTER_MESSAGE=0`
- Aggregate quick high-signal matrix:
  `bash scripts/current_client_quick_regression.sh`
  Coverage: dual Android phase2 bidirectional message, dual iOS DNS/ACL/MQTT
  quick, dual Docker Linux DNS/ACL/message control-plane, and standalone macOS
  local DNS smoke.
- Show latest quick high-signal summary:
  `bash scripts/current_client_quick_summary.sh`
  Prints the latest `summary.txt` from the quick regression result root.
- Show latest full current regression summary:
  `bash scripts/current_client_regression_summary.sh`
  Prints the latest `summary.txt` from the full current regression result root.
- Show latest failed log paths:
  `bash scripts/current_client_failed_logs.sh`
  Defaults to `full`; pass `quick` to inspect the latest quick regression.
  Add `--tail N` to print the last `N` lines from each failed log.
  Prints a friendly message when the latest run has no failures.

### Stable Defaults

- Dual Android phase 3/4 now pins
  `SLAN_ANDROID_DUAL_PHASE34_RELAY_TRANSPORT_ALLOWLIST=udp` by default.
- Mixed Mac + Android stable mode now also pins
  `SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST=udp` by default.
- Dual Android sender-side socket probes now wait for
  `SLAN_TEST_SOCKET_TARGETS_READY` before firing UDP/TCP payloads.
- Mixed Mac/Android, Mac/iOS, and Linux/Mac checks create independent device
  authorization keys per run, avoiding stale remote devices in peer selection.
- Android + remote Linux now runs
  `scripts/tests/linux/linux_remote_install_check.sh` first by default. Set
  `SLAN_RUN_REMOTE_LINUX_INSTALL_CHECK=0` only when you intentionally want to
  skip the install preflight.
- Mac + remote Linux uses the same remote install preflight by default and can
  preserve the failure workspace with `SLAN_KEEP_MAC_REMOTE_LINUX_WORK_DIR=1`
  for postmortem inspection.

### Known Boundaries

- iOS simulator validates control-plane and app-level business flows, but it
  does not prove real-device PacketTunnel UDP/TCP data-plane behavior.
- Linux Docker packet smoke still requires a Linux host with real TUN support;
  Docker Desktop on macOS is not sufficient for that path.
