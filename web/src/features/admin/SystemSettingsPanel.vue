<script setup lang="ts">
import { computed, onMounted, reactive, ref, shallowRef } from 'vue'
import { load as yamlLoad } from 'js-yaml'
import { useAuthStore } from '@/features/auth/stores/auth'
import * as api from './api'
import ConfigField from './ConfigField.vue'
import {
  applySelectedGaps,
  dumpYamlTree,
  formatPreview,
  kindLabel,
  parseYamlTree,
  schemaMap,
  type ConfigTree,
} from './configTree'
import type { AdminConfigView, ConfigGap, FieldSchema, SectionMeta } from './types'

const auth = useAuthStore()

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const okMsg = ref('')
const mode = ref<'form' | 'yaml'>('form')
const sectionId = ref('basic')

const view = shallowRef<AdminConfigView | null>(null)
const editTree = shallowRef<ConfigTree>({})
const yamlText = ref('')
const dirty = ref(false)

const selectedGaps = reactive<Record<string, boolean>>({})
const gapOverrides = reactive<Record<string, string>>({})

const schemas = computed(() => schemaMap(view.value?.schema ?? []))
const sections = computed<SectionMeta[]>(() => view.value?.sections ?? [])
const actionableGaps = computed(() =>
  (view.value?.gaps ?? []).filter((g) => g.kind === 'missing_in_live' || g.kind === 'type_mismatch'),
)
const infoGaps = computed(() => (view.value?.gaps ?? []).filter((g) => g.kind === 'missing_in_example'))

const activeSection = computed(() => sections.value.find((s) => s.id === sectionId.value) ?? sections.value[0])

const formFields = computed(() => {
  const keys = activeSection.value?.keys ?? []
  const tree = editTree.value
  const fields: { path: string; schema?: FieldSchema }[] = []
  for (const key of keys) {
    if (tree[key] === undefined) continue
    collectLeafPaths(key, tree[key], fields)
  }
  return fields
})

function collectLeafPaths(prefix: string, node: unknown, out: { path: string; schema?: FieldSchema }[]) {
  const sch = schemas.value.get(prefix)
  if (sch && (sch.widget === 'string_list' || sch.widget === 'object_list' || sch.widget === 'textarea')) {
    out.push({ path: prefix, schema: sch })
    return
  }
  if (node && typeof node === 'object' && !Array.isArray(node)) {
    const obj = node as ConfigTree
    const keys = Object.keys(obj)
    if (!keys.length) {
      out.push({ path: prefix, schema: sch })
      return
    }
    for (const k of keys) {
      collectLeafPaths(`${prefix}.${k}`, obj[k], out)
    }
    return
  }
  out.push({ path: prefix, schema: sch })
}

function clearGapSelection() {
  for (const k of Object.keys(selectedGaps)) delete selectedGaps[k]
  for (const k of Object.keys(gapOverrides)) delete gapOverrides[k]
}

