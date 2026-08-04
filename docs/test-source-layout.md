# 测试源码布局

这份文档描述当前仓库中测试源码和测试脚本的主要归类方式。

## 语言原生测试目录

- Flutter 单元测试：
  [`client/app_flutter/test`](/Users/thorli/workspace/slan/slan/client/app_flutter/test)
- Flutter 集成测试：
  [`client/app_flutter/integration_test`](/Users/thorli/workspace/slan/slan/client/app_flutter/integration_test)
- Go / Rust 服务端测试：
  继续按语言惯例保留在各自模块内，例如 `*_test.go`、crate 内测试模块。

这些目录本身已经符合各自生态习惯，所以本次重构没有硬搬迁。

## 脚本测试目录

历史上大量 smoke / matrix / check / integration 脚本都堆在
[`scripts/`](/Users/thorli/workspace/slan/slan/scripts) 根目录下。现在已把真实实现归类到：

- [`scripts/tests/android`](/Users/thorli/workspace/slan/slan/scripts/tests/android)
- [`scripts/tests/ios`](/Users/thorli/workspace/slan/slan/scripts/tests/ios)
- [`scripts/tests/linux`](/Users/thorli/workspace/slan/slan/scripts/tests/linux)
- [`scripts/tests/macos`](/Users/thorli/workspace/slan/slan/scripts/tests/macos)
- [`scripts/tests/matrix`](/Users/thorli/workspace/slan/slan/scripts/tests/matrix)
- [`scripts/tests/backend`](/Users/thorli/workspace/slan/slan/scripts/tests/backend)
- [`scripts/tests/wire`](/Users/thorli/workspace/slan/slan/scripts/tests/wire)
- [`scripts/tests/ui`](/Users/thorli/workspace/slan/slan/scripts/tests/ui)
- [`scripts/tests/shared`](/Users/thorli/workspace/slan/slan/scripts/tests/shared)
- [`scripts/tests/guard`](/Users/thorli/workspace/slan/slan/scripts/tests/guard)

兼容策略：

- 根目录 `scripts/<name>` 入口仍然保留，用软链指向新的分类目录。
- 这样现有 Makefile、文档、CI、人工操作命令暂时无需一起改动。

## 后续约定

- 新增测试脚本优先放入 `scripts/tests/<category>/`
- 仅当脚本需要成为稳定入口时，再在 `scripts/` 增加同名入口
- 涉及多端联调时，优先放到 `matrix/`
- 可复用的 Go helper、shell helper 放到 `shared/`

## 不纳入本次重构的目录

以下内容未作为“测试源码”整理对象：

- `build/`、`build.rootcache.*` 等编译产物
- `node_modules/` 内第三方依赖自带测试目录
- `package_*`、`deploy`、`install`、`setup` 这类非测试主职责脚本
