import type { ProviderStatusResponse } from '@/types/providers'

export async function fetchProviderStatus(): Promise<ProviderStatusResponse> {
  const res = await fetch('/api/v1/providers/status')
  if (!res.ok) {
    throw new Error(`providers status HTTP ${res.status}`)
  }
  return res.json() as Promise<ProviderStatusResponse>
}
