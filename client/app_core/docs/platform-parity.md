# Platform parity matrix

SLAN is an overlay mesh networking client, similar in product shape to a private tailnet.
It is not positioned as a traditional VPN. Platform APIs such as Android `VpnService`
and Apple Network Extension are packet-routing primitives used to implement the local
interface; user-facing copy should describe SLAN as private networking, mesh networking,
overlay networking, or device-to-device access.

| Capability | Windows | macOS | Linux | Android | iOS |
| --- | --- | --- | --- | --- | --- |
| Flutter MethodChannel `slan/app_core` | Complete | Complete | Complete | Bridge scaffold | Plugin scaffold |
| Control-plane RPCs | Windows service JSON bridge | Rust helper / TCP helper | Rust helper / TCP helper | JSON-line TCP bridge when a foreground service exposes `dev.slan.app_core.SERVICE_HOST` | JSON-line TCP bridge when the app or extension exposes `SLANAppCoreServiceHost` |
| MQTT control messages | Rust app-core service | Rust helper path available | Rust helper path available | Pending foreground service | Pending app/extension bridge |
| XML control task queue | Rust app-core service | Rust helper compatible | Rust helper compatible | Pending foreground service storage | Pending app group storage |
| Network enable / disable | Wintun service | PacketTunnel or helper-host | Linux WireGuard backend | Pending overlay packet service (`VpnService`) | Pending overlay packet extension (Network Extension) |
| Runtime heartbeat | Service reports applied state | Helper/plugin can report applied state | Helper can report applied state | Pending foreground service | Pending app/extension bridge |
| Installer / service lifecycle | Inno + Windows service | App bundle + Network Extension signing | Package/systemd script needed | Android foreground service needed | iOS entitlements/provisioning needed |
| `platformDoctor` / install plan | Complete | Complete | Complete via helper | Implemented with bridge checks | Implemented scaffold |

## Next platform work

- Android: implement a foreground app-core service that owns MQTT, XML task persistence, heartbeat, and packet routing through Android `VpnService`; expose its JSON-line control endpoint through manifest meta-data `dev.slan.app_core.SERVICE_HOST`.
- iOS: add a Runner iOS target, packet-routing Network Extension target, app group storage for queued control tasks, and a durable bridge provider behind `SLANAppCoreServiceHost`.
- Linux: add installer/systemd packaging so the helper/service lifecycle matches Windows service reliability.
- macOS: verify signing/provisioning for PacketTunnel and decide when to use PacketTunnel vs Rust helper-host for tunnel actions.
