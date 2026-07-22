import type {
  AllocationResult,
  EligibleSymbolsResult,
  HoldingInput,
  HoldingsResult,
  ListSnapshotsQuery,
  PortfolioOverview,
  PortfolioSettings,
  ReportRange,
  ReportSeriesResult,
  SnapshotsResult,
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

export async function getHoldings(token: string): Promise<HoldingsResult> {
  const res = await fetch('/api/v1/portfolio/holdings', { headers: authHeaders(token) })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function putHoldings(token: string, holdings: HoldingInput[]): Promise<HoldingsResult> {
  const res = await fetch('/api/v1/portfolio/holdings', {
    method: 'PUT',
    headers: authHeaders(token, true),
    body: JSON.stringify({ holdings }),
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function putSettings(token: string, principalCny: number): Promise<PortfolioSettings> {
  const res = await fetch('/api/v1/portfolio/settings', {
    method: 'PUT',
    headers: authHeaders(token, true),
    body: JSON.stringify({ principalCny }),
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function getOverview(token: string): Promise<PortfolioOverview> {
  const res = await fetch('/api/v1/portfolio/overview', { headers: authHeaders(token) })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function listSnapshots(token: string, opts: ListSnapshotsQuery = {}): Promise<SnapshotsResult> {
  const params = toParams({
    page: opts.page ?? 1,
    pageSize: opts.pageSize ?? 10,
    from: opts.from,
    to: opts.to,
    sort: opts.sort ?? 'date',
    order: opts.order ?? 'desc',
  })
  const res = await fetch(`/api/v1/portfolio/snapshots?${params}`, { headers: authHeaders(token) })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function getEligibleSymbols(token: string): Promise<EligibleSymbolsResult> {
  const res = await fetch('/api/v1/portfolio/eligible-symbols', { headers: authHeaders(token) })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function getReportSeries(token: string, range: ReportRange = '30d'): Promise<ReportSeriesResult> {
  const params = toParams({ range })
  const res = await fetch(`/api/v1/portfolio/reports/series?${params}`, { headers: authHeaders(token) })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function getReportAllocation(token: string): Promise<AllocationResult> {
  const res = await fetch('/api/v1/portfolio/reports/allocation', { headers: authHeaders(token) })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}
