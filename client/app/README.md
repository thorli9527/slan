# slan_app

Flutter desktop client for SLAN.

## AppCore Modes

Default mode stays on local mock APIs.

To use the real control plane over HTTP:

```bash
flutter run -d macos \
  --dart-define=SLAN_CONTROL_BASE_URL=http://127.0.0.1:8080
```

To use Rust facade bridge mode:

1. Build the helper first:

```bash
cd ../app_core
cargo build -p app-core-helper
```

2. Start the desktop app with the helper path and bridge mode:

```bash
cd ../app
SLAN_APP_CORE_HELPER=/path/to/client/app_core/target/debug/app-core-helper \
SLAN_CONTROL_BASE_URL=http://127.0.0.1:8080 \
flutter run -d macos --dart-define=SLAN_APP_CORE_MODE=bridge
```

Notes:

- `SLAN_APP_CORE_MODE=bridge` makes Dart call `MethodChannel('slan/app_core')`.
- `SLAN_APP_CORE_HELPER` points to the Rust helper executable.
- On macOS, `SLAN_APP_CORE_SERVICE_HOST` or `SLAN_APP_CORE_HELPER_HOST` makes
  bridge mode use an already-running TCP helper instead of launching the local
  helper process.
- `SLAN_CONTROL_BASE_URL` is passed through to the helper so Rust `HttpControllerClient` can reach the control plane.

## Main Client Flow

The desktop app keeps the public client flow aligned across mock, HTTP, and
bridge modes:

1. Authenticate or complete the browser callback.
2. Register or recover the local device.
3. Register the local node.
4. Create an owned network, or join another network by join key.
5. Optionally persist the device alias through attachment remark.
6. Switch the selected network and activate the local device on that network.
7. Bootstrap app_core with the selected `nodeId` and `networkId`.
8. Connect to peers, trying direct paths first and using relay/DERP fallback when needed.

The network page exposes join-by-key, alias, refresh, and network switching
controls. In bridge/service mode, the home and device pages only trigger service
actions such as enable, sync, disable, connect, probe, and send. The actual
tunnel, DNS, MQTT control sync, and network-state heartbeat are owned by
`app-core-service` and `app-core-helper`.

The device page still keeps the legacy WireGuard tunnel debug form for non-bridge
development modes. In bridge/service mode that form is hidden; the page shows
only service runtime controls and status views so Flutter stays a UI/action
layer instead of mutating local tunnel configuration directly.

## Tunnel Host Modes

Tunnel backend selection is controlled by Dart defines:

- default `plugin`: use the platform plugin tunnel host directly
- `service`, `helper`, or `helper-host`: connect to a Rust helper host over TCP

Example desktop run using a TCP helper host:

```bash
flutter run -d windows \
  --dart-define=SLAN_CONTROL_BASE_URL=http://127.0.0.1:28080 \
  --dart-define=SLAN_TUNNEL_HOST_MODE=service \
  --dart-define=SLAN_APP_CORE_HELPER_HOST=127.0.0.1:46321
```

Start the Rust helper host like this:

```bash
cd ../app_core
SLAN_CONTROL_BASE_URL=http://127.0.0.1:28080 \
cargo run -q -p app-core-helper -- --tcp-host 127.0.0.1:46321
```

Notes:

- Empty `SLAN_TUNNEL_HOST_MODE` falls back to `plugin`.
- `SLAN_APP_CORE_HELPER_HOST` and `SLAN_APP_CORE_SERVICE_HOST` accept either
  `host:port` or `tcp://host:port` as Dart defines for the Dart tunnel gateway.
- `SLAN_APP_CORE_SERVICE_HOST` is for an already-running external helper or
  service. Native desktop plugins also read it from the process environment and
  connect to it without trying to start local helper processes or OS services.
- Windows runner build copies `app-core-helper.exe` into the app output directory after build.

## Bridge Runtime Boundary

When `SLAN_APP_CORE_MODE=bridge` is enabled:

- Flutter persists the user's last requested enable/disable choice.
- A valid restored session may ask `app-core-service` to re-enable the last
  selected network.
- `Enable Network`, `Sync State`, and `Disable Network` call the app-core
  bridge APIs.
- Flutter does not apply WireGuard configuration, start/stop local DNS, or own
  the 15-second network-state heartbeat.
