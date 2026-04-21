# server-ui/web

Angular 用户网络控制台。

当前功能：

- 用户登录 / 注册
- 无网络时创建自己的网络
- 按宿主邮箱加入别人的网络
- 切回自己的网络
- 查看当前管理设备
- 当当前网络属于自己时：
  - 修改默认网段
  - 创建额外子网
  - 配置子网 DHCP 范围
  - 查看并修改成员设备绑定的虚拟 IP
- 对 CIDR / DHCP 起止地址做前端校验
- 把后端错误文案整理成页面可读提示

默认服务端地址：

- 同源 `/api/*`
- 本地 Docker 中由 `nginx` 反代到 `server-biz:8080`

本地 Docker / Caddy 入口：

- `https://web.slan.localhost:18443`

启动：

```bash
npm install
npm start
```
