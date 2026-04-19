# macOS Runner Notes

当前 macOS 宿主工程位于本目录。

Packet Tunnel Provider 接入规划见：

- [docs/architecture/macos-packet-tunnel-provider-plan.md](../../../docs/architecture/macos-packet-tunnel-provider-plan.md)

当前结论：

- 若目标包含非 Mac App Store 分发，Packet Tunnel Provider 应按 `system extension` 路线设计
- `Runner` 负责 `NETunnelProviderManager` 和 session lifecycle
- 后续新增的 `PacketTunnel` target 才是真正持有 `NEPacketTunnelProvider` 与 utun 的位置
