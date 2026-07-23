const CODE_MESSAGES: Record<string, string> = {
  ai_quota_exceeded: '今日 AI 额度已用完，请明天再试',
  ai_disabled: 'AI 助手未启用',
  ai_misconfigured: 'AI 配置不完整，请检查服务端密钥',
  ai_conversation_busy: '当前会话正在回复中，请稍候',
  ai_upstream: '模型服务暂时不可用，请稍后重试',
  invalid_request: '请求无效，请检查输入后重试',
  not_found: '会话不存在或已删除',
  internal_error: '服务异常，请稍后重试',
}

/** 把后端 code / 原始文案收敛成可读中文。 */
export function formatAiError(code?: string, message?: string): string {
  if (code && CODE_MESSAGES[code]) return CODE_MESSAGES[code]
  const msg = (message || '').trim()
  if (!msg) return '请求失败'
  if (/quota/i.test(msg)) return CODE_MESSAGES.ai_quota_exceeded
  if (/disabled/i.test(msg)) return CODE_MESSAGES.ai_disabled
  if (/busy/i.test(msg)) return CODE_MESSAGES.ai_conversation_busy
  if (/upstream|模型/i.test(msg)) return CODE_MESSAGES.ai_upstream
  return msg
}
