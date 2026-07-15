import type {
  AlertDeliveriesResult,
  AlertRule,
  AlertRulesResult,
  CreateAlertRuleInput,
  ListDeliveriesQuery,
  ListRulesQuery,
  UpdateAlertRuleInput,
} from './types'

async function parseError(res: Response): Promise<string> {
  const body = await res.json().catch(() => ({}))
  return body?.error?.message ?? `HTTP ${res.status}`
}

function authHeaders(token: string, json = false): HeadersInit {
  const h: Record<string, string> = { Authorization: `Bearer ${token}` }
  if (json) h['Content-Type'] = 'application/json'
  return h
}

function toParams(opts: Record<string, string | number | undefined | null>): URLSearchParams {
  const params = new URLSearchParams()
  for (const [k, v] of Object.entries(opts)) {
    if (v === undefined || v === null || v === '') continue
    params.set(k, String(v))
  }
  return params
}

export async function listRules(token: string, opts: ListRulesQuery = {}): Promise<AlertRulesResult> {
  const params = toParams({
    page: opts.page ?? 1,
    pageSize: opts.pageSize ?? 20,
    status: opts.status,
    assetType: opts.assetType,
    symbol: opts.symbol,
    ruleType: opts.ruleType && opts.ruleType > 0 ? opts.ruleType : undefined,
    sortBy: opts.sortBy ?? 'id',
    sortOrder: opts.sortOrder ?? 'desc',
  })
  const res = await fetch(`/api/v1/alerts/rules?${params}`, { headers: authHeaders(token) })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function createRule(token: string, input: CreateAlertRuleInput): Promise<AlertRule> {
  const res = await fetch('/api/v1/alerts/rules', {
    method: 'POST',
    headers: authHeaders(token, true),
    body: JSON.stringify({
      field: 'price',
      intervalMinutes: 10,
      ...input,
    }),
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function updateRule(
  token: string,
  id: number,
  input: UpdateAlertRuleInput,
): Promise<AlertRule> {
  const res = await fetch(`/api/v1/alerts/rules/${id}`, {
    method: 'PATCH',
    headers: authHeaders(token, true),
    body: JSON.stringify(input),
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function deleteRule(token: string, id: number): Promise<void> {
  const res = await fetch(`/api/v1/alerts/rules/${id}`, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
  if (!res.ok) throw new Error(await parseError(res))
}

export async function listDeliveries(
  token: string,
  opts: ListDeliveriesQuery = {},
): Promise<AlertDeliveriesResult> {
  const params = toParams({
    page: opts.page ?? 1,
    pageSize: opts.pageSize ?? 20,
    ruleId: opts.ruleId && opts.ruleId > 0 ? opts.ruleId : undefined,
    channel: opts.channel,
    status: opts.status,
    assetType: opts.assetType,
    symbol: opts.symbol,
    ruleType: opts.ruleType && opts.ruleType > 0 ? opts.ruleType : undefined,
    sortBy: opts.sortBy ?? 'createdAt',
    sortOrder: opts.sortOrder ?? 'desc',
  })
  const res = await fetch(`/api/v1/alerts/deliveries?${params}`, {
    headers: authHeaders(token),
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function ackInbox(token: string, deliveryIds: number[]): Promise<void> {
  if (!deliveryIds.length) return
  const res = await fetch('/api/v1/alerts/inbox/ack', {
    method: 'POST',
    headers: authHeaders(token, true),
    body: JSON.stringify({ deliveryIds }),
  })
  if (!res.ok) throw new Error(await parseError(res))
}
