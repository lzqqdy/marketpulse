/** 与 docs/RFC-002 §11 / specs/004-alert-push 对齐 */

export type AlertAssetType = 'spot' | 'index'
export type AlertFrequency = 'once' | 'loop' | 'daily'
export type AlertStatus = 'active' | 'disabled'
export type AlertChannel = 'in_app' | 'email' | 'pushplus'
export type AlertDeliveryStatus = 'success' | 'failed' | 'skipped'

export interface AlertRuleParams {
  target?: number
  range?: number
  upper?: number
  lower?: number
  ampl?: number
  rapid_chg?: number
}

export interface AlertRule {
  id: number
  assetType: AlertAssetType
  symbol: string
  field: string
  ruleType: number
  params: AlertRuleParams
  channels: AlertChannel[]
  frequency: AlertFrequency
  intervalMinutes: number
  setPrice: string
  status: AlertStatus
  lastTriggeredAt: number | null
  triggerCount: number
  createdAt: number
  updatedAt: number
}

export interface CreateAlertRuleInput {
  assetType: AlertAssetType
  symbol: string
  field?: string
  ruleType: number
  params: AlertRuleParams
  channels: AlertChannel[]
  frequency: AlertFrequency
  intervalMinutes?: number
}

export interface UpdateAlertRuleInput {
  params?: AlertRuleParams
  channels?: AlertChannel[]
  frequency?: AlertFrequency
  intervalMinutes?: number
  status?: AlertStatus
}

export interface AlertDelivery {
  id: number
  ruleId: number
  assetType: AlertAssetType
  symbol: string
  ruleType: number
  channel: AlertChannel
  triggerValue: string
  title: string
  body: string
  status: AlertDeliveryStatus
  errorMsg: string
  createdAt: number
}

export interface AlertDeliveriesResult {
  items: AlertDelivery[]
  page: number
  pageSize: number
  total: number
}

export interface AlertInboxItem {
  deliveryId: number
  ruleId: number
  title: string
  body: string
  symbol: string
  createdAt: number
}

export type AlertWsMessage =
  | { type: 'inbox_snapshot'; data: { items: AlertInboxItem[] } }
  | { type: 'alert'; data: AlertInboxItem }
  | { type: 'pong'; ts?: number }
