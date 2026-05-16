# SLAN Client V2 macOS installer

The macOS installer mirrors the Windows installer boundary:

- The package installs `SLAN Client V2.app` into `/Applications`.
- The package installs `client-core-service` into `/Library/Application Support/SLAN`.
- The package installs LaunchAgent `dev.slan.client-v2` to run the menu bar app in the logged-in user session.
- `preinstall` stops and disables old launchd jobs, kills stale `slan_client_v2`/`client-core-service` processes, removes old launchd plist files, removes the old service binary, removes the old PID file, and removes the old app bundle before the new payload is installed.
- `postinstall` defensively stops any old launchd jobs and stale UI/service processes again, then runs `client-core-service --reset-device-id` to generate and persist a UUID device ID before the service starts.
- `postinstall` installs and starts launchd label `dev.slan.client-core-service`.
- Device install registration does not require login: `client-core-service` retries `/devices/install-register` on startup until the installed UUID device ID exists in server-biz.
- Browser login later binds the same installed device ID to the authenticated user and provisions MQTT/network state.

Build:

```sh
make client-macos-package
```

Install:

```sh
sudo installer -pkg client_v2/.tmp/installer/macos/SLAN-Client-V2-macos.pkg -target /
```
