# server-wire-derp docs

`server-wire-derp` 是新系统的 DERP over TCP/TLS 443 最终兜底数据面。

- [需求文档](./requirements.md)
- [接口文档](./interfaces.md)

阅读顺序：

1. 先看“需求文档”，确认它只承担最终兜底中继职责
2. 再看“接口文档”，确认客户端、`server-wire` 与管理面如何交互
