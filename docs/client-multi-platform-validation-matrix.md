# Client Multi-Platform Validation Matrix

Last updated: 2026-07-09

This document records the client-side integration checks that were actually rerun against the current remote control stack at `http://47.245.40.231:28080`.

It is intentionally execution-oriented:
- `Passed` means the check was rerun and completed successfully in this validation round.
- `Partial` means the available environment can only verify part of the target behavior.
- `Blocked by environment` means the code path exists, but the current hardware/runtime cannot prove it end-to-end.

## Environment

- Control API: `http://47.245.40.231:28080`
- Web UI: `http://47.245.40.231:24200`
- MQTT host observed in tests: `47.245.40.231`
- Local macOS privileged service: verified healthy at `127.0.0.1:46392`
- Android test devices:
  - `emulator-5554`
  - `emulator-5556`
- iOS test devices:
  - `iPhone 17 Pro` simulator
  - `SLAN iPhone 16 Pro Clean 26.5` simulator
- Real iOS device: not available in this round

## Summary

| Area | Status | Notes |
| --- | --- | --- |
| Dual Android full business chain | Passed | DNS, ACL, bidirectional `client_message`, bidirectional UDP, bidirectional TCP |
| Dual iOS simulator business chain | Passed | DNS, ACL, MQTT message delivery, Flutter bidirectional messages |
| Dual Docker Linux full business chain | Passed | DNS, ACL, bidirectional `client_message`, bidirectional UDP, bidirectional TCP; rerun passed twice consecutively |
| Mac + Android | Passed | Android -> Mac UDP/TCP and Mac -> Android UDP/TCP both passed |
| Mac + iOS fast | Passed | DNS/ACL + bidirectional control-message delivery |
| iOS + Android simulator/emulator | Partial | Control/data-plane readiness passed; true iOS PacketTunnel UDP/TCP send skipped |
| Tri-device Mac + Android + iOS control-message chain | Passed | `Mac -> Android -> iOS -> Mac` passed |
| iOS true PacketTunnel real UDP/TCP | Blocked by environment | Requires real iOS device |

## Detailed Results

### Dual Android

- Script: `bash scripts/tests/android/android_dual_fast_check.sh --full-stable`
- Status: `Passed`
- Coverage:
  - device login
  - network enable
  - DNS zone provisioning
  - ACL rule provisioning
  - bidirectional `client_message`
  - `android-b -> android-a` via DNS: UDP + TCP
  - `android-a -> android-b` via DNS: UDP + TCP
- Final observed result:
  - `android-a=11111111111141118111111111111111 / 10.0.0.73`
  - `android-b=22222222222242228222222222222222 / 10.0.0.74`
  - `zone=android-dual-1783596614-28924.lan`
  - `android dual emulator integration ok`

### Dual iOS Simulator

- Scripts:
  - `bash scripts/tests/ios/ios_full_business_check.sh`
  - `bash scripts/tests/ios/ios_dual_flutter_message_check.sh`
- Status: `Passed`
- Coverage:
  - DNS / ACL quick validation
  - MQTT `client_message` quick validation
  - Flutter bidirectional message send/wait
- Final observed result:
  - DNS / ACL quick validation: `iosDualAclDnsIntegration: ok`
  - MQTT quick validation: `clientMessageMqttSmoke: ok`
  - Flutter bidirectional messages:
    - `ios-a=bc5c5bc9f4684f7ea083d19bd2a59e86`
    - `ios-b=9e45fd187de44a698525388170ad1402`
    - `iosDualFlutterMessageCheck: ok`

### Dual Docker Linux

- Script: `bash scripts/linux_dual_docker_packet_smoke.sh`
- Status: `Passed`
- Coverage:
  - device login
  - network enable
  - DNS zone provisioning
  - ACL rule provisioning
  - bidirectional `client_message`
  - `docker-b -> docker-a` via DNS: UDP + TCP
  - `docker-a -> docker-b` via DNS: UDP + TCP
- Final observed result, run 1:
  - `networkId=nete82dec2289ee0b06cefecd0de4e9b8a5`
  - `zone=linux-dual-1783135306.slan.test`
  - `deviceA=7a28dc1b113748acb54d0e09e4e0a4c3`
  - `deviceB=1ef85471d77144c196d093b5045b5503`
  - `linuxDualDockerIntegration: ok`
- Final observed result, run 2:
  - `networkId=netacf4500039bb09db9024e2dae47ff630`
  - `zone=linux-dual-1783135537.slan.test`
  - `deviceA=051bedd4bec843e6aeacbbac61ade28e`
  - `deviceB=8e8d871d60aa49f899f4d4796c3388ef`
  - `linuxDualDockerIntegration: ok`
- Stability note:
  - this check was rerun twice consecutively after fixing session persistence writes to be atomic

### iOS Tri-Client Protocol Matrix

- Script: `bash scripts/ios_triclient_packet_matrix.sh`
- Status: `Passed`
- Purpose:
  - local protocol-model verification
  - not a real simulator/client business login test
- Coverage:
  - `lan_udp`
  - `ipv6_udp`
  - `direct_udp`
  - `relay_udp`
  - `derp_tcp_tls_443`
  - both IP and DNS naming paths
- Final observed result:
  - `iosTriClientPacketMatrix: ok`

### Mac + iOS Fast

- Script: `bash scripts/tests/matrix/mac_ios_fast_check.sh`
- Status: `Passed`
- Coverage:
  - local macOS service login
  - iOS DNS / ACL smoke
  - Mac receives iOS `client_message`
  - iOS receives Mac `client_message`
