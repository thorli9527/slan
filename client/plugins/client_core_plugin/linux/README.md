# Linux Plugin

The Linux plugin mirrors the desktop boundary used by macOS and Windows:

- Flutter sends `dev.slan/client_core_v2` method-channel calls.
- The plugin forwards those calls to `client-core-service` on
  `SLAN_CLIENT_CORE_SERVICE_HOST` or `127.0.0.1:46392`.

System network changes are still owned by `client-core-service` through the
Rust Linux platform backend. The GUI plugin does not touch TUN, routes, DNS, or
privileged operations directly.
