<script setup lang="ts">
import { computed } from 'vue'
import type { FieldSchema, ItemField } from './types'
import { getAtPath, setAtPath, type ConfigTree } from './configTree'

const props = defineProps<{
  path: string
  schema?: FieldSchema
  tree: ConfigTree
}>()

const emit = defineEmits<{ change: [] }>()

const widget = computed(() => props.schema?.widget ?? inferWidget(props.path, value.value))
const label = computed(() => props.schema?.label || props.path.split('.').slice(-1)[0])

const value = computed(() => getAtPath(props.tree, props.path))

const objectItemFields = computed<ItemField[]>(() => {
  if (props.schema?.itemFields?.length) return props.schema.itemFields
  const list = asObjectList()
  if (list.length) {
    const keys = Object.keys(list[0]).filter((k) => typeof list[0][k] !== 'object')
    if (keys.length) return keys.map((key) => ({ key, label: key }))
  }
  return [
    { key: 'id', label: 'ID' },
    { key: 'name', label: '名称' },
    { key: 'symbol', label: '交易对' },
  ]
})

function inferWidget(path: string, v: unknown): string {
  const key = path.split('.').pop()?.toLowerCase() ?? ''
  if (typeof v === 'boolean') return 'switch'
  if (typeof v === 'number') return 'number'
  if (Array.isArray(v)) {
    if (v.length && typeof v[0] === 'object') return 'object_list'
    return 'string_list'
  }
  if (key.includes('password') || key.includes('api_key')) return 'password'
  if (key.endsWith('interval') || key.endsWith('timeout') || key.endsWith('ttl')) return 'duration'
  return 'text'
}

function bump() {
  emit('change')
}

function setValue(next: unknown) {
  setAtPath(props.tree, props.path, next)
  bump()
}

function asStringList(): string[] {
  const v = value.value
  if (!Array.isArray(v)) return []
  return v.map((x) => String(x ?? ''))
}

function updateListItem(i: number, text: string) {
  const list = asStringList()
  list[i] = text
  setValue(list)
}

function addListItem() {
  setValue([...asStringList(), ''])
}

function removeListItem(i: number) {
  const list = asStringList()
  list.splice(i, 1)
  setValue(list)
}

function asObjectList(): Record<string, unknown>[] {
  const v = value.value
  if (!Array.isArray(v)) return []
  return v.map((item) => {
    if (item && typeof item === 'object' && !Array.isArray(item)) {
      return { ...(item as Record<string, unknown>) }
    }
    return {}
  })
}

function emptyObjectItem(): Record<string, unknown> {
  const row: Record<string, unknown> = {}
  for (const f of objectItemFields.value) row[f.key] = ''
  return row
}

function addObjectItem() {
  setValue([...asObjectList(), emptyObjectItem()])
}

function removeObjectItem(i: number) {
  const list = asObjectList()
  list.splice(i, 1)
  setValue(list)
}

function updateObjectField(i: number, key: string, text: string) {
  const list = asObjectList()
  list[i] = { ...list[i], [key]: text }
  setValue(list)
}
</script>

