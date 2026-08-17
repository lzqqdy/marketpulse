# 微信云托管部署（本分支）

本分支把 `marketd` + 前端 `web/dist` 打进**一个容器**，监听 **80**。密钥不要进 Git，写在控制台环境变量里。

默认开启：用户中心、价格告警、资产中心、市场异动。会话用进程内 Redis（副本必须 1/1）。AI 需另配 API Key。

## 控制台

1. 代码源选本仓库 **`wxcloudrun` 分支**，Dockerfile 用仓库根目录。
2. **监听端口：`80`**。
3. 实例副本：**最小 1、最大 1**。
4. 规格建议 ≥ 0.5 核 / 1G；打开公网访问。
5. 健康检查：`/health` 或 `/healthz`。

## 环境变量 JSON

控制台「环境变量」打开 JSON，在已有 MySQL / COS 基础上补全（**不要把密码提交到 Git**）：

```json
{
  "COS_BUCKET": "你的桶名",
  "COS_REGION": "ap-shanghai",
  "COS_SECRET_ID": "腾讯云 SecretId，没有则头像走容器本地盘",
  "COS_SECRET_KEY": "腾讯云 SecretKey",
  "MYSQL_ADDRESS": "10.x.x.x:3306",
  "MYSQL_USERNAME": "root",
  "MYSQL_PASSWORD": "控制台里的数据库密码",
  "MYSQL_DATABASE": "marketpulse",
  "MARKETPULSE_USERS_SEED_PHONE": "13800000000",
  "MARKETPULSE_USERS_SEED_PASSWORD": "改成你的登录密码",
  "MARKETPULSE_USERS_SEED_NAME": "管理员"
}
```

`MYSQL_ADDRESS` 有值就会自动开 MySQL，并 `CREATE DATABASE IF NOT EXISTS marketpulse`。

头像走 COS 需要 `COS_SECRET_ID` / `COS_SECRET_KEY`（或 `TENCENTCLOUD_SECRETID` / `TENCENTCLOUD_SECRETKEY`）。只有桶名没有密钥时，头像仍写容器本地，重新发布会丢。

AI 再加：`MARKETPULSE_AI_API_KEY`。邮件告警再加：`MARKETPULSE_SMTP_HOST` 等。

改环境变量后重新发布或重启才生效。

## 验收

```bash
curl -s https://<公网域名>/healthz
```

`users` / `alerts` / `portfolio` / `event` 应为 `enabled`。用 seed 手机号登录用户中心。
