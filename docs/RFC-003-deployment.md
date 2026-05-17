# RFC-003：部署指南

| 字段 | 内容 |
|------|------|
| 状态 | Draft |
| 依赖 | RFC-001 |
| 日期 | 2026-05-16 |

---

## 1. 部署模式

| 模式 | 命令 | 适用 |
|------|------|------|
| **nginx**（默认） | `make deploy` | 前后端可独立更新 |
| **embed** | `DEPLOY_MODE=embed make build` | 单二进制、无 Nginx |

环境变量 `DEPLOY_MODE`：`nginx` | `embed`。

---

## 2. nginx 模式（推荐）

### 2.1 服务器目录

```
/opt/marketpulse/
  bin/marketd
  config/config.yaml
/var/www/marketpulse/          # 前端 dist
```

### 2.2 构建

```bash
make web          # 产出 web/dist
make api          # 产出 bin/marketd
```

### 2.3 发布

```bash
# 仅前端
make deploy-web DEPLOY_HOST=user@your-hk-vps

# 仅后端
make deploy-api DEPLOY_HOST=user@your-hk-vps

# 全量
make deploy DEPLOY_HOST=user@your-hk-vps
```

### 2.4 Nginx 要点

- `/` → `/var/www/marketpulse/`
- `/api/`、`/ws/`、`/healthz` → `http://127.0.0.1:8080`
- WebSocket：`proxy_http_version 1.1`、`Upgrade`、`Connection upgrade`
- 模板：`deploy/nginx.conf.example`

### 2.5 systemd

- 模板：`deploy/marketpulse.service.example`
- 仅重启 `marketd`；改静态 **无需** restart

---

## 3. embed 模式

```bash
DEPLOY_MODE=embed make build
scp bin/marketd ...
systemctl restart marketpulse
```

Go 通过 `//go:embed` 提供 `web/dist`，单端口对外。

---

## 4. 本地开发

```bash
make dev-api    # :8080
make dev-web    # :5173 → proxy api/ws
```

---

## 5. 上线检查

```bash
./scripts/check-connectivity.sh
curl -s https://your-domain/healthz | jq .
```

---

## 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 0.1 | 2026-05-16 | 草案 |
