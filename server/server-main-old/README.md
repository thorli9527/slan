# server-main

`server-main` 是独立的运营后台轻界面，面向 `server-biz` 的 ops HTTP 实例。

- 本地 Docker 默认端口：`24201`
- 本地 Caddy 入口：`https://main.slan.localhost:18443`
- 本地回环入口：`https://127.0.0.1:18443`、`https://localhost:18443`
- API 代理前缀：`/ops-api`
- 后端目标：`server-biz:8081`

登录方式：

- 管理员账号密码：调用 `/ops-api/login`
- 应急 token：可直接填入 `SLAN_OPS_ACCESS_TOKEN`
