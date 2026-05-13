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

Outputs are written to `client_v2/.tmp/installer/linux`:

- `SLAN-Client-V2-linux-x64.tar.gz`
- `slan-client-v2_<version>_amd64.deb` when `dpkg-deb` is available

The package installs:

- `/opt/slan-client-v2/bin/client-core-service`
- `/opt/slan-client-v2/gui/slan_client_v2` for GUI builds
- `/lib/systemd/system/slan-client-v2.service`
- `/usr/bin/slan-client-v2-console`

The systemd service grants `CAP_NET_ADMIN` and `CAP_NET_RAW` so the Linux
backend can create `slan0`, assign the device IP, install routes, and apply DNS
with `resolvectl`. For local development without privileged networking, set:

```sh
SLAN_LINUX_NETWORK_MOCK=1
```

## Console bootstrap

The console entry accepts server and login parameters without showing Web Console.
Invitation and access-code flows are only available in the Web Console.

```sh
sudo slan-client-v2-console \
  --server-url http://127.0.0.1:18080 \
  --email user@example.com \
  --password secret \
  --enable-network
```

For the first installer pass, the script writes `/etc/slan/client-v2-console.env` and restarts `slan-client-v2.service`. Device access-code generation and confirmation should be completed from Web Console before the client logs in.
