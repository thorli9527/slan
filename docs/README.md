# SLAN 文档索引

本文档目录用于沉淀组网软件的需求、架构和阶段性实施方案。

## 文档列表

- [完整需求范围](./prd/full-scope.md)
- [项目目录与模块设计](./architecture/project-structure.md)
- [一期 MVP 方案](./architecture/phase-1-mvp.md)
- [主要序列图](./architecture/sequence-diagrams.md)
- [Relay 功能梳理](./architecture/relay-flow.md)
- [DERP 集群与连接池设计](./architecture/derp-cluster-design.md)
- [AppCore 内部需求接口](./architecture/app-core-derp-internal-interfaces.md)
- [协议合同清单](./architecture/protocol-contract-inventory.md)
- [Client 重构清单](./architecture/client-refactor-plan.md)
- [Client 本地测试入口](./architecture/client-local-test-entrypoints.md)
- [Client Integration Test Runner](./architecture/client-integration-test-runner.md)
- [macOS Packet Tunnel Provider 接入清单](./architecture/macos-packet-tunnel-provider-plan.md)
- [系统边界与接口矩阵](./architecture/system-boundary-matrix.md)
- [.gitignore 约定](./architecture/gitignore.md)
- [上线准备清单（server-biz）](./architecture/production-readiness-checklist.md)
- [小规模上线检查清单](./architecture/small-scale-rollout-checklist.md)
- [local 到 production 配置迁移说明](./architecture/local-to-production-config.md)
- [Token / 会话设计（生产建议）](./architecture/token-session-design.md)

## 当前约定

- 客户端一级目录为 `client/`
- 服务端一级目录为 `server/`
- 客户端核心目录为 `client/app_core`

当前设计文档统一以当前仓库实际目录为准，不再保留 `server-ops` 目标目录描述。
