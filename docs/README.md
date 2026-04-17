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
- [系统边界与接口矩阵](./architecture/system-boundary-matrix.md)
- [.gitignore 约定](./architecture/gitignore.md)

## 当前约定

- 客户端一级目录为 `client/`
- 服务端一级目录为 `server/`
- 客户端核心目录为 `client/app_core`
- 运营平台目录目标命名为 `server/server-ops`

## 仓库现状与目标命名映射

当前仓库中已存在旧骨架目录，后续应逐步对齐到目标命名：

- `server/server-ui` -> `server/server-ops`

当前设计文档统一以仓库实际目录为准，避免继续扩散旧名称。
