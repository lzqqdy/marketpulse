# Spec Kit 开发指南

MarketPulse 已集成 [GitHub Spec Kit](https://github.com/github/spec-kit)，用于后续功能的**规格驱动开发（Spec-Driven Development, SDD）**。

## 与现有文档的关系

| 文档体系 | 用途 | 何时使用 |
|----------|------|----------|
| `docs/RFC-*.md` | 架构、API 契约、部署、路线图 | 全局设计、契约变更、部署 |
| `docs/VIBE_GUIDE.md` | AI 辅助开发规则 | 每次开发前参考 |
| `specs/<feature>/` | 单个功能的 spec / plan / tasks | 新功能开发 |
| `.specify/memory/constitution.md` | 项目治理原则 | Spec Kit 全流程自动引用 |

**Brownfield 策略**：已有代码和 RFC 保持不变；新功能走 `specs/` 目录（Flow-Forward 模式），每个功能目录保留完整历史。

## 前置条件

- [uv](https://docs.astral.sh/uv/) + `specify-cli`（已安装 v0.12.4）
- Cursor Agent（skills 已安装到 `.cursor/skills/`）

升级 CLI：

```bash
specify self check
specify self upgrade
```

刷新 Spec Kit 项目文件（不覆盖 `specs/` 和自定义 constitution）：

```bash
cd marketpulse
specify init --here --force --integration cursor-agent
```

## 标准工作流

在 Cursor Agent 中按顺序使用以下 skills：

| 步骤 | Skill | 作用 |
|------|-------|------|
| 0 | `/speckit-constitution` | 建立/更新项目原则（已完成初版） |
| 1 | `/speckit-specify` | 描述**做什么、为什么**（不涉及技术栈） |
| 2 | `/speckit-clarify` | （可选）澄清模糊需求，plan 之前运行 |
| 3 | `/speckit-plan` | 制定技术实现方案（引用 MarketPulse 技术栈） |
| 4 | `/speckit-tasks` | 生成可执行任务列表 |
| 5 | `/speckit-analyze` | （可选）检查 spec/plan/tasks 一致性 |
| 6 | `/speckit-implement` | 按 tasks 执行实现 |
| 7 | `/speckit-converge` | 评估完成度，补充遗漏任务 |

### 示例：开发 K 线增强功能

```
/speckit-specify 为 MarketPulse 添加 K 线多周期切换（1m/5m/1h/1d），
支持在行情看板侧边抽屉中查看，数据来自已有 kline API，要求切换周期时
保留当前交易对选择，加载中显示骨架屏。
```

plan 阶段补充技术上下文：

```
/speckit-plan 后端已有 internal/api/kline.go 和 eastmoney/tencent kline provider。
前端使用 Vue 3 + Pinia chart store + lightweight-charts。
遵循 docs/RFC-002-api-contract.md 中 kline 路由定义。
改动范围：web/src/features/market/ 和必要的 API 适配。
```

## 与 RFC-004 路线图配合

- **路线图 Step（增量改造）**：继续用「做 Step X」方式，参考 `RFC-004-implementation-roadmap.md`
- **新功能（独立特性）**：用 Spec Kit 完整流程，产出 `specs/00N-feature-name/` 目录

两者可并存：路线图管「已知步骤」，Spec Kit 管「新需求从 0 到 1」。

## 目录结构

初始化后新增：

```
marketpulse/
├── .specify/           # Spec Kit 模板、脚本、constitution
│   ├── memory/constitution.md
│   ├── scripts/bash/
│   └── templates/
├── .cursor/skills/     # Cursor Agent skills（speckit-*）
└── specs/              # 功能规格（首次 /speckit-specify 后创建）
    └── 001-feature-name/
        ├── spec.md
        ├── plan.md
        ├── tasks.md
        └── ...
```

## Plan 阶段应引用的 MarketPulse 上下文

在 `/speckit-plan` 时，让 Agent 阅读：

- `docs/RFC-001-architecture.md` — 整体架构
- `docs/RFC-002-api-contract.md` — API/WS 契约
- `docs/MODULES.md` — 模块边界
- `docs/DATA_SOURCES.md` — 数据源（如涉及行情）
- `docs/VIBE_GUIDE.md` — 开发规则

## 常见模式

### Flow-Forward（推荐，新功能）

每次新功能创建新 `specs/00N-*/` 目录，保留历史记录。

### Living Spec（迭代同一功能）

修改已有 `spec.md` → 重新 `/speckit-plan` → `/speckit-tasks` → `/speckit-implement`。

### Flow-Back（实现中发现需求变更）

在 plan/tasks/代码中发现问题后，回溯更新 spec，再 `/speckit-analyze` 对齐。

## 注意事项

- `.cursor/` 可能含 Agent 凭据，已建议加入 `.gitignore`
- `specs/` 和 `.specify/memory/constitution.md` 应纳入版本控制
- 契约变更仍需同步更新 `docs/RFC-002-api-contract.md`
- 运行 `/speckit-implement` 前确保 `make test` 和 `npm run build` 基线通过
