# 子项目测试工程拆分

这份文档描述“业务代码工程”和“测试代码工程”在当前仓库中的拆分方式。

## 拆分原则

- 贴语言生态的单元测试继续留在业务模块旁边
  - Go: `*_test.go`
  - Flutter: `test/`
  - Flutter 集成测试: `integration_test/`
- 独立运行的 smoke / e2e / matrix / 平台回归脚本抽到测试工程视图
- 每个主要子项目都提供自己的 `tests/` 入口，便于从子项目维度查看测试体系

## client

- 业务工程：
  [`client/app_flutter`](/Users/thorli/workspace/slan/slan/client/app_flutter)
  [`client/plugins/client_core_plugin`](/Users/thorli/workspace/slan/slan/client/plugins/client_core_plugin)
  [`client/rust/crates`](/Users/thorli/workspace/slan/slan/client/rust/crates)
- 测试工程入口：
  [`client/tests`](/Users/thorli/workspace/slan/slan/client/tests)

## service-biz

- 业务工程：
  [`server/service-biz/internal`](/Users/thorli/workspace/slan/slan/server/service-biz/internal)
- 测试工程入口：
  [`server/service-biz/tests`](/Users/thorli/workspace/slan/slan/server/service-biz/tests)

## server-wire 系列

- 业务工程：
  [`server/server-wire/internal`](/Users/thorli/workspace/slan/slan/server/server-wire/internal)
  [`server/server-wire-relay/internal`](/Users/thorli/workspace/slan/slan/server/server-wire-relay/internal)
  [`server/server-wire-derp/internal`](/Users/thorli/workspace/slan/slan/server/server-wire-derp/internal)
  [`server/server-wire-punch/internal`](/Users/thorli/workspace/slan/slan/server/server-wire-punch/internal)
- 测试工程入口：
  [`server/server-wire/tests`](/Users/thorli/workspace/slan/slan/server/server-wire/tests)
  [`server/server-wire-relay/tests`](/Users/thorli/workspace/slan/slan/server/server-wire-relay/tests)
  [`server/server-wire-derp/tests`](/Users/thorli/workspace/slan/slan/server/server-wire-derp/tests)
  [`server/server-wire-punch/tests`](/Users/thorli/workspace/slan/slan/server/server-wire-punch/tests)

## Ops 前端

- 业务工程：
  [`server/opt-ui/src`](/Users/thorli/workspace/slan/slan/server/opt-ui/src)
- 测试工程入口：
  [`server/opt-ui/tests`](/Users/thorli/workspace/slan/slan/server/opt-ui/tests)

## 外部测试脚本总入口

全仓库共用的外部测试实现目录仍保留在：

- [`scripts/tests`](/Users/thorli/workspace/slan/slan/scripts/tests)

但现在各子项目也都有自己的 `tests/` 入口视图，从“子项目工程”角度可以直接进入对应测试工程。
