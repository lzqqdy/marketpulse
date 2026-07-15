export type MpSortOrder = 'asc' | 'desc'

export interface MpColumn {
  key: string
  label: string
  sortable?: boolean
  width?: string
  align?: 'left' | 'center' | 'right'
}
