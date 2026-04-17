# server-biz

Phase 1 MVP 的 Go 控制面服务。

## 当前职责

- 用户注册与登录
- 设备注册
- 网络创建、加入与成员管理
- 子网与虚拟 IP 分配
- 控制通道 bootstrap 配置下发
- relay 票据签发

## 目录说明

- `cmd/biz-server`：进程入口
- `api/http`：HTTP 路由层
- `api/dto`：请求响应模型
- `internal/*`：领域服务边界与实现

## 文档

- [文档索引](./docs/README.md)
- [内部需求](./docs/internal-requirements.md)
- [对外输出功能](./docs/exported-capabilities.md)
- [对外接入接口](./docs/integration-interfaces.md)
