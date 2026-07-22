# deploy/ — 部署模板与脚本

本目录为**可提交到 Git 的部署资源**（不含密钥）。本机私密配置见 `deploy/deploy.local.yaml`（已 gitignore）。

完整说明：[docs/RFC-003-deployment.md](../docs/RFC-003-deployment.md)（含 **§1.1 Docker**）

Docker 运维速查：[docker.md](./docker.md)

## 快速开始（Docker）

```bash
docker compose up -d --build              # 仅行情
make docker-up-db                         # 行情 + MySQL + Redis
```

完整步骤、灰度回滚、FAQ 见 [RFC-003 §1.1](../docs/RFC-003-deployment.md)。

## 快速开始（ship · IP + 端口）

```bash
cp deploy/deploy.local.yaml.example deploy/deploy.local.yaml
# 填写 ssh_host、remote_dir（服务器 git 仓库路径）

make ship              # 部署
make ship-commit       # 部署 + 服务器 git commit
```

## 文件说明

| 文件 | 用途 |
|------|------|
| `deploy.local.yaml.example` | 本机 SSH / 远程目录 / 是否自动 commit |
| `config.remote.yaml.example` | 远程生产 `config.yaml` 模板（含 mysql/redis/users/alerts/portfolio，默认关） |
| `rsync-excludes.txt` | `make ship` 同步源码时的排除规则（含保留 `data/` 上传目录） |
| `remote-restart.sh` | 上传为 `scripts/restart.sh`，重启 marketd |
| `remote-git-commit.sh` | 上传为 `deploy/remote-git-commit.sh`，服务器提交 |
| `nginx.conf.example` | nginx 分离部署 |
| `marketpulse.service.example` | systemd 单元 |

## deploy.local.yaml 常用项

```yaml
remote_dir: "/home/lzqqdy/github/marketpulse"
sync_source: true
sync_config: false       # true 或 SHIP_SYNC_CONFIG=1 才会覆盖线上 config
git_auto_commit: false
```

**配置提示**：默认不覆盖服务器 `config/config.yaml`。要启用用户/告警/资产，请在服务器上手动合并 `config.remote.yaml.example` / `config.example.yaml` 中的 mysql/redis/users/alerts/portfolio/smtp 段，或临时 `make ship SHIP_SYNC_CONFIG=1`（会覆盖远程 config，慎用）。

## 命令速查

```bash
# Linux/macOS/Git Bash
make ship
make ship SHIP_GIT_COMMIT=1 SHIP_GIT_MSG="feat: xxx"
make ship SHIP_NO_SOURCE=1
make check-binance-remote
```

Windows 本地开发无需 make，见 `scripts/dev.ps1` 或 [docs/RFC-003-deployment.md](../docs/RFC-003-deployment.md) §4。
