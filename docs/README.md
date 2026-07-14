# MarketPulse 文档

| 文档 | 说明 |
|------|------|
| [RFC-001-architecture.md](./RFC-001-architecture.md) | **整体架构**（前后端、数据源、仓库、里程碑） |
| [RFC-002-api-contract.md](./RFC-002-api-contract.md) | **REST / WebSocket 契约**（与实现对齐） |
| [RFC-003-deployment.md](./RFC-003-deployment.md) | 部署（**Docker** / ship / nginx、Git 仓库、进程管理） |
| [RFC-004-implementation-roadmap.md](./RFC-004-implementation-roadmap.md) | **分步实现路线图**（Vibe Coding，含完成状态） |
| [DATA_SOURCES.md](./DATA_SOURCES.md) | 每个市场数据源的接入方式、主备关系、频率和输出 |
| [MODULES.md](./MODULES.md) | 模块边界和后续功能归属 |
| [VIBE_GUIDE.md](./VIBE_GUIDE.md) | AI 辅助开发规则 |
| [SPEC_KIT_GUIDE.md](./SPEC_KIT_GUIDE.md) | **Spec Kit 规格驱动开发**（新功能工作流） |
| [providers/baidu_finance.md](./providers/baidu_finance.md) | 百度财经 API 调研报告 |
| [BAIDU_FINANCE_API_RESEARCH.md](./BAIDU_FINANCE_API_RESEARCH.md) | 百度财经 API 调研（早期版本，见 providers/ 目录） |

## 功能规格（specs/）

| 目录 | 状态 | 说明 |
|------|------|------|
| [specs/001-baidu-index-provider/](../specs/001-baidu-index-provider/) | ✅ 已实现 | 百度财经指数主源切换 |
| [specs/002-market-center-panel/](../specs/002-market-center-panel/) | ✅ 已实现 | 行情中心面板（A股/港股/美股） |

阅读顺序：**001 → 002 → DATA_SOURCES → 003**（含 Docker 与 ship）；开发时以 **004** 和 **VIBE_GUIDE** 为 checklist；**新功能**用 **SPEC_KIT_GUIDE**。

Docker 运维速查另见：[deploy/docker.md](../deploy/docker.md)。
