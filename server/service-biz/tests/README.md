# service-biz Test Engineering

`service-biz` 的业务实现保留在：

- [`source`](/Users/thorli/workspace/slan/slan/server/service-biz/tests/source)

其中贴代码的 Go 单元测试与仓储/用例测试继续跟随 `internal/` 中的包组织，
例如 `*_test.go`。

独立于业务工程的外部测试入口收敛到：

- [`external`](/Users/thorli/workspace/slan/slan/server/service-biz/tests/external)

这里主要包括：

- service-biz smoke
- remote smoke
- API / app / ACL / DNS 相关集成校验
- 与其它子系统联动的端到端验证入口
