# Docker 部署

单二进制镜像内含 `marketd` + 前端 `web/dist`，默认监听 `8080`。MySQL / Redis 通过 Compose profile 可选接入。

## 快速开始（仅行情）

```bash
# 可选：复制环境变量模板
cp .env.example .env

docker compose up -d --build
curl -s http://127.0.0.1:8080/healthz
# 浏览器打开 http://127.0.0.1:8080/
```

停止：

```bash
docker compose down
```

## 带 MySQL + Redis（后续 users / alerts / portfolio）

```bash
# .env 中改为：
# MYSQL_ENABLED=true
# REDIS_ENABLED=true

docker compose --profile db up -d --build
```

- MySQL 默认库 / 用户：`marketpulse` / `marketpulse`（可用 `.env` 覆盖）
- Redis：`redis:6379`，密码默认空
- 应用通过环境变量 `MARKETPULSE_MYSQL_*` / `MARKETPULSE_REDIS_*` 连接（见 `internal/config`）

回滚：把 `MYSQL_ENABLED` / `REDIS_ENABLED` 改回 `false` 后重启 app，或去掉 `--profile db` 只跑行情。

## 覆盖配置

镜像内带 `config/config.docker.yaml`。若需挂载本机配置：

```bash
cp config/config.docker.yaml config/config.yaml
# 编辑 config/config.yaml
```

在 `docker-compose.yml` 的 `marketd.volumes` 取消注释：

```yaml
- ./config/config.yaml:/app/config/config.yaml:ro
```

密码等敏感项优先用环境变量，不要提交进 Git。

## 常用命令

```bash
make docker-build   # 仅构建镜像
make docker-up      # 构建并启动（仅行情）
make docker-up-db   # 构建并启动行情 + MySQL + Redis
make docker-down    # 停止并移除容器
make docker-logs    # 跟随 marketd 日志
```

## 文件

| 文件 | 说明 |
|------|------|
| `Dockerfile` | 多阶段构建：Node 前端 + Go 后端 + Alpine 运行 |
| `docker-compose.yml` | 编排；`profile: db` 启持久化 |
| `config/config.docker.yaml` | 容器默认配置 |
| `.env.example` | Compose 环境变量模板 |
| `.dockerignore` | 缩小构建上下文 |

## 注意

- 容器需能访问 Binance / 百度等外网数据源；国内机器可能要调整 `ingest.binance.ws_base`。
- 日志写入命名卷 `marketpulse-log`（对应容器内 `/app/log`）。
- 现有 `make ship` 部署方式保持不变，Docker 为并行方案。
