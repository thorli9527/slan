# SLAN 文档索引

当前文档以新系统切割为准：

- `service-biz`：业务控制面
- `service-ui`：客户 Web Console
- `server-main`：运营管理控制台
- `server-wire`：联网控制面
- `server-wire-relay`：UDP relay 数据面
- `server-wire-derp`：TCP/TLS 443 最终兜底数据面

## 子系统文档

- [service-biz README](../server/service-biz/README.md)
- [service-ui](../server/service-ui)
- [server-main](../server/server-main)
- [server-wire docs](../server/server-wire/docs/README.md)
- [server-wire-relay docs](../server/server-wire-relay/docs/README.md)
- [server-wire-derp docs](../server/server-wire-derp/docs/README.md)
- [Wire client protocol](./wire-client-protocol.md)

## 说明

- 原有 `server-relay` 子系统已删除，新系统统一使用 `server-wire-relay` 与 `server-wire-derp`
- 原有 `service-biz`、`service-ui-old`、`server-main-old` 已删除，新入口统一使用 `service-biz`、`service-ui`、`server-main`