async function load() {
  if (!auth.token) return
  loading.value = true
  error.value = ''
  okMsg.value = ''
  try {
    const data = await api.fetchAdminConfig(auth.token)
    view.value = data
    editTree.value = parseYamlTree(data.yaml)
    yamlText.value = data.yaml
    dirty.value = false
    clearGapSelection()
    if (sections.value.length && !sections.value.find((s) => s.id === sectionId.value)) {
      sectionId.value = sections.value[0].id
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function onFormChange() {
  dirty.value = true
  // shallowRef：原地改嵌套字段不会触发视图，换新根对象强制刷新表单
  editTree.value = { ...editTree.value }
  try {
    yamlText.value = dumpYamlTree(editTree.value)
  } catch {
    /* ignore */
  }
}

function selectAllGaps(on: boolean) {
  for (const g of actionableGaps.value) {
    selectedGaps[g.path] = on
  }
}

function parseGapOverride(g: ConfigGap): unknown {
  const raw = gapOverrides[g.path]
  if (raw === undefined || raw === '') return g.exampleValue
  try {
    return JSON.parse(raw)
  } catch {
    return raw
  }
}

function buildSaveTree(): ConfigTree {
  const overrides: Record<string, unknown> = {}
  for (const g of actionableGaps.value) {
    if (!selectedGaps[g.path]) continue
    overrides[g.path] = parseGapOverride(g)
  }
  return applySelectedGaps(editTree.value, actionableGaps.value, { ...selectedGaps }, overrides)
}

function switchMode(next: 'form' | 'yaml') {
  if (next === mode.value) return
  if (next === 'yaml') {
    try {
      yamlText.value = dumpYamlTree(buildSaveTree())
      mode.value = 'yaml'
      error.value = ''
    } catch (e) {
      error.value = e instanceof Error ? e.message : '无法导出 YAML'
    }
    return
  }
  // yaml -> form
  try {
    editTree.value = parseYamlTree(yamlText.value)
    mode.value = 'form'
    dirty.value = true
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'YAML 解析失败，请修正后再切回表单'
  }
}

async function save() {
  if (!auth.token) return
  if (mode.value !== 'form') {
    error.value = '请先切换到表单再保存（便于校验）'
    return
  }
  saving.value = true
  error.value = ''
  okMsg.value = ''
  try {
    const tree = buildSaveTree()
    const text = dumpYamlTree(tree)
    // validate locally
    yamlLoad(text)
    const res = await api.saveAdminConfig(auth.token, text)
    okMsg.value = res.message || '已保存'
    dirty.value = false
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败'
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="sys-panel">
    <header class="sys-head">
      <div>
        <h2>系统设置</h2>
        <p v-if="view" class="sys-path">编辑 {{ view.path }} · 对照 {{ view.examplePath }}</p>
      </div>
      <div class="mode-toggle" role="group" aria-label="编辑模式">
        <button type="button" :class="{ active: mode === 'form' }" @click="switchMode('form')">表单</button>
        <button type="button" :class="{ active: mode === 'yaml' }" @click="switchMode('yaml')">Yaml</button>
      </div>
    </header>

    <p v-if="loading" class="sys-status">加载中…</p>
    <p v-if="error" class="sys-error">{{ error }}</p>
    <p v-if="okMsg" class="sys-ok">{{ okMsg }}</p>

    <template v-if="view && !loading">
      <section v-if="actionableGaps.length" class="gaps-card">
        <div class="gaps-head">
          <h3>待对齐（相对 config.example.yaml）</h3>
          <div class="gaps-actions">
            <button type="button" class="ghost-btn" @click="selectAllGaps(true)">全选缺口</button>
            <button type="button" class="ghost-btn" @click="selectAllGaps(false)">清空</button>
          </div>
        </div>
        <p class="gaps-hint">仅勾选项会在保存时合入；未勾选保持 live 原样。默认不会自动写入。</p>
        <article v-for="g in actionableGaps" :key="g.path" class="gap-row">
          <label class="gap-check">
            <input v-model="selectedGaps[g.path]" type="checkbox" />
            <span>{{ g.path }}</span>
          </label>
          <span class="gap-kind">{{ kindLabel(g.kind) }}</span>
          <div class="gap-values">
            <div class="gap-preview-block">
              <span class="muted">example</span>
              <pre class="gap-preview">{{ formatPreview(g.exampleValue) }}</pre>
            </div>
            <label class="gap-override">
              <span class="muted">合入值</span>
              <textarea
                v-model="gapOverrides[g.path]"
                class="gap-override-input"
                rows="3"
                :placeholder="formatPreview(g.exampleValue)"
                :disabled="!selectedGaps[g.path]"
              />
            </label>
          </div>
        </article>
      </section>

      <section v-if="infoGaps.length" class="gaps-card soft">
        <h3>仅 live 存在（不会一键删除）</h3>
        <p v-for="g in infoGaps" :key="g.path" class="info-gap">{{ g.path }}</p>
      </section>

      <div v-if="mode === 'form'" class="form-layout">
        <nav class="section-tabs" aria-label="配置分区">
          <button
            v-for="s in sections"
            :key="s.id"
            type="button"
            class="section-tab"
            :class="{ active: activeSection?.id === s.id }"
            @click="sectionId = s.id"
          >
            {{ s.label }}
          </button>
        </nav>
        <div class="form-body">
          <ConfigField
            v-for="f in formFields"
            :key="f.path"
            :path="f.path"
            :schema="f.schema"
            :tree="editTree"
            @change="onFormChange"
          />
          <p v-if="!formFields.length" class="sys-status">该分区暂无字段</p>
        </div>
      </div>

      <div v-else class="yaml-wrap">
        <textarea v-model="yamlText" class="yaml-editor" spellcheck="false" @input="dirty = true" />
        <p class="gaps-hint">Yaml 模式下请先切回表单再保存，以便校验必填与类型。</p>
      </div>

      <footer class="sys-foot">
        <div class="foot-top">
          <button type="button" class="ghost-btn" :disabled="loading || saving" @click="load">载入未更改</button>
          <span v-if="dirty" class="dirty">未保存更改</span>
        </div>
        <button type="button" class="primary-btn foot-save" :disabled="saving || loading" @click="save">
          {{ saving ? '保存中…' : '保存' }}
        </button>
      </footer>
      <p class="restart-hint">保存成功后需重启 marketd 才会生效。</p>
    </template>
  </section>
</template>

<style scoped>
.sys-panel {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.sys-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.sys-head h2 {
  margin: 0;
  font-size: 16px;
  color: var(--coin);
}

.sys-path {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted);
  word-break: break-all;
}

.mode-toggle {
  display: inline-flex;
  border: 1px solid var(--line);
  border-radius: 8px;
  overflow: hidden;
}

.mode-toggle button {
  border: 0;
  background: transparent;
  color: var(--text);
  padding: 7px 14px;
  font-size: 13px;
  cursor: pointer;
}

.mode-toggle button.active {
  background: var(--coin);
  color: #111;
  font-weight: 700;
}

.sys-status,
.gaps-hint,
.restart-hint {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
}

.sys-error {
  margin: 0;
  color: var(--danger, #ef4444);
  font-size: 13px;
}

.sys-ok {
  margin: 0;
  color: var(--ok, #22c55e);
  font-size: 13px;
}

.gaps-card {
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 12px;
  background: color-mix(in srgb, var(--panel) 80%, transparent);
  display: grid;
  gap: 8px;
}

.gaps-card.soft {
  opacity: 0.9;
}

.gaps-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.gaps-head h3,
.gaps-card h3 {
  margin: 0;
  font-size: 14px;
}

.gaps-actions {
  display: flex;
  gap: 6px;
}

.gap-row {
  display: grid;
  gap: 6px;
  padding: 8px 0;
  border-top: 1px solid var(--line);
}

.gap-check {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
}

.gap-kind {
  font-size: 11px;
  color: var(--warning, #d97706);
}

.gap-values {
  display: grid;
  gap: 8px;
  font-size: 12px;
  min-width: 0;
}

.gap-preview-block {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.gap-preview {
  margin: 0;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: color-mix(in srgb, var(--bg, #111) 70%, transparent);
  color: var(--text);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  line-height: 1.45;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
  max-height: 140px;
  overflow: auto;
  -webkit-overflow-scrolling: touch;
}

.gap-override {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.gap-override input,
.gap-override-input {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--text);
  border-radius: 6px;
  padding: 6px 8px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.4;
  resize: vertical;
}

.muted {
  color: var(--muted);
}

.info-gap {
  margin: 0;
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.form-layout {
  border: 1px solid var(--line);
  border-radius: 8px;
  overflow: hidden;
}

.section-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0;
  border-bottom: 1px solid var(--line);
  background: var(--card-soft, var(--panel));
}

.section-tab {
  border: 0;
  background: transparent;
  color: var(--text);
  padding: 10px 14px;
  font-size: 13px;
  cursor: pointer;
  border-bottom: 2px solid transparent;
}

.section-tab.active {
  color: var(--coin);
  border-bottom-color: var(--coin);
  font-weight: 700;
}

.form-body {
  padding: 4px 12px 12px;
}

.yaml-wrap {
  display: grid;
  gap: 8px;
}

.yaml-editor {
  width: 100%;
  min-height: 420px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #0f1419;
  color: #e6edf3;
  padding: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.45;
  resize: vertical;
}

.sys-foot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.foot-top {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.foot-save {
  margin-left: auto;
}

.dirty {
  font-size: 12px;
  color: var(--warning, #d97706);
}

.ghost-btn,
.primary-btn {
  border-radius: 6px;
  font-size: 13px;
  cursor: pointer;
  padding: 8px 14px;
}

.ghost-btn {
  border: 1px solid var(--line);
  background: transparent;
  color: var(--text);
}

.primary-btn {
  border: none;
  background: var(--coin);
  color: #111;
  font-weight: 700;
}

.primary-btn:disabled,
.ghost-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

@media (max-width: 680px) {
  .sys-panel {
    gap: 12px;
    width: 100%;
    overflow-x: clip;
  }

  .sys-head {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }

  .sys-head h2 {
    font-size: 15px;
  }

  .sys-path {
    font-size: 11px;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    overflow: hidden;
  }

  .mode-toggle {
    width: 100%;
  }

  .mode-toggle button {
    flex: 1;
    min-height: 42px;
    padding: 10px 12px;
  }

  .gaps-card {
    padding: 12px;
    border-radius: 8px;
    min-width: 0;
  }

  .gaps-head {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }

  .gaps-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .gaps-actions .ghost-btn {
    min-height: 42px;
  }

  .gap-row {
    min-width: 0;
  }

  .gap-check {
    align-items: flex-start;
    word-break: break-word;
  }

  .gap-check span {
    line-height: 1.35;
  }

  .gap-preview {
    font-size: 11px;
    max-height: 120px;
  }

  .gap-override-input {
    font-size: 16px; /* avoid iOS zoom */
    min-height: 72px;
    padding: 10px 12px;
  }

  .info-gap {
    overflow-wrap: anywhere;
  }

  .form-layout {
    border-radius: 8px;
    min-width: 0;
  }

  .section-tabs {
    flex-wrap: nowrap;
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    scrollbar-width: none;
  }

  .section-tabs::-webkit-scrollbar {
    display: none;
  }

  .section-tab {
    flex: 0 0 auto;
    min-height: 44px;
    padding: 10px 14px;
    white-space: nowrap;
  }

  .form-body {
    padding: 4px 12px 12px;
    padding-bottom: 24px;
  }

  .yaml-editor {
    min-height: 240px;
    max-height: 50vh;
    font-size: 12px;
    padding: 10px;
    border-radius: 8px;
  }

  .sys-foot {
    position: sticky;
    bottom: 0;
    z-index: 5;
    margin: 0 -12px;
    padding: 12px 12px 12px;
    padding-right: 56px; /* clear floating dock */
    padding-bottom: calc(12px + env(safe-area-inset-bottom, 0px));
    background: color-mix(in srgb, var(--card, var(--panel)) 94%, transparent);
    backdrop-filter: blur(10px);
    border-top: 1px solid var(--line);
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
    /* 避免透明区域挡住上方「添加」按钮 */
    pointer-events: none;
  }

  .sys-foot > * {
    pointer-events: auto;
  }

  .foot-top {
    width: 100%;
    justify-content: space-between;
    min-height: 28px;
  }

  .foot-top .ghost-btn {
    flex: 0 0 auto;
    min-height: 40px;
    padding: 8px 12px;
  }

  .foot-save {
    width: 100%;
    margin-left: 0;
    min-height: 46px;
    box-sizing: border-box;
  }

  .restart-hint {
    padding: 0 4px 4px;
    padding-right: 48px;
  }
}
</style>