- Final observed result:
  - `mac=bd9d2e5fe5ac4fab81b156b6d0879566`
  - `ios=0604f23e0d904c50a595be7d1f5e7f6e`
  - `macIosIntegrationCheck: ok`

### iOS + Android Without Real iOS Device

- Script: `bash scripts/ios_android_socket_check.sh`
- Status: `Partial`
- Verified:
  - iOS simulator login and network enable
  - Android emulator login and network enable
  - both virtual IPs allocated
  - Android relay/data-plane request state present
- Final observed result:
  - `iosIp=10.0.0.186`
  - `androidIp=10.0.0.187`
  - `androidRelaySessions=1`
  - `iosAndroidSocketCheck: control/data config ok ... iOS simulator true UDP/TCP send skipped`
- Limitation:
  - simulator cannot prove true iOS PacketTunnel system UDP/TCP send

### Mac + Android Passive Direction

- Script: `env SLAN_SUDO_PASSWORD='...' scripts/tests/matrix/mac_android_socket_check.sh`
- Status: `Passed`
- Coverage:
  - macOS service login + network enable
  - Mac hosts UDP/TCP echo
  - Android sends UDP/TCP to Mac
- Final observed result:
  - `macIp=10.0.0.83`
  - `androidIp=10.0.0.84`
  - `SLAN_TEST_UDP_ECHO_OK=10.0.0.83:19090`
  - `SLAN_TEST_TCP_ECHO_OK=10.0.0.83:19091`
  - `macAndroidSocketCheck: ok`

### Mac + Android Active Direction

- Script:
  - `env SLAN_SUDO_PASSWORD='...' bash scripts/tests/matrix/mac_android_active_socket_check.sh`
- Status: `Passed`
- Coverage:
  - macOS service login + network enable
  - Android hosts UDP/TCP echo
  - Mac sends UDP/TCP to Android
- Final observed result:
  - `macIp=10.0.0.88`
  - `androidIp=10.0.0.89`
  - Android log recorded UDP receive/send and TCP receive/send markers
  - `macAndroidActiveSocketCheck: ok`
 - Stability note:
   - first rerun in this round exited early before Android echo readiness markers appeared
   - immediate rerun passed end-to-end, so the business chain is working but this case still shows some run-to-run flakiness

### Tri-Device Control Message Chain

- Script:
  - `env SLAN_CLIENT_CORE_SERVICE_BIN=client_v2/rust/target/debug/client-core-service bash scripts/tests/matrix/mac_android_ios_message_check.sh`
- Status: `Passed`
- Coverage:
  - control-plane `client_message` across three clients
  - ordered chain:
    - `Mac -> Android`
    - `Android -> iOS`
    - `iOS -> Mac`
- Final observed result:
  - `Mac -> Android`
  - `Android -> iOS`
  - `iOS -> Mac`
  - `triDeviceMessageSmoke: ok`

## Known Current Limitation

### Real iOS PacketTunnel UDP/TCP

Status: `Blocked by environment`

Reason:
- iOS simulator cannot validate real system packet routing through `PacketTunnel`.
- The simulator can validate control-plane logic and app-level flows, but not the true device data path for UDP/TCP socket send/receive through the iOS tunnel.

What is needed:
- one real iPhone or iPad available to Flutter/Xcode
- rerun:
  - `bash scripts/ios_real_device_socket_check.sh`
  - `bash scripts/mac_ios_real_device_socket_check.sh`
  - optionally `SLAN_IOS_SEND_UDP=1 bash scripts/ios_android_socket_check.sh`

## Operational Notes

- In this round, several matrix scripts were updated to avoid `mapfile` so they work on macOS default bash:
  - `scripts/tests/android/android_dual_emulator_integration.sh`
  - `scripts/tests/ios/ios_dual_flutter_message_check.sh`
  - `scripts/tests/matrix/mac_android_socket_check.sh`
  - `scripts/tests/matrix/mac_android_active_socket_check.sh`
  - `scripts/tests/matrix/mac_ios_integration_check.sh`
- For stable Mac + Android reruns, `scripts/mac_android_fast_check.sh` now defaults `SLAN_RESET_EXISTING_MAC_SERVICE_IDENTITY=1`, which helps recover from stale installed macOS identity/session state.
- For dual iOS reruns, the iOS plugin now persists the requested test device ID override into native stable state, so repeated `flutter test` invocations stay aligned across Flutter UI, iOS plugin, and embedded Rust service.
- For dual Docker reruns, the Rust client atomically writes `config.json`; only `deviceId` is plaintext and credentials are stored in an AES-256-GCM encrypted payload.

## Recommended Rerun Entry

For the current local machine and device inventory, the most practical single entrypoint is:

- `bash scripts/current_client_regression.sh`

It intentionally includes only checks that are currently feasible without a real iOS device:

- dual Android full chain
- dual iOS simulator chain
- dual Docker Linux full chain
- Mac + Android passive-direction socket chain
- Mac -> Android active-direction socket chain
- Mac + iOS fast chain
- iOS + Android partial readiness chain
- tri-device control-message chain
- iOS tri-client local protocol matrix

It intentionally excludes:

- `scripts/ios_real_device_socket_check.sh`
- `scripts/mac_ios_real_device_socket_check.sh`
- `SLAN_IOS_SEND_UDP=1 bash scripts/ios_android_socket_check.sh`

because those require a real iOS PacketTunnel-capable device.
