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
client_v2/install/linux/package-linux.sh --variant=all
client_v2/install/linux/package-linux.sh --variant=console
```

On macOS or another non-Linux host, build the Linux bundle in Docker:

```sh
make client-linux-docker-package
```

This builds `client-core-service` and the Flutter Linux GUI inside a `linux/amd64` container, then calls `package-linux.sh`.

Outputs are written to `client_v2/.tmp/installer/linux`:

- `SLAN-Client-V2-linux-x64.tar.gz`
- `slan-client-v2_<version>_amd64.deb` when `dpkg-deb` is available

The package installs:

- `/opt/slan-client-v2/bin/client-core-service`
- `/opt/slan-client-v2/gui/slan_client_v2` for GUI builds
- `/lib/systemd/system/slan-client-v2.service`
- `/usr/bin/slan-client-v2-console`

## Console bootstrap

The console entry accepts server and invite parameters without showing Web Console:

```sh
sudo slan-client-v2-console \
  --server-url http://127.0.0.1:18080 \
  --email user@example.com \
  --password secret \
  --join-key 0123456789abcdef0123456789abcdef \
  --enable-network
```

`--invite` is an alias for `--join-key` and can parse invite URLs containing `joinKey`, `code`, or `key`.

For the first installer pass, the script writes `/etc/slan/client-v2-console.env` and restarts `slan-client-v2.service`. The next core-service command should consume these pending values and call `/networks/join-by-key` after password login.
