# slan_app_core_plugin_macos

macOS implementation of the federated `slan_app_core_plugin`.

## Helper transport

By default, bridge calls start the bundled `app-core-helper` or the executable
configured by `SLAN_APP_CORE_HELPER` and communicate over stdio.

Set `SLAN_APP_CORE_SERVICE_HOST` or `SLAN_APP_CORE_HELPER_HOST` to
`host:port` or `tcp://host:port` to connect to an already-running TCP helper
instead. In that mode the plugin does not start a local helper process.

## Local native verification

本地测试/构建入口已经统一整理在：

- [docs/architecture/client-local-test-entrypoints.md](../../docs/architecture/client-local-test-entrypoints.md)

这层最常用的两个入口是：

```bash
make macos-tunnel-control-test
make macos-packet-tunnel-build-check
```

如果后面要跑完整 Flutter macOS integration，再先执行：

```bash
make macos-packet-tunnel-signing-check
```

What this currently validates:

- the `PacketTunnel` app extension target still compiles and links
- `PacketTunnelProvider.swift` can consume the shared
  `PacketTunnelProviderSupport.swift` source file
- the target can be built without pulling `Runner` / `Flutter Assemble` into the
  dependency graph

This does not replace full Xcode/Flutter macOS builds. It is a smaller native
build loop for the macOS PacketTunnel extension target.
