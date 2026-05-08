# server-biz docs

`server-biz` 在新系统里只承担业务控制面职责。

- [需求文档](./requirements.md)
- [接口文档](./interfaces.md)
- [公共错误码](./public-error-codes.md)

阅读顺序：

1. 先看“需求文档”，确认它只负责身份、资源和授权
2. 再看“接口文档”，确认它对客户端和 `server-wire` 暴露什么
