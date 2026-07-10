# MarketPulse

个人加密货币行情看板：实时币价（WebSocket）、股指、宏观指标。

- 设计文档：[docs/README.md](./docs/README.md)
- 架构 RFC：[docs/RFC-001-architecture.md](./docs/RFC-001-architecture.md)

## 仓库结构

```
marketpulse/
├── cmd/marketd/      # Go 后端入口
├── internal/         # 后端实现
├── web/              # Vue 前端
├── docs/             # RFC 与设计
├── deploy/           # 部署模板
└── Makefile          # 构建 / 部署命令
```

## 快速开始

```bash
make help
make setup-config   # 首次：生成 config/config.yaml
make test           # Phase A 单元测试
make dev-api        # Gin :8080

# 另一个终端（前端占位）
make dev-web
```

```bash
curl -s http://127.0.0.1:8080/healthz | jq .
curl -s http://127.0.0.1:8080/api/v1/snapshot | jq .
```

## 部署（HK VPS）

推荐 **`make ship`**：单端口（默认 `:8080`）对外，源码同步到服务器 Git 仓库目录。

```bash
cp deploy/deploy.local.yaml.example deploy/deploy.local.yaml
# 配置 ssh_host、remote_dir: /home/lzqqdy/github/marketpulse

make ship                 # 构建 + 同步 + 重启
make ship-commit          # 同上，并在服务器 git commit
make ship SHIP_GIT_COMMIT=1   # 单次自动提交
```

- 部署详解：[docs/RFC-003-deployment.md](./docs/RFC-003-deployment.md)
- 模板说明：[deploy/README.md](./deploy/README.md)
- 使用 **Nginx + 域名** 时：`make deploy DEPLOY_HOST=user@host`

## 开发方式

### Spec Kit（新功能推荐）

已集成 [GitHub Spec Kit](https://github.com/github/spec-kit)。在 Cursor Agent 中使用 `/speckit-specify` 开始新功能开发。详见 [docs/SPEC_KIT_GUIDE.md](./docs/SPEC_KIT_GUIDE.md)。

### Vibe Coding（增量改造）

- 后端改动：`internal/`、`cmd/`
- 前端改动：`web/`
- 契约变更：先改 `docs/RFC-002-api-contract.md`
- 路线图：按 `docs/RFC-004-implementation-roadmap.md` 逐步推进
