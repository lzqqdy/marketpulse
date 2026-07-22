# Docker 部署（运维速查）

正式说明以 **[docs/RFC-003-deployment.md §1.1](../docs/RFC-003-deployment.md)** 为准。本文保留常用命令，便于在 `deploy/` 目录内快速查阅。

单容器镜像含 `marketd` + 前端 `web/dist`，默认 `:8080`。MySQL / Redis 用 Compose profile `db` 可选接入（默认关）。

## 仅行情

```bash
cp .env.example .env   # 可选
docker compose up -d --build
# 或: make docker-up

curl -s http://127.0.0.1:8080/healthz
# 浏览器 http://127.0.0.1:8080/
```

```bash
docker compose down
```

## 行情 + MySQL + Redis

```bash
make docker-up-db
# 等价: MYSQL_ENABLED=true REDIS_ENABLED=true docker compose --profile db up -d --build
```

| 项 | 默认 |
|----|------|
| MySQL | 库/用户 `marketpulse`，口令见 `.env` |
| Redis | `redis:6379` |
| 连接开关 | `MYSQL_ENABLED` / `REDIS_ENABLED`（经 `MARKETPULSE_*` 传入进程） |

回滚：关掉 enabled 后重启，或去掉 `--profile db`。

## 覆盖配置

```bash
cp config/config.docker.yaml config/config.yaml
# 编辑后在 docker-compose.yml 取消注释 volume 挂载
```

密钥优先放 `.env`，勿提交。

## Make

```bash
make docker-build
make docker-up
make docker-up-db
make docker-down
make docker-logs
```

## 文件索引

| 文件 | 说明 |
|------|------|
| `Dockerfile` | 多阶段构建 |
| `docker-compose.yml` | 编排 + `db` profile |
| `config/config.docker.yaml` | 容器默认配置 |
| `.env.example` | 环境变量模板 |
| `.dockerignore` | 构建上下文排除 |

## 注意

- 容器需访问外网数据源；国内可调 `ingest.binance.ws_base`
- 日志卷：`marketpulse-log` → `/app/log`
- 上传卷：`marketpulse-uploads` → `/app/data/uploads`（头像等；`down -v` 会清空）
- 与 `make ship` 并行，互不替代
