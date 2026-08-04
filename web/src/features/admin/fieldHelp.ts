/** Admin config field help texts (path → explanation). */
export const FIELD_HELP: Record<string, string> = {
  'app.mode':
    '运行模式。\ndebug：更详细日志，便于排查；release：生产建议，日志更精简。',
  'app.addr':
    'HTTP 监听地址，例如 :8080 或 0.0.0.0:8080。修改后需重启 marketd。',
  'app.log_dir': '日志文件目录。为空时按程序默认策略输出。',
  'app.static_dir': '前端静态资源目录（如 web/dist）。一般部署时由构建产物路径决定。',

  'cors.allowed_origins':
    '允许跨域访问的 Origin 列表。本地开发常写 http://localhost:5173；生产请写实际域名。',

  'mysql.enabled':
    '是否启用 MySQL。用户中心 / 告警 / 资产 / AI / 市场异动 等模块依赖它；关闭时这些模块会 soft-skip。',
  'mysql.params': 'DSN 额外参数，例如 charset=utf8mb4&parseTime=true。',
  'mysql.max_open_conns': '最大打开连接数。过高可能压垮数据库，过低会排队变慢。',
  'mysql.max_idle_conns': '空闲连接池大小。建议小于等于 max_open_conns。',
  'mysql.conn_max_lifetime': '连接最长存活时间，到期回收重建，避免被中间件掐断的陈旧连接。',
  'mysql.conn_max_idle_time': '空闲连接最长保留时间，超时关闭以释放资源。',

  'redis.enabled':
    '是否启用 Redis。用户会话、告警等会用到；关闭时依赖 Redis 的模块可能 soft-skip。',
  'redis.db': 'Redis 逻辑库编号（0–15 常见）。多服务共用实例时用不同 db 隔离。',
  'redis.pool_size': '连接池大小。并发高时可适当增大。',
  'redis.min_idle_conns': '保持的最小空闲连接数，降低突发请求建连延迟。',

  'users.enabled':
    '用户中心开关。需要同时开启 mysql + redis；否则会自动跳过并打 warn，行情仍可启动。',
  'users.auto_migrate': '启动时自动迁移用户相关表结构。生产首次启用可开，稳定后可关。',
  'users.session_ttl': '登录会话有效期。例如 168h 表示约 7 天。',
  'users.seed.phone': '首次启动可写入的种子管理员手机号（仅初始化用）。',
  'users.seed.password': '种子账号密码。初始化后请尽快修改，勿把真实密码提交到仓库。',
  'users.seed.display_name': '种子账号显示名。',

  'alerts.enabled':
    '价格告警开关。需要 mysql + redis + users；邮件通道另需配置 smtp。',
  'alerts.auto_migrate': '启动时自动迁移告警相关表。',
  'alerts.daily_timezone':
    '告警日切时区，例如 Asia/Shanghai。影响「每日」类规则与统计边界。',
  'alerts.loop_interval_min': '告警巡检最短间隔（分钟）。实际间隔会在 min–max 间波动。',
  'alerts.loop_interval_max': '告警巡检最长间隔（分钟）。',
  'alerts.inbox_max_len': '每用户站内告警收件箱最大条数，超出淘汰旧消息。',

  'portfolio.enabled':
    '资产中心开关。需要 mysql + users（不强制 redis）。',
  'portfolio.auto_migrate': '启动时自动迁移资产相关表。',
  'portfolio.daily_timezone':
    '资产日快照时区，例如 Asia/Shanghai。约在该时区次日 0:05 后写入「昨天」快照。',
  'portfolio.default_usdt_cny':
    '当 OTC 汇率暂时不可用时的 USDT/CNY 回退默认值，避免资产估值完全空白。',

  'ai.enabled':
    '观澜 AI 助手开关。需要 mysql + users + api_key。',
  'ai.auto_migrate': '启动时自动迁移 AI 会话相关表。',
  'ai.provider': '模型服务商标识。当前内置 deepseek，也可填兼容的自定义值。',
  'ai.base_url': 'API Base URL。留空则用该 provider 默认地址。',
  'ai.api_key':
    '模型 API Key。也可用环境变量 MARKETPULSE_AI_API_KEY / DEEPSEEK_API_KEY，勿提交真实密钥。',
  'ai.model': '默认对话模型。可从下拉选择，也允许自定义模型名。',
  'ai.timeout': '单次模型请求超时时间。',
  'ai.max_tool_rounds': '一轮对话中允许的最大工具调用轮数，防止工具循环过深。',
  'ai.max_history_messages': '带入模型上下文的历史消息条数上限。',
  'ai.daily_quota_per_user': '每用户每日可用对话次数配额。',
  'ai.system_prompt': '系统提示词，决定助手人设与回答风格。改动后对新会话生效更明显。',

  'event.enabled':
    '市场异动（Event Engine）开关。需要同时开启 mysql；关闭后右侧快捷栏异动入口不可用，行情主路径不受影响。\n修改后需重启 marketd。',
  'event.auto_migrate':
    '启动时自动创建/升级 market_event、market_event_signal 表。首次启用建议打开，稳定后可关。',
  'event.evaluate_interval':
    '检测轮询间隔。例如 20s 表示大约每 20 秒对监控标的跑一轮价格/量能/波动/爆仓检测。\n过短会增加 K 线请求；过长会漏掉短时尖峰。',
  'event.aggregate_window':
    '历史字段：现已改为「同一标的未结束事件强制合并」。保留配置兼容，重启后仍可读。',
  'event.max_open_age':
    '单条事件未结束状态的最长存活时间。超时强制标记为已结束，避免 DEESCALATING 永久挂着。默认 90m。',
  'event.max_deescalate_age':
    '处于「缓和中」且长时间无刷新时的最长空闲。超时强制结束。默认 20m。',
  'event.kline_timeout':
    '单次拉取 K 线的超时时间。超时则跳过该标的本轮检测，不阻塞整轮。',
  'event.symbols':
    '异动监控的加密标的列表，例如 BTC、ETH。只检测列表内品种；与首页行情 symbols 可不同。',
  'event.price.5m.threshold_pct':
    '5 分钟价格异动阈值（绝对值，单位 %）。基于已收盘 K 线。\n例如 2 表示 5 分钟涨或跌超过约 2% 触发。数值越大越不容易报。',
  'event.price.15m.threshold_pct':
    '15 分钟价格异动阈值（绝对值，单位 %）。基于已收盘 K 线。',
  'event.price.1h.threshold_pct':
    '1 小时价格异动阈值（绝对值，单位 %）。基于已收盘 K 线。',
  'event.volume.lookback_bars':
    '成交量基线回看的 K 线根数。用最近 N 根已收盘均量作为「正常量」。',
  'event.volume.ratio_medium':
    '放量触发倍数：当前已收盘量 ÷ 近 N 根均量 ≥ 该值才记「成交量放大」。\n例如 2 表示至少 2 倍量。合成「成交量放大异动」事件还需要约 ≥3 倍。',
  'event.volume.ratio_high':
    '放量「较高」档倍数，主要用于事件强度打分，不是额外开关。通常 ≥ medium。',
  'event.volume.ratio_extreme':
    '放量「极高」档倍数，主要用于事件强度打分。通常 ≥ high。',
  'event.volatility.lookback_bars':
    '波动率基线回看根数。用近 N 根已收盘 ATR（真实波幅）均值作对照。',
  'event.volatility.ratio':
    '波动放大倍数：当前 ATR ÷ 近 N 根均值 ≥ 该值触发「波动放大」。\n例如 2 表示波动至少达到基线两倍。',
  'event.liquidation.sample_interval':
    '全市场爆仓额采样间隔，用于维护历史基线。例如 1m 表示大约每分钟记一个点。',
  'event.liquidation.baseline_samples':
    '爆仓基线最多保留的样本点数。例如 60 且采样 1m，约等于近 1 小时均值。',
  'event.liquidation.ratio':
    '爆仓放大倍数：当前全市场 1h 滚动爆仓额 ÷ 基线均值 ≥ 该值触发。\n注意：现为交易所级聚合，不是单币精确爆仓。',

  'smtp.host': '发信 SMTP 主机。告警邮件通道依赖此项。',
  'smtp.port': 'SMTP 端口，常见 465（SSL）或 587（STARTTLS）。',
  'smtp.from': '发件人地址（From），需与邮箱服务商要求一致。',

  'upload.dir': '本地上传文件存储目录（头像等）。',
  'upload.public_path': '对外访问路径前缀，例如 /uploads。',
  'upload.max_avatar_bytes': '头像最大字节数。例如 10485760 约等于 10MB。',

  symbols:
    '行情订阅的现货/合约交易对列表，例如 BTC、ETH。影响首页报价与相关采集。',

  'alpha.enabled': '美股/指数代币化参考行情（Bitget / Binance Alpha）开关。',
  'alpha.provider': '上游行情源：bitget（优先）或 binance。',
  'alpha.product_type': 'Bitget 等产品类型字段，按上游文档填写。',
  'alpha.quote_asset': '计价资产，通常为 USDT。',
  'alpha.poll_interval': '报价轮询间隔。',
  'alpha.resolve_interval': '交易对解析/校验刷新间隔。',
  'alpha.indices':
    '指数类参考标的列表。\nid：内部标识；name：展示名；symbol：上游交易对（如 SPYUSDT）。',
  'alpha.stocks':
    '股票类参考标的列表。\nid：内部标识；name：展示名；symbol：上游交易对（如 AAPLUSDT）。',

  'ingest.binance.ws_base': '币安行情 WebSocket 基址。',
  'ingest.baidu.enabled': '是否启用百度行情相关数据源。',
  'ingest.baidu.ws_enabled': '是否启用百度行情 WebSocket。',
  'ingest.baidu.ws_reconnect_max': 'WebSocket 最大重连次数。',
  'ingest.baidu.ws_reconnect_delay': '重连基础间隔。',
  'ingest.baidu.ws_patch_interval': 'WS 补丁/对账间隔。',
  'ingest.otc.usdt_cny_interval': 'USDT/CNY（U 价）轮询间隔。',
  'ingest.forex.usd_cny_interval': 'USD/CNY（美元）轮询间隔。',
  'ingest.equity.interval': '股指/股票类行情刷新间隔。',
  'ingest.equity.providers': '股指数据源优先级列表，例如 baidu / tencent / eastmoney。',
  'ingest.equity.index_ids': '需要采集的指数 ID 列表。',
  'ingest.macro.interval': '币圈宏观指标（总市值、情绪等）刷新间隔。',
}

const DURATION_HELP =
  '时长格式（Go duration），例如 30s、5m、1h、168h。不要写「30秒」这类中文单位。'

/** Resolve help text for a config path. */
export function resolveFieldHelp(path: string, widget?: string, schemaHelp?: string): string {
  if (schemaHelp?.trim()) return schemaHelp.trim()
  if (FIELD_HELP[path]) return FIELD_HELP[path]
  if (widget === 'duration') return DURATION_HELP
  const key = path.split('.').pop()?.toLowerCase() ?? ''
  if (
    key.endsWith('interval') ||
    key.endsWith('timeout') ||
    key.endsWith('ttl') ||
    key.endsWith('lifetime') ||
    key.endsWith('delay')
  ) {
    return DURATION_HELP
  }
  return ''
}
