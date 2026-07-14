# RFC-003：部署指南

| 字段 | 内容 |
|------|------|
| 状态 | Active |
| 依赖 | RFC-001 |
| 日期 | 2026-05-16 |

---

## 1. 部署模式一览

| 模式 | 命令 | 适用场景 |
|------|------|----------|
| **Docker** | `docker compose up -d --build` / `make docker-up` | 本机或任意装了 Docker 的机器；一键起行情；`--profile db` 可带 MySQL/Redis |
| **ship**（推荐 VPS） | `make ship` | 香港/海外 VPS，**IP + 端口** 访问；`marketd` 单进程托管 API + 前端 |
| **nginx** | `make deploy` | 域名 + Nginx 反代；前后端目录分离，可只发一端 |
| **embed 构建** | 本地 `DEPLOY_MODE=embed` | 历史方案；当前使用 `app.static_dir` 文件系统挂载，非 Go embed |

日常个人 VPS：**用 ship**，配置写在 `deploy/deploy.local.yaml`（已 gitignore）。
新环境 / 想一把梭：**用 Docker**，见 [deploy/docker.md](../deploy/docker.md)。

---

## 1.1 Docker 模式（摘要）

```bash
docker compose up -d --build                        # 仅行情，浏览器打开 :8080
MYSQL_ENABLED=true REDIS_ENABLED=true \
  docker compose --profile db up -d --build         # 行情 + MySQL + Redis
```

- 镜像：多阶段构建，内含前端静态资源与 `marketd`
- 默认配置：`config/config.docker.yaml`
- 详细命令与灰度回滚：[deploy/docker.md](../deploy/docker.md)

---

## 2. ship 模式（IP + 端口 + Git 仓库）

### 2.1 思路

1. 本机 `make ship`：构建前端、交叉编译 Linux `marketd`。
2. **rsync 源码** 到服务器 `remote_dir`（该目录为 **Git 仓库**，不覆盖服务器 `.git`）。
3. 同步运行产物：`bin/marketd`、`web/dist/`、`config/config.yaml`。
4. 执行 `scripts/restart.sh` 重启进程。
5. **可选**：在服务器 `git commit` 记录本次源码变更。

对外访问：`http://<公网IP>:8080/`（端口以配置为准）。

### 2.2 服务器目录（Git 仓库）

推荐将 `remote_dir` 指向已有仓库，例如：

```
/home/lzqqdy/github/marketpulse/    # git 根目录
├── .git/                           # 仅服务器维护，部署不覆盖
├── cmd/ internal/ web/src/ docs/   # rsync 同步的源码
├── Makefile deploy/ scripts/
├── bin/marketd                     # 运行用二进制（.gitignore，不入库）
├── web/dist/                       # 构建产物（.gitignore，不入库）
├── config/config.yaml              # 生产配置（.gitignore，不入库）
├── log/YYYY-MM-DD/{info,warn,error}.log
└── scripts/restart.sh
```

### 2.3 首次配置（本机）

```bash
cp deploy/deploy.local.yaml.example deploy/deploy.local.yaml
# 编辑 ssh_host、ssh_port、remote_dir 等
```

`deploy/deploy.local.yaml` 示例：

```yaml
ssh_host: "43.154.133.211"
ssh_port: 2233
ssh_user: "root"
ssh_key: ""   # 可填 ~/.ssh/id_rsa

remote_dir: "/home/lzqqdy/github/marketpulse"

app_addr: "0.0.0.0:8080"
app_port: 8080

goos: "linux"
goarch: "amd64"

sync_source: true        # 每次部署是否同步源码
git_auto_commit: false   # 是否每次部署后自动 git commit
git_commit_message: ""  # 留空则用 deploy: 时间戳
```

服务器上若尚未初始化仓库：

```bash
mkdir -p /home/lzqqdy/github/marketpulse
cd /home/lzqqdy/github/marketpulse
git init
# 或 git clone <你的远程> .
```

### 2.4 部署命令

```bash
make setup-deploy   # 首次：生成 deploy.local.yaml 模板

make ship           # 标准部署（构建 + 同步 + 重启）

make ship-commit    # 部署后在服务器自动 git commit

# 单次参数（不改 yaml）
make ship SHIP_GIT_COMMIT=1
make ship SHIP_GIT_MSG="fix: 指数限流"
make ship SHIP_NO_SOURCE=1    # 只更新 bin/web/config，不同步源码
```

| 变量 / 配置 | 说明 |
|-------------|------|
| `sync_source: true` | 同步完整源码（见 `deploy/rsync-excludes.txt`） |
| `SHIP_NO_SOURCE=1` | 临时关闭源码同步 |
| `git_auto_commit: true` | 每次 `make ship` 后自动提交 |
| `SHIP_GIT_COMMIT=1` | 仅本次自动提交 |
| `SHIP_GIT_MSG` | 提交说明，覆盖 `git_commit_message` |

