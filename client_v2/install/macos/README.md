# SLAN Client V2 macOS installer

The macOS installer mirrors the Windows installer boundary:

- The package installs `SLAN Client V2.app` into `/Applications`.
- The package installs `client-core-service` into `/Library/Application Support/SLAN`.
- `postinstall` runs `client-core-service --ensure-device-id` to persist the stable hardware-derived device ID before the service starts.
- `postinstall` installs and starts launchd label `dev.slan.client-core-service`.
- Device registration still belongs to `client-core-service`: on service startup it re-registers an existing valid session, and after browser login it registers the stable device ID returned by `--ensure-device-id`.

Build:

```sh
make client-macos-package
```

Install:

```sh
sudo installer -pkg client_v2/.tmp/installer/macos/SLAN-Client-V2-macos.pkg -target /
```
