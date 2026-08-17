# 微信云托管部署（本分支）

本分支把 `marketd` + 前端 `web/dist` 打进**一个容器**，监听 **80**。密钥不要进 Git，写在控制台环境变量里。

## 控制台

1. 新建服务，代码源选本仓库 **`wxcloudrun` 分支**，Dockerfile 用仓库根目录。
2. **监听端口：`80`**（与镜像默认一致；也可填控制台注入的 `PORT`）。
3. 实例副本：**最小 1、最大 1**（行情在进程内存里，且 ingest 是后台长连接，缩到 0 会断流）。
4. 规格建议 ≥ 0.5 核 / 1G；打开公网访问。
5. 健康检查路径：`/health` 或 `/healthz`。

发布后打开公网域名：页面和 `/api`、`/ws` 同源，前端不用改。

## 环境变量（密钥只放这里）

第一期只上行情，可以不配。常用项：

| 变量 | 说明 |
|------|------|
| `PORT` | 可选，默认 80 |
| `MYSQL_ADDRESS` / `MYSQL_USERNAME` / `MYSQL_PASSWORD` / `MYSQL_DATABASE` | 云托管 MySQL 常自动注入 |
| `MARKETPULSE_MYSQL_ENABLED` | 开用户模块时设 `true` |
| `MARKETPULSE_REDIS_ENABLED` / `MARKETPULSE_REDIS_ADDR` / `MARKETPULSE_REDIS_PASSWORD` | Redis 用腾讯云上海 + 资源互联 |
| `MARKETPULSE_USERS_ENABLED` | 需 MySQL + Redis |
| `MARKETPULSE_USERS_SEED_PHONE` / `MARKETPULSE_USERS_SEED_PASSWORD` | 初始账号 |
| `MARKETPULSE_ALERTS_ENABLED` / `MARKETPULSE_PORTFOLIO_ENABLED` | 告警 / 资产 |
| `MARKETPULSE_AI_API_KEY` / `MARKETPULSE_SMTP_*` | AI / 邮件 |

改环境变量后重新发布或重启才生效。

## 验收

```bash
curl -s https://<公网域名>/health
curl -s https://<公网域名>/healthz
```

浏览器打开根路径应看到看板；`healthz` 里 ingest 状态应为 connected（视出网而定）。
