# server-relay

Phase 1 MVP 的 Rust 中继数据面服务。

## 当前职责

- 校验控制面签发的 relay ticket
- 创建并维护 relay session
- 转发双方设备之间的 UDP 负载
- 维护会话保活与过期控制

## 文档

- [文档索引](./docs/README.md)
- [内部需求](./docs/internal-requirements.md)
- [对外输出功能](./docs/exported-capabilities.md)
- [对外接入接口](./docs/integration-interfaces.md)
