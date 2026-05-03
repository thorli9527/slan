# Android Plugin

Android should bridge Flutter commands to Rust/client service APIs.

Do not expose mesh network internals to Flutter. Keep user-visible language as SLAN mesh networking.

## Native network contract

Android network control is implemented through `VpnService`, not by creating a
system adapter directly. Keep the Android side behind the Rust
`AndroidVpnBackend` contract:

- `permission_state`: check whether `VpnService.prepare(...)` is already
  granted.
- `request_permission`: return a Flutter-visible callback id and ask Android to
  launch the user consent activity.
- `start_vpn`: create the `VpnService.Builder`, apply virtual IP, DNS, routes,
  MTU and selected relay metadata, then return the runtime state.
- `stop_vpn`: close the active VPN interface and return the new runtime state.
- `protect_socket`: call `VpnService.protect(...)` for control plane, MQTT,
  relay probes and relay transport sockets so they do not loop through the VPN.
- `read_runtime_state`: report whether the VPN interface is present, enabled and
  which virtual IP is active.
- `poll_event`: surface Android-side events such as permission granted, VPN
  revoked, connectivity changed, relay changed and errors.

The shared DTOs live in `client-core/src/platform.rs`:

- `AndroidVpnPermissionState`
- `AndroidVpnConsentRequest`
- `AndroidVpnSessionConfig`
- `AndroidSocketProtectionRequest`
- `AndroidNetworkEvent`

Flutter MethodChannel methods:

- `androidVpnPermissionState`
- `androidRequestVpnPermission`
- `androidStartVpn`
- `androidStopVpn`
- `androidProtectSocket`
- `androidRuntimeState`
- `androidPollNetworkEvent`

Implemented native pieces:

- `ClientCorePlugin`: MethodChannel bridge, VPN permission request and event polling.
- `SlanVpnService`: Android `VpnService` lifecycle, foreground notification,
  IP/DNS/routes/MTU application, stop/revoke handling.
- `SlanVpnRuntime`: in-process runtime state and event queue for Flutter.
- `JsonCodec`: safe Dart map/list to JSON conversion for VPN configs.
- `SlanNativeBridge`: loads `libclient_core_ffi.so` and transfers the detached
  TUN fd plus one protected relay UDP fd per relay peer session to Rust.
- `client-core-ffi`: owns the Android TUN fd and starts the Rust-side TUN
  runtime loop. It attaches each protected UDP socket to the relay daemon before
  the data loop starts, enforces the configured `maxFramePayload`, and routes
  outbound TUN packets to the peer session that owns the destination virtual IP.
  When the TUN runtime stops, it sends `detach` for each relay peer session so
  relay-side endpoint bindings are cleaned up consistently with Windows.
  Full direct UDP path switching should stay behind the same Rust data-plane
  boundary.

Native library packaging:

- Build `client-core-ffi` for Android ABIs as `libclient_core_ffi.so`.
- Place outputs under `android/src/main/jniLibs/<abi>/libclient_core_ffi.so`
  before assembling the Android app.
