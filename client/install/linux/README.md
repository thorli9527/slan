# Linux Install Policy

Linux tray support is selected during installation because desktop environments differ in tray/AppIndicator support.

```sh
./install.sh --tray=disabled
./install.sh --tray=enabled
```

`--tray=disabled` is the default and installs a service/helper oriented policy. This is the right mode for server-style Linux or desktop environments without a reliable tray.

`--tray=enabled` installs the desktop shell policy. In that mode closing the window hides it to tray, and tray Quit must call `shutdownNetwork` before the Flutter shell exits.

The installer writes:

- `/etc/slan/client-v2-install.env`
- `/etc/slan/client-v2-desktop.policy`

Installer constants shared by packaging, direct install, and console bootstrap
live in `lib/slan-linux-install.sh`. Keep install roots, service names, config
file names, process names, and default network settings there first.

When a downloadable package is available, `install.sh` stops and disables the
existing `slan-client-v2.service`, kills stale `slan_client_v2` and
`client-core-service` processes, removes the old systemd unit, clears the old
application root, extracts the new package, and then enables/restarts the newly
installed service.

Packagers can override paths with:

```sh
SLAN_CONFIG_DIR=/tmp/slan ./install.sh --tray=enabled
./install.sh --root=/usr/lib/slan-client-v2 --config-dir=/etc/slan
```

## Packages

Build the Rust service and Flutter Linux bundle first, then package:

```sh
make client-linux-package
```

Or package existing artifacts directly:

```sh
scripts/package_linux.sh --variant=all --service-bin=/path/to/linux/client-core-service
scripts/package_linux.sh --variant=console --service-bin=/path/to/linux/client-core-service
```

Outputs are written to `client/.tmp/installer/linux`:

- `SLAN-Client-V2-linux-amd64.tar.gz`
- `SLAN-Client-V2-linux-amd64-console.run`
- `slan-client-v2_<version>_amd64.deb` when `dpkg-deb` is available

The package installs:

- `/opt/slan-client-v2/bin/client-core-service`
- `/opt/slan-client-v2/gui/slan_client_v2` for GUI builds
- `/usr/lib/systemd/system/slan-client-v2.service`
- `/usr/bin/slan-client-v2-console`

The `.deb` lifecycle scripts perform the same reinstall boundary: `preinst`
stops the old UI/service processes and removes the old systemd unit before the
new payload is unpacked, `postinst` reloads systemd and restarts the new service,
and `prerm` stops the runtime during uninstall or replacement.

The systemd service grants `CAP_NET_ADMIN` and `CAP_NET_RAW` so the Linux
backend can create `slan0`, assign the device IP, install routes, and apply DNS
with `resolvectl`. For local development without privileged networking, set:

```sh
SLAN_LINUX_NETWORK_MOCK=1
```

For the stable dual-container real packet-path regression we use:

```sh
scripts/linux_dual_docker_packet_smoke.sh
```

That wrapper enables:

- real packet path: `SLAN_LINUX_DUAL_PACKET_TESTS=1`
- real Linux network mode: `SLAN_LINUX_NETWORK_MOCK=0`
- privileged containers for `/dev/net/tun` and route changes

If you also want the package rebuilt first:

```sh
SLAN_LINUX_DUAL_BUILD_PACKAGE=1 scripts/linux_dual_docker_packet_smoke.sh
```

## Console bootstrap

The single-file console installer requires both the server API address and an
Opt-issued authorization key:

```sh
chmod +x SLAN-Client-V2-linux-amd64-console.run
sudo ./SLAN-Client-V2-linux-amd64-console.run \
  --server-url https://slan.example.com \
  --authorization-key YOUR_AUTHORIZATION_KEY
```

Add `--enable-network` to enable the network immediately after activation. The
installer writes the authorization bootstrap file with mode `0600`; the Rust
runtime removes it after the key has been exchanged successfully.

The console entry accepts the server URL and an Opt-issued device authorization key.

```sh
sudo slan-client-v2-console \
  --server-url http://127.0.0.1:18080 \
  --authorization-key slan_key_example \
  --enable-network
```

For the first installer pass, the script writes `/etc/slan/client-v2-console.env` and restarts `slan-client-v2.service`. The service exchanges the authorization key for a device token and activates the device.

For unattended installs, provide `SLAN_DEVICE_AUTHORIZATION_KEY` or
`--authorization-key`. Create and revoke this key from Opt.

The installer rejects a package built for a different CPU architecture. Use the
`amd64` installer on x86_64 hosts and the `arm64` installer on aarch64 hosts.
