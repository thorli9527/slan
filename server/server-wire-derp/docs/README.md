# server-wire-derp docs

`server-wire-derp` 是新系统的 DERP 最终兜底数据面。当前实现为裸 TCP JSON-lines；生产 443/TLS 需要外部四层/TLS 终止或后续内置 TLS transport。

- [需求文档](./requirements.md)
- [接口文档](./interfaces.md)

阅读顺序：

1. 先看“需求文档”，确认它只承担最终兜底中继职责
2. 再看“接口文档”，确认客户端、`server-wire` 与管理面如何交互
