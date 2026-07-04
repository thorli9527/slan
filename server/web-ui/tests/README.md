# web-ui Test Engineering

- 业务前端源码：
  [`source`](/Users/thorli/workspace/slan/slan/server/web-ui/tests/source)
- 独立外部测试工程入口：
  [`external`](/Users/thorli/workspace/slan/slan/server/web-ui/tests/external)

说明：

- 浏览器 UI smoke 与外部校验入口从业务源码目录中抽离到 `tests` 视图。
- 业务前端实现仍按 Angular 工程默认结构保留在 `src/`。
