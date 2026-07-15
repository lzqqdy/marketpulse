import type {
  AlertDeliveriesResult,
  AlertRule,
  CreateAlertRuleInput,
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

export async function listRules(token: string, status?: string): Promise<AlertRule[]> {
  const q = status ? `?status=${encodeURIComponent(status)}` : ''
  const res = await fetch(`/api/v1/alerts/rules${q}`, { headers: authHeaders(token) })
  if (!res.ok) throw new Error(await parseError(res))
  const body = await res.json()
  return (body.items ?? []) as AlertRule[]
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
  opts: { page?: number; pageSize?: number; ruleId?: number; channel?: string } = {},
): Promise<AlertDeliveriesResult> {
  const params = new URLSearchParams()
  params.set('page', String(opts.page ?? 1))
  params.set('pageSize', String(opts.pageSize ?? 20))
  if (opts.ruleId) params.set('ruleId', String(opts.ruleId))
  if (opts.channel) params.set('channel', opts.channel)
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
