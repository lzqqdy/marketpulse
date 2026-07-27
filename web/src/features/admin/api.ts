import type { AdminConfigView, AdminMeResponse, SaveConfigResult } from './types'

async function parseError(res: Response): Promise<string> {
  const body = await res.json().catch(() => ({}))
  return body?.error?.message ?? `HTTP ${res.status}`
}

export async function fetchAdminMe(token: string): Promise<AdminMeResponse> {
  const res = await fetch('/api/v1/admin/me', {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function fetchAdminConfig(token: string): Promise<AdminConfigView> {
  const res = await fetch('/api/v1/admin/config', {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function saveAdminConfig(token: string, yaml: string): Promise<SaveConfigResult> {
  const res = await fetch('/api/v1/admin/config', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ yaml }),
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}
