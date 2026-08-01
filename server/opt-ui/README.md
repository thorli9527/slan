# SLAN Operations Console

运营管理控制台，面向平台运营人员。

## 功能范围

- 运营用户管理
- 中继节点管理
- 全局用户管理与密码重置
- 全局设备与设备分组管理
- 全局网络管理
- 网络设备、引用分组、安全组和 DNS 管理
- 运营审计

所有 `/api/ops/*` 资源接口均要求运营端 Bearer Token。用户、设备、分组和网络不再挂在独立客户 Web UI 下。

## 开发

```bash
npm install
npm start
```
