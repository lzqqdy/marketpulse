export type ProviderState =
  | 'healthy'
  | 'stale'
  | 'circuit_open'
  | 'unavailable'
  | 'disabled'
  | 'degraded'

export interface ProviderOverall {
  status: ProviderState
  healthy: number
  total: number
  avg_latency_ms: number
  updated_at: number
}

export interface ProviderHealth {
  name: string
  label: string
  category: string
  status: ProviderState
  role: string
  current_used: boolean
  latency_ms: number
  last_success_at: number
  last_error_at: number
  last_error: string
  fail_count: number
  circuit_open: boolean
  stale_seconds: number
}

export interface ProviderStatusResponse {
  overall: ProviderOverall
  providers: ProviderHealth[]
}
