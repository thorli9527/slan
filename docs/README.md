# SLAN 文档索引

当前文档以新系统切割为准：

- `service-biz`：业务控制面
- `opt-ui`：运营管理控制台
- `server-wire`：联网控制面
- `server-wire-relay`：UDP relay 数据面
- `server-wire-derp`：DERP 最终兜底数据面；当前实现为裸 TCP JSON-lines，生产 443/TLS 需要外部四层/TLS 终止或后续内置 TLS transport

## 子系统文档

- [后端外部 HTTP 业务接口定义](./backend-external-http-api.md)
- [service-biz OpenAPI 草案](../protocol/openapi/service-biz-external.yaml)
- [service-biz README](../server/service-biz/README.md)
- [opt-ui](../server/opt-ui)
- [server-wire docs](../server/server-wire/docs/README.md)
- [server-wire-relay docs](../server/server-wire-relay/docs/README.md)
- [server-wire-derp docs](../server/server-wire-derp/docs/README.md)
- [Wire client protocol](./wire-client-protocol.md)
- [client-v2 多端联调与双 iOS 最小回归](./client-v2-multidevice-dev.md)
- [客户端多平台验证矩阵](./client-multi-platform-validation-matrix.md)
- [测试源码布局](./test-source-layout.md)
- [子项目测试工程拆分](./subproject-test-engineering-split.md)

## 说明

- 原有 `server-relay` 子系统已删除，新系统统一使用 `server-wire-relay` 与 `server-wire-derp`
