# SLAN Client V2 macOS installer

The macOS installer mirrors the Windows installer boundary:

- The package installs `SLAN Client V2.app` into `/Applications`.
- The package installs `client-core-service` into `/Library/Application Support/SLAN`.
- The package installs LaunchAgent `dev.slan.client-v2` to run the menu bar app in the logged-in user session.
- `preinstall` stops and disables old launchd jobs, kills stale `slan_client_v2`/`client-core-service` processes, removes old launchd plist files, removes the old service binary, removes the old PID file, and removes the old app bundle before the new payload is installed.
- `postinstall` defensively stops any old launchd jobs and stale UI/service processes again. Installation does not create or register a device; `client-core-service` creates the local UUID on its first runtime start. Upgrades preserve the existing device identity, while a full uninstall/reset removes local state.
- `postinstall` installs and starts launchd label `dev.slan.client-core-service`.
- Device activation uses an Opt-issued authorization key and persists only the resulting device session.
- An Opt-issued authorization Key activates the installed device ID and provisions MQTT/network state.

Build:

```sh
make client-macos-package
```

Install:

```sh
sudo installer -pkg client/.tmp/installer/macos/SLAN-Client-V2-macos.pkg -target /
```
