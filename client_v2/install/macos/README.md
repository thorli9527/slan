# SLAN Client V2 macOS installer

The macOS installer mirrors the Windows installer boundary:

- The package installs `SLAN Client V2.app` into `/Applications`.
- The package installs `client-core-service` into `/Library/Application Support/SLAN`.
- `postinstall` runs `client-core-service --ensure-device-id` to persist the stable hardware-derived device ID before the service starts.
- `postinstall` installs and starts launchd label `dev.slan.client-core-service`.
- Device install registration does not require login: `client-core-service` retries `/devices/install-register` on startup until the stable device ID exists in server-biz.
- Browser login later binds the same stable device ID to the authenticated user and provisions MQTT/network state.

Build:

```sh
make client-macos-package
```

Install:

```sh
sudo installer -pkg client_v2/.tmp/installer/macos/SLAN-Client-V2-macos.pkg -target /
```
