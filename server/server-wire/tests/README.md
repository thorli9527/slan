# server-wire Test Engineering

- 业务实现与贴代码单元测试：
  [`source`](/Users/thorli/workspace/slan/slan/server/server-wire/tests/source)
- 独立外部测试工程入口：
  [`external`](/Users/thorli/workspace/slan/slan/server/server-wire/tests/external)

说明：

- `internal/` 中的 `*_test.go` 继续保留在业务模块旁边。
- 跨子系统 smoke / wire 栈验证放在外部测试工程入口下统一查看。
