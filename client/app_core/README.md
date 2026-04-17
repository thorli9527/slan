# app_core

SLAN Phase 1 的 Rust 应用核心层。

## 作用

- 对接 `server-biz` 的控制面接口
- 完成设备注册与启动配置拉取
- 执行 NAT 类型探测
- 优先尝试 P2P 连接
- 在直连失败时请求 relay 回退
- 向 Flutter 暴露稳定的门面接口

## crate 划分

- `app-core`：共享领域模型定义
- `controller-client`：控制面客户端 trait 与请求模型
- `nat`：NAT 探测抽象
- `p2p`：点对点连接抽象
- `relay-client`：relay 回退连接抽象
- `tunnel`：加密隧道抽象
- `ffi-bridge`：面向 Flutter 的统一门面

## 文档

- [文档索引](./docs/README.md)
- [内部需求](./docs/internal-requirements.md)
- [对外输出功能](./docs/exported-capabilities.md)
- [对外接入接口](./docs/integration-interfaces.md)