### 2.5 源码同步排除项

见 `deploy/rsync-excludes.txt`，主要包括：

- `.git/`（保留服务器仓库历史）
- `web/node_modules/`、`bin/`、`web/dist/`（dist 单独同步到 `web/` 供运行）
- `config/config.yaml`、`deploy/deploy.local.yaml`（密钥不入库）

**Git 提交**只包含源码；`bin/marketd` 与 `web/dist` 由 `.gitignore` 排除，但仍会部署到服务器用于运行。

### 2.6 服务器手动 Git

```bash
ssh -p 2233 root@<host>
cd /home/lzqqdy/github/marketpulse

git status
git add -A
git commit -m "手动说明"
git push    # 需要时
```

或使用仓库内脚本：

```bash
./deploy/remote-git-commit.sh "手动说明"
```

### 2.7 进程管理

**停止**（单进程，无独立前端进程）：

```bash
pkill -f /home/lzqqdy/github/marketpulse/bin/marketd
fuser -k 8080/tcp 2>/dev/null
```

**启动 / 重启**：

```bash
/home/lzqqdy/github/marketpulse/scripts/restart.sh
```

**健康检查**：

```bash
curl -s http://127.0.0.1:8080/healthz | python3 -m json.tool
```

### 2.8 连通性检查

```bash
# 本机外网数据源
make check

# 本机 Binance REST + WS
make check-binance

# SSH 到 deploy.local 里的服务器做完整检查
make check-binance-remote
```

### 2.9 防火墙

云厂商安全组 / 宝塔：**放行 `app_port`（默认 8080）**。香港机房一般可直接访问 `stream.binance.com`；大陆机房若币价不通，可在 `config.yaml` 将 `ingest.binance.ws_base` 改为 `wss://data-stream.binance.vision/stream`。

---

## 3. nginx 模式

适用于域名 + HTTPS + 静态与 API 分离。

### 3.1 目录

```
/opt/marketpulse/          # 后端
  bin/marketd
  config/config.yaml
/var/www/marketpulse/      # 前端 dist
```

### 3.2 发布

```bash
make web && make api
make deploy-web DEPLOY_HOST=user@host    # 仅静态
make deploy-api DEPLOY_HOST=user@host    # 仅二进制
make deploy DEPLOY_HOST=user@host        # 全量
```

### 3.3 模板

- `deploy/nginx.conf.example` — `/` 静态，`/api/`、`/ws/`、`/healthz` 反代
- `deploy/marketpulse.service.example` — systemd

改静态 **无需** restart `marketd`；改后端需 `systemctl restart marketpulse`。

---

## 4. 本地开发

### Linux / macOS / Git Bash

```bash
make setup-config
make dev-api      # :8080
make dev-web      # :5173，代理 /api、/ws
make test
```

### Windows（PowerShell）

```powershell
.\scripts\dev.ps1 setup-config
.\scripts\dev.ps1 api            # :8080
.\scripts\dev.ps1 web            # :5173，代理 /api、/ws
.\scripts\dev.ps1 dev              # 同时启动前后端
.\scripts\dev.ps1 test
```

---

## 5. 上线检查清单

- [ ] `curl http://<IP>:8080/healthz` → `binance_ws: connected`
- [ ] 页面币价持续更新，状态非长期「指数加载中」
- [ ] `make check-binance-remote` 通过
- [ ] 若用 Git：`git status` 干净或已按需 commit / push

---

## 6. deploy/ 目录说明

| 文件 | 说明 |
|------|------|
| `deploy.local.yaml.example` | 本机 SSH 配置模板 → 复制为 `deploy.local.yaml` |
| `config.remote.yaml.example` | 远程 `config.yaml` 模板（ship 时按 `app_addr` 生成） |
| `rsync-excludes.txt` | ship 同步源码时的排除列表 |
| `remote-restart.sh` | 服务器重启 `marketd` |
| `remote-git-commit.sh` | 服务器端提交脚本 |
| `nginx.conf.example` | nginx 模式 |
| `marketpulse.service.example` | systemd 单元 |
| `docker.md` | Docker / Compose 部署说明 |

仓库根目录另有：`Dockerfile`、`docker-compose.yml`、`.env.example`、`config/config.docker.yaml`。

---

## 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 0.1 | 2026-05-16 | 草案（nginx / embed） |
| 0.2 | 2026-05-16 | 增加 ship、Git 仓库目录、源码同步与可选 git commit |
| 0.3 | 2026-07-11 | 增加 Windows 开发说明；embed 改为 static_dir |
| 0.4 | 2026-07-14 | 增加 Docker Compose 部署（可选 MySQL/Redis profile） |
