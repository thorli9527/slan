# SLAN Client Platform Plugin Interface

This is the boundary between `client-core-service` and platform plugins.

## Ownership

`client-core-service` owns:

- login, session refresh, logout, and device registration
- MQTT connection, subscription, downstream policy consumption, and QoS handling
- network quality policy selection and final policy materialization
- peer, path, relay, and DERP path calculation
- final network configuration generation
- heartbeat, runtime state aggregation, traffic/error reporting, and server reporting
- optional packet or relay-frame forwarding when a platform cannot consume config directly

Additional boundary rule:

- Flutter/Dart must not call `/api/app/...` directly.
- Flutter/Dart must consume business state only through Rust local API or embedded API.

Platform plugins own:

- platform permissions and lifecycle prompts
- TUN, utun, WinTun, PacketTunnel, or VpnService creation
- system DNS and route application
- packet read/write when handled by the platform runtime
- runtime state, heartbeat, traffic, and error reports back to `client-core-service`

## Stable Local Service Methods

Flutter and platform plugins should use these service methods for control-plane data.

| Method | Owner | Purpose |
| --- | --- | --- |
| `dispatch` | core-service | Login, logout, refresh, enable/disable intent. |
| `localState` | core-service | Current UI state snapshot. |
| `localBusinessEventWatch` | core-service | State/control event stream. |
| `localControlStatus` | core-service | MQTT/control connection status. |
| `localPlatformNetworkConfig` | core-service | Final IP/DNS/routes/MTU/relay/path config plus current server network configs for platform application. |
| `ingestPlatformRuntimeState` | core-service | Receive platform runtime, traffic, heartbeat, and error reports; Rust updates local state and reports the normalized runtime to the control plane. |
| `localNetworkShutdown` | core-service | Disable local network and clear runtime state. |
| `localDiagnosticsExport` | core-service | Unified diagnostics. |

## Stable Platform Plugin Methods

Platform plugins should expose only platform execution methods to Flutter.

| Method | Platform | Purpose |
| --- | --- | --- |
| `androidVpnPermissionState` | Android | Query VpnService permission. |
| `androidRequestVpnPermission` | Android | Request VpnService permission. |
| `androidStartVpn` | Android | Apply `PlatformNetworkConfig` to VpnService. |
| `androidStopVpn` | Android | Stop VpnService. |
| `androidProtectSocket` | Android | Protect control/relay sockets from VPN capture. |
| `androidRuntimeState` | Android | Report VpnService runtime state. |
| `androidWatchNetworkEvent` | Android | Report permission/connectivity/VPN events. |
| `iosEnableNetwork` | iOS | Apply `PlatformNetworkConfig` to PacketTunnel. |
| `iosDisableNetwork` | iOS | Stop PacketTunnel. |
| `iosPacketTunnelStats` | iOS | Report PacketTunnel runtime stats. |
| `iosSharedStoreDiagnostics` | iOS | Report App Group/shared config availability. |

iOS PacketTunnel config/stats use App Group `group.dev.slan.client.v2` when
available. The implementation falls back to standard `UserDefaults` for
simulator and unsigned development builds where the App Group is not present.

Desktop plugins may forward service methods to the local daemon, but platform
execution remains platform-owned:

- macOS: NetworkExtension/utun, DNS, routes
- Windows: WinTun/WFP, DNS, routes
- Linux: tun, ip route, resolv or systemd-resolved

## Migration Status

Completed direction:

- Mobile UI keeps username/password login, but login/session commands go to
  embedded `client-core-service`.
- Mobile platform config is fetched through `localPlatformNetworkConfig`.
- Platform plugins only apply platform config and report runtime state.
- Mobile MQTT and policy consumption are owned by embedded Rust service/FFI.
- iOS native MQTT worker code has been removed from the active plugin path.

Embedded Rust entrypoints now exist for mobile migration:

- C ABI: `client_core_v2_service_request_json`
- Android JNI: `SlanNativeBridge.serviceRequest(...)`
- Android plugin method: `embeddedServiceRequest`
- iOS plugin method: `embeddedServiceRequest`.
- iOS xcframework build script:
  `client_v2/scripts/build_ios_ffi_xcframework.sh`

The embedded surface supports login/session/local state/logout/refresh,
device registration, MQTT connect, business-event watch, runtime-state ingest,
client messaging, and platform network config generation. Remaining validation
is Mac/iOS simulator device-pair delivery and iOS PacketTunnel config reload
after relay policy changes.