<template>
  <div class="cfg-field">
    <span class="cfg-label">{{ label }}</span>
    <span class="cfg-path">{{ path }}</span>

    <template v-if="widget === 'switch'">
      <input
        type="checkbox"
        class="cfg-switch"
        :checked="Boolean(value)"
        @change="setValue(($event.target as HTMLInputElement).checked)"
      />
    </template>

    <template v-else-if="widget === 'select'">
      <div class="cfg-select-row">
        <select
          :value="String(value ?? '')"
          @change="setValue(($event.target as HTMLSelectElement).value)"
        >
          <option v-for="opt in schema?.options || []" :key="opt" :value="opt">{{ opt }}</option>
          <option
            v-if="schema?.allowCustom && value != null && !(schema.options || []).includes(String(value))"
            :value="String(value)"
          >
            {{ value }} (自定义)
          </option>
        </select>
        <input
          v-if="schema?.allowCustom"
          class="cfg-input"
          type="text"
          :value="String(value ?? '')"
          placeholder="自定义值"
          @input="setValue(($event.target as HTMLInputElement).value)"
        />
      </div>
    </template>

    <template v-else-if="widget === 'number'">
      <input
        class="cfg-input"
        type="number"
        :value="Number(value ?? 0)"
        @input="setValue(Number(($event.target as HTMLInputElement).value))"
      />
    </template>

    <template v-else-if="widget === 'password'">
      <input
        class="cfg-input"
        type="password"
        autocomplete="new-password"
        :value="String(value ?? '')"
        @input="setValue(($event.target as HTMLInputElement).value)"
      />
    </template>

    <template v-else-if="widget === 'textarea'">
      <textarea
        class="cfg-textarea"
        rows="4"
        :value="String(value ?? '')"
        @input="setValue(($event.target as HTMLTextAreaElement).value)"
      />
    </template>

    <template v-else-if="widget === 'string_list'">
      <div class="cfg-list">
        <div v-for="(item, i) in asStringList()" :key="i" class="cfg-list-row">
          <input
            class="cfg-input"
            type="text"
            :value="item"
            @input="updateListItem(i, ($event.target as HTMLInputElement).value)"
          />
          <button type="button" class="ghost-btn" @click.stop.prevent="removeListItem(i)">删除</button>
        </div>
        <button type="button" class="ghost-btn cfg-add-btn" @click.stop.prevent="addListItem">+ 添加</button>
      </div>
    </template>

    <template v-else-if="widget === 'object_list'">
      <div class="cfg-obj-list">
        <article v-for="(item, i) in asObjectList()" :key="i" class="cfg-obj-card">
          <header class="cfg-obj-head">
            <span class="cfg-obj-index">#{{ i + 1 }}</span>
            <button type="button" class="ghost-btn danger-tone" @click.stop.prevent="removeObjectItem(i)">
              删除
            </button>
          </header>
          <div class="cfg-obj-grid">
            <label v-for="f in objectItemFields" :key="f.key" class="cfg-obj-field">
              <span>{{ f.label || f.key }}</span>
              <input
                class="cfg-input"
                type="text"
                :value="String(item[f.key] ?? '')"
                :placeholder="f.key"
                @input="updateObjectField(i, f.key, ($event.target as HTMLInputElement).value)"
              />
            </label>
          </div>
        </article>
        <button type="button" class="ghost-btn cfg-add-btn" @click.stop.prevent="addObjectItem">+ 添加</button>
      </div>
    </template>

    <template v-else>
      <input
        class="cfg-input"
        type="text"
        :value="String(value ?? '')"
        @input="setValue(($event.target as HTMLInputElement).value)"
      />
    </template>
  </div>
</template>

<style scoped>
.cfg-field {
  display: grid;
  gap: 4px;
  padding: 10px 0;
  border-bottom: 1px solid var(--line);
}

.cfg-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-strong);
}

.cfg-path {
  font-size: 11px;
  color: var(--muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.cfg-input,
.cfg-textarea,
select {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--text);
  border-radius: 6px;
  padding: 7px 10px;
  font-size: 13px;
}

.cfg-textarea.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
}

.cfg-switch {
  width: 18px;
  height: 18px;
}

.cfg-select-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.cfg-list {
  display: grid;
  gap: 6px;
}

.cfg-list-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 6px;
}

.cfg-obj-list {
  display: grid;
  gap: 10px;
  min-width: 0;
}

.cfg-obj-card {
  display: grid;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: color-mix(in srgb, var(--panel) 85%, transparent);
  min-width: 0;
}

.cfg-obj-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.cfg-obj-index {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
}

.cfg-obj-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.cfg-obj-field {
  display: grid;
  gap: 4px;
  font-size: 11px;
  color: var(--muted);
  min-width: 0;
}

.cfg-add-btn {
  justify-self: start;
  min-height: 36px;
  user-select: none;
  -webkit-user-select: none;
}

.cfg-hint {
  font-size: 11px;
  color: var(--muted);
}

.ghost-btn {
  border: 1px solid var(--line);
  background: transparent;
  color: var(--text);
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 12px;
  cursor: pointer;
}

.danger-tone {
  color: var(--danger, #ef4444);
  border-color: color-mix(in srgb, var(--danger, #ef4444) 40%, var(--line));
}

@media (max-width: 680px) {
  .cfg-field {
    padding: 12px 0;
    gap: 6px;
  }

  .cfg-label {
    font-size: 14px;
  }

  .cfg-path {
    font-size: 10px;
    overflow-wrap: anywhere;
  }

  .cfg-select-row {
    grid-template-columns: 1fr;
  }

  .cfg-input,
  .cfg-textarea,
  select {
    font-size: 16px;
    min-height: 40px;
    padding: 10px 12px;
  }

  .cfg-textarea {
    min-height: 96px;
  }

  .cfg-list-row {
    grid-template-columns: 1fr auto;
    align-items: center;
  }

  .cfg-obj-grid {
    grid-template-columns: 1fr;
  }

  .cfg-obj-card {
    padding: 12px;
  }

  .cfg-list-row .ghost-btn,
  .cfg-list > .ghost-btn,
  .cfg-add-btn,
  .cfg-obj-head .ghost-btn {
    min-height: 44px;
    min-width: 44px;
  }

  .cfg-switch {
    width: 22px;
    height: 22px;
  }
}

@media (min-width: 681px) and (max-width: 980px) {
  .cfg-obj-grid {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
