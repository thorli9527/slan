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
