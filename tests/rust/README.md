# Rust Tests

这个子项目用于承接仓库内 Rust 代码的测试用例，和业务 crate 平行维护。

当前包含：

- `app-core-tests`
- `server-relay-tests`

运行方式：

```bash
cargo test
```

目录原则：

- 业务 crate 不再内嵌 `#[cfg(test)]` 测试模块
- 测试代码统一放在这里
- 测试项目通过 path dependency 引用真实业务 crate
