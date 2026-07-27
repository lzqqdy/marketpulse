export type FieldWidget =
  | 'switch'
  | 'select'
  | 'text'
  | 'password'
  | 'number'
  | 'duration'
  | 'string_list'
  | 'object_list'
  | 'textarea'

export interface ItemField {
  key: string
  label?: string
}

export interface FieldSchema {
  path: string
  widget: FieldWidget
  options?: string[]
  allowCustom?: boolean
  label?: string
  itemFields?: ItemField[]
}

export interface SectionMeta {
  id: string
  label: string
  keys: string[]
}

export interface ConfigGap {
  path: string
  kind: 'missing_in_live' | 'missing_in_example' | 'type_mismatch' | string
  exampleValue?: unknown
  liveValue?: unknown
  suggestedYamlSnippet?: string
}

export interface AdminConfigView {
  path: string
  examplePath: string
  yaml: string
  exampleYaml: string
  gaps: ConfigGap[]
  schema: FieldSchema[]
  sections: SectionMeta[]
  restartRequiredHint: boolean
}

export interface AdminMeResponse {
  isAdmin: boolean
}

export interface SaveConfigResult {
  ok: boolean
  restartRequired: boolean
  message: string
}