- Device-page tunnel/IP/key/endpoint inputs are hidden because those values are
  derived and applied by the service/helper runtime.

## Linux Helper In Docker From Windows

When developing on Windows but running the Linux client core in Docker, build the
Linux helper inside Docker:

```powershell
docker build -f client/app_core/Dockerfile.linux-helper -t slan-linux-helper client/app_core
```

Run it with network-administration capability so the container can create the
WireGuard interface:

```powershell
docker run --rm -it `
  --name slan-linux-helper `
  --cap-add NET_ADMIN `
  --device /dev/net/tun `
  -e SLAN_CONTROL_BASE_URL=http://host.docker.internal:28080 `
  -e SLAN_TUNNEL_DRIVER=linux-kernel `
  -e SLAN_LINUX_EXECUTOR=shell `
  -p 46321:46321 `
  slan-linux-helper
```

For a privileged local smoke test, `--privileged` can replace `--cap-add` and
`--device`, but keep the narrower flags for normal development.

From the Windows Flutter app, point the tunnel host at the container:

```powershell
flutter run -d windows `
  --dart-define=SLAN_CONTROL_BASE_URL=http://127.0.0.1:28080 `
  --dart-define=SLAN_TUNNEL_HOST_MODE=service `
  --dart-define=SLAN_APP_CORE_HELPER_HOST=127.0.0.1:46321
```

The Linux Flutter plugin uses the same helper JSON-line protocol. It first
connects to the process environment variable `SLAN_APP_CORE_SERVICE_HOST` or
`SLAN_APP_CORE_HELPER_HOST`. If `SLAN_APP_CORE_HELPER_HOST` is not listening,
or neither variable is set, it starts `SLAN_APP_CORE_HELPER` or
`app-core-helper` from the app executable directory with
`--tcp-host 127.0.0.1:46321`.

Use `SLAN_APP_CORE_SERVICE_HOST` for an already-running external helper or
container. In that mode the plugin only connects to the service and will not
try to spawn a local helper.

If startup fails, the Linux plugin reports the resolved helper host and helper
path in the Flutter `PlatformException`. This is the first place to check when a
Linux build cannot find `app-core-helper`, cannot execute it, or the helper
exits before opening its TCP listener.

Inside the container, verify adapter creation with:

```bash
ip link show
ip address show
wg show
```

Or run the bundled smoke test from Windows. It checks `platformDoctor` and
`platformInstallPlan`, creates `slan0`, applies a temporary WireGuard
configuration, verifies the address and WireGuard device, then removes the
interface on exit:

```powershell
docker run --rm `
  --cap-add NET_ADMIN `
  --device /dev/net/tun `
  -e SLAN_CONTROL_BASE_URL=http://host.docker.internal:28080 `
  --entrypoint /bin/sh `
  slan-linux-helper `
  /app/scripts/linux-helper-smoke.sh
```

The Linux helper defaults to `SLAN_TUNNEL_DRIVER=linux-kernel`, which uses
kernel WireGuard through `ip` and `wg`. Set `SLAN_TUNNEL_DRIVER=in-memory` only
for logic tests where no real network adapter should be created.

## Local HTTPS Hostnames

The local Docker + Caddy stack serves secure desktop auth flows from:

- `https://slan.localhost:18443`
- `https://web.slan.localhost:18443`

Some Windows setups do not resolve `*.localhost` subdomains automatically. If
`web.slan.localhost` does not resolve on your machine, add these entries to your
hosts file:

```text
127.0.0.1 slan.localhost
127.0.0.1 web.slan.localhost
127.0.0.1 ops.slan.localhost
```

## Auth Callback Notes

Desktop auth callback now prefers the current `deviceId` as the callback key.

That means:

- the web console only needs to carry `deviceId`
- missing `callbackId` falls back to `deviceId`
- repeated login attempts on the same device are treated as fresh callback attempts rather than duplicate stale callbacks

## Local Test Entrypoints

See:

- [docs/architecture/client-local-test-entrypoints.md](../../docs/architecture/client-local-test-entrypoints.md)
- [docs/architecture/local-auth-callback-smoke.md](../../docs/architecture/local-auth-callback-smoke.md)

Common local commands:

```bash
make client-desktop-ui-test
make devices-integration
```
