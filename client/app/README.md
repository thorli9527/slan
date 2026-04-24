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
- `SLAN_CONTROL_BASE_URL` is passed through to the helper so Rust `HttpControllerClient` can reach the control plane.

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
- `SLAN_APP_CORE_HELPER_HOST` accepts either `host:port` or `tcp://host:port`.
- Windows runner build copies `app-core-helper.exe` into the app output directory after build.

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
connects to `SLAN_APP_CORE_SERVICE_HOST` or `SLAN_APP_CORE_HELPER_HOST`; if no
helper is listening, it starts `SLAN_APP_CORE_HELPER` or `app-core-helper` from
the app executable directory with `--tcp-host 127.0.0.1:46321`.

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
