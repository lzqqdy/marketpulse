<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import MpListTable from '@/components/MpListTable.vue'
import type { MpColumn, MpSortOrder } from '@/components/mpListTable'
import { useAuthStore } from '@/features/auth/stores/auth'
import * as alertsApi from './api'
import type { AlertChannel, AlertFrequency, AlertRule, AlertRuleParams, CreateAlertRuleInput } from './types'
import { useAlertSymbols } from './useAlertSymbols'

const RULE_TYPE_OPTIONS = [
  { value: 1, label: '上涨触达（价格 ≥ 目标）' },
  { value: 2, label: '下跌触达（价格 ≤ 目标）' },
  { value: 3, label: '区间触达（突破上/下沿）' },
  { value: 4, label: '相对设定价振幅' },
  { value: 5, label: '5 分钟剧烈波动 %' },
]

const CHANNEL_OPTIONS: { value: AlertChannel; label: string }[] = [
  { value: 'in_app', label: '站内' },
  { value: 'email', label: '邮件' },
  { value: 'pushplus', label: 'PushPlus' },
]

const COLUMNS: MpColumn[] = [
  { key: 'symbol', label: '标的', sortable: true, width: '9%' },
  { key: 'status', label: '状态', sortable: true, width: '8%' },
  { key: 'ruleType', label: '规则', sortable: true, width: '22%' },
  { key: 'params', label: '参数', width: '14%' },
  { key: 'triggerCount', label: '触发', sortable: true, width: '8%', align: 'right' },
  { key: 'lastTriggeredAt', label: '最近触发', sortable: true, width: '16%' },
  { key: 'id', label: '创建', sortable: true, width: '8%' },
  { key: 'actions', label: '操作', width: '15%', align: 'right' },
]

const auth = useAuthStore()
const symbols = useAlertSymbols()
const {
  loading: symbolsLoading,
  hint: symbolsHint,
  optionsForAssetType,
  loadSymbols,
} = symbols

const rules = ref<AlertRule[]>([])
const loading = ref(false)
const listLoading = ref(false)
const error = ref('')
const msg = ref('')

const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const filters = reactive({
  status: '',
  assetType: '',
  symbol: '',
  ruleType: 0,
  sortBy: 'id',
  sortOrder: 'desc' as MpSortOrder,
})

const form = reactive({
  assetType: 'spot' as 'spot' | 'index' | 'alpha',
  symbol: '',
  ruleType: 1,
  target: '',
  upper: '',
  lower: '',
  ampl: '',
  rapidChg: '',
  channels: ['in_app'] as AlertChannel[],
  frequency: 'loop' as AlertFrequency,
  intervalMinutes: 10,
})

const formSymbolOptions = computed(() => optionsForAssetType(form.assetType))
const filterSymbolOptions = computed(() => optionsForAssetType(filters.assetType))

onMounted(async () => {
  await loadSymbols()
  if (!form.symbol) form.symbol = formSymbolOptions.value[0]?.value ?? ''
  await loadRules()
})

async function loadRules(p = page.value) {
  if (!auth.token) return
  listLoading.value = true
  error.value = ''
  try {
    const res = await alertsApi.listRules(auth.token, {
      page: p,
      pageSize: pageSize.value,
      status: filters.status || undefined,
      assetType: filters.assetType || undefined,
      symbol: filters.symbol || undefined,
      ruleType: filters.ruleType || undefined,
      sortBy: filters.sortBy,
      sortOrder: filters.sortOrder,
    })
    rules.value = res.items
    page.value = res.page
    total.value = res.total
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    listLoading.value = false
  }
}

function applyFilters() {
  page.value = 1
  void loadRules(1)
}

function onFilterAssetTypeChange() {
  filters.symbol = ''
  applyFilters()
}

function resetFilters() {
  filters.status = ''
  filters.assetType = ''
  filters.symbol = ''
  filters.ruleType = 0
  filters.sortBy = 'id'
  filters.sortOrder = 'desc'
  page.value = 1
  void loadRules(1)
}

function onSort(key: string, order: MpSortOrder) {
  filters.sortBy = key
  filters.sortOrder = order
  page.value = 1
  void loadRules(1)
}

function onAssetChange() {
  form.symbol = formSymbolOptions.value[0]?.value ?? ''
}

function toggleChannel(ch: AlertChannel) {
  const i = form.channels.indexOf(ch)
  if (i >= 0) {
    if (form.channels.length === 1) return
    form.channels.splice(i, 1)
  } else {
    form.channels.push(ch)
  }
}

function buildParams(): AlertRuleParams {
  const n = (s: string) => {
    const v = Number(s)
    return Number.isFinite(v) ? v : NaN
  }
  switch (form.ruleType) {
    case 1:
    case 2:
      return { target: n(form.target) }
    case 3:
      return { upper: n(form.upper), lower: n(form.lower) }
    case 4:
      return { ampl: n(form.ampl) }
    case 5:
      return { rapid_chg: n(form.rapidChg) }
    default:
      return {}
  }
}

function paramsValid(p: AlertRuleParams): boolean {
  const ok = (v?: number) => v !== undefined && Number.isFinite(v)
  switch (form.ruleType) {
    case 1:
    case 2:
      return ok(p.target)
    case 3:
      return ok(p.upper) && ok(p.lower) && (p.upper as number) > (p.lower as number)
    case 4:
      return ok(p.ampl) && (p.ampl as number) > 0
    case 5:
      return ok(p.rapid_chg) && (p.rapid_chg as number) > 0
    default:
      return false
  }
}

function formatParams(r: AlertRule): string {
  const p = r.params
  switch (r.ruleType) {
    case 1:
    case 2:
      return `目标 ${p.target ?? '-'}`
    case 3:
      return `${p.lower ?? '-'} ~ ${p.upper ?? '-'}`
    case 4:
      return `振幅 ${p.ampl ?? '-'}%`
    case 5:
      return `5m ${p.rapid_chg ?? '-'}%`
    default:
      return '-'
  }
}

function formatTime(ts: number | null): string {
  if (!ts) return '—'
  return new Date(ts * 1000).toLocaleString()
}

function assetTypeLabel(t: string): string {
  if (t === 'spot') return '现货'
  if (t === 'index') return '指数'
  if (t === 'alpha') return '美股参考'
  return t
}

function shortRuleType(t: number): string {
  return RULE_TYPE_OPTIONS.find((o) => o.value === t)?.label.replace(/（.*）/, '') ?? `类型 ${t}`
}

async function onCreate() {
  if (!auth.token) return
  msg.value = ''
  error.value = ''
  if (!form.symbol.trim()) {
    error.value = '请选择标的'
    return
  }
  if (!form.channels.length) {
    error.value = '至少选择一个通道'
    return
  }
  const params = buildParams()
  if (!paramsValid(params)) {
    error.value = '请填写合法阈值参数'
    return
  }
  const input: CreateAlertRuleInput = {
    assetType: form.assetType,
    symbol: form.symbol.trim(),
    ruleType: form.ruleType,
    params,
    channels: [...form.channels],
    frequency: form.frequency,
    intervalMinutes: form.frequency === 'loop' ? form.intervalMinutes : 10,
  }
  loading.value = true
  try {
    await alertsApi.createRule(auth.token, input)
    msg.value = '规则已创建'
    page.value = 1
    await loadRules(1)
  } catch (e) {
    error.value = e instanceof Error ? e.message : '创建失败'
  } finally {
    loading.value = false
  }
}

async function toggleStatus(r: AlertRule) {
  if (!auth.token) return
  const next = r.status === 'active' ? 'disabled' : 'active'
  try {
    await alertsApi.updateRule(auth.token, r.id, { status: next })
    await loadRules()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '更新失败'
  }
}

async function onDelete(r: AlertRule) {
  if (!auth.token) return
  if (!confirm(`删除规则 ${r.symbol} #${r.id}？`)) return
  try {
    await alertsApi.deleteRule(auth.token, r.id)
    msg.value = '已删除'
    await loadRules()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '删除失败'
  }
}
</script>

<template>
  <section class="user-card alert-panel">
    <h2>新建规则</h2>
    <form class="form-grid" @submit.prevent="onCreate">
      <label class="field">
        <span>资产类型</span>
        <select v-model="form.assetType" @change="onAssetChange">
          <option value="spot">现货</option>
          <option value="index">指数</option>
          <option value="alpha">美股参考</option>
        </select>
      </label>
      <label class="field">
        <span>标的</span>
        <select v-model="form.symbol" :disabled="symbolsLoading || !formSymbolOptions.length">
          <option v-if="!formSymbolOptions.length" value="" disabled>
            {{ symbolsLoading ? '加载中…' : '暂无可用标的' }}
          </option>
          <option v-for="opt in formSymbolOptions" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </option>
        </select>
        <span v-if="symbolsHint" class="field-hint">{{ symbolsHint }}</span>
      </label>
      <label class="field wide">
        <span>规则类型</span>
        <select v-model.number="form.ruleType">
          <option v-for="o in RULE_TYPE_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
      </label>

      <label v-if="form.ruleType === 1 || form.ruleType === 2" class="field">
        <span>目标价格</span>
        <input v-model="form.target" type="number" step="any" placeholder="100000" required />
      </label>
      <template v-else-if="form.ruleType === 3">
        <label class="field">
          <span>上沿</span>
          <input v-model="form.upper" type="number" step="any" required />
        </label>
        <label class="field">
          <span>下沿</span>
          <input v-model="form.lower" type="number" step="any" required />
        </label>
      </template>
      <label v-else-if="form.ruleType === 4" class="field">
        <span>振幅 %（相对设定价）</span>
        <input v-model="form.ampl" type="number" step="any" min="0" required />
      </label>
      <label v-else-if="form.ruleType === 5" class="field">
        <span>5 分钟振幅 %</span>
        <input v-model="form.rapidChg" type="number" step="any" min="0" required />
      </label>

      <label class="field">
        <span>频率</span>
        <select v-model="form.frequency">
          <option value="once">仅一次</option>
          <option value="loop">循环</option>
          <option value="daily">每日一次</option>
        </select>
      </label>
      <label v-if="form.frequency === 'loop'" class="field">
        <span>循环间隔（分钟）</span>
        <input v-model.number="form.intervalMinutes" type="number" min="1" max="1440" />
      </label>

      <div class="field wide">
        <span>通道</span>
        <div class="channel-row">
          <label v-for="ch in CHANNEL_OPTIONS" :key="ch.value" class="check">
            <input
              type="checkbox"
              :checked="form.channels.includes(ch.value)"
              @change="toggleChannel(ch.value)"
            />
            {{ ch.label }}
          </label>
        </div>
      </div>

      <p v-if="error" class="form-error">{{ error }}</p>
      <p v-else-if="msg" class="form-ok">{{ msg }}</p>
      <button type="submit" class="primary-btn" :disabled="loading">创建告警</button>
    </form>
  </section>

  <section class="user-card alert-panel">
    <MpListTable
      :columns="COLUMNS"
      :sort-by="filters.sortBy"
      :sort-order="filters.sortOrder"
      :page="page"
      :page-size="pageSize"
      :total="total"
      :loading="listLoading"
      :has-data="rules.length > 0"
      empty-text="暂无规则"
      @sort="onSort"
      @page-change="loadRules"
      @page-size-change="(n) => { pageSize = n; applyFilters() }"
    >
      <template #header>
        <h2>我的规则</h2>
        <button type="button" class="ghost-btn" :disabled="listLoading" @click="loadRules()">刷新</button>
      </template>

      <template #toolbar>
        <label class="tool">
          <span>状态</span>
          <select v-model="filters.status" @change="applyFilters">
            <option value="">全部</option>
            <option value="active">启用</option>
            <option value="disabled">停用</option>
          </select>
        </label>
        <label class="tool">
          <span>类型</span>
          <select v-model="filters.assetType" @change="onFilterAssetTypeChange">
            <option value="">全部</option>
            <option value="spot">现货</option>
            <option value="index">指数</option>
            <option value="alpha">美股参考</option>
          </select>
        </label>
        <label class="tool grow">
          <span>标的</span>
          <select v-model="filters.symbol" @change="applyFilters">
            <option value="">全部标的</option>
            <option v-for="opt in filterSymbolOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </label>
        <label class="tool">
          <span>规则</span>
          <select v-model.number="filters.ruleType" @change="applyFilters">
            <option :value="0">全部</option>
            <option v-for="o in RULE_TYPE_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</option>
          </select>
        </label>
        <button type="button" class="ghost-btn" @click="resetFilters">重置</button>
      </template>

      <tr v-for="r in rules" :key="r.id">
        <td>
          <strong>{{ r.symbol }}</strong>
          <div class="sub">{{ assetTypeLabel(r.assetType) }}</div>
        </td>
        <td>
          <span class="pill" :class="r.status">{{ r.status === 'active' ? '启用' : '停用' }}</span>
        </td>
        <td>
          {{ shortRuleType(r.ruleType) }}
          <div class="sub">{{ r.frequency }}<template v-if="r.frequency === 'loop'"> / {{ r.intervalMinutes }}m</template></div>
        </td>
        <td>{{ formatParams(r) }}</td>
        <td style="text-align: right">{{ r.triggerCount }}</td>
        <td>{{ formatTime(r.lastTriggeredAt) }}</td>
        <td>#{{ r.id }}</td>
        <td class="actions">
          <button type="button" class="ghost-btn" @click="toggleStatus(r)">
            {{ r.status === 'active' ? '停用' : '启用' }}
          </button>
          <button type="button" class="danger-btn" @click="onDelete(r)">删除</button>
        </td>
      </tr>
    </MpListTable>
  </section>
</template>

<style scoped>
.user-card {
  background: var(--card);
  border-radius: 8px;
  padding: 16px;
}

.alert-panel h2 {
  margin: 0 0 12px;
  font-size: 15px;
  color: var(--text-strong);
}

.form-grid {
  display: grid;
  gap: 12px;
}

@media (min-width: 720px) {
  .form-grid {
    grid-template-columns: 1fr 1fr;
  }

  .form-grid .wide,
  .form-grid .primary-btn,
  .form-grid .form-error,
  .form-grid .form-ok {
    grid-column: 1 / -1;
  }
}

.field {
  display: grid;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}

.field input,
.field select,
.tool input,
.tool select {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--text);
  border-radius: 6px;
  padding: 8px 10px;
  font-size: 13px;
}

.tool select {
  padding: 6px 8px;
  font-size: 12px;
}

.field select:disabled {
  opacity: 0.65;
  cursor: wait;
}

.field-hint {
  font-size: 11px;
  color: var(--muted-2);
}

.channel-row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.check {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
}

.primary-btn {
  border: 0;
  background: var(--coin);
  color: #111;
  font-weight: 600;
  border-radius: 6px;
  padding: 10px 14px;
  cursor: pointer;
  justify-self: start;
}

.primary-btn:disabled {
  opacity: 0.6;
  cursor: wait;
}

.ghost-btn,
.danger-btn {
  border: 1px solid var(--line);
  background: transparent;
  color: var(--text);
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 12px;
  cursor: pointer;
}

.danger-btn {
  border-color: color-mix(in srgb, #e5484d 50%, var(--line));
  color: #e5484d;
}

.form-error {
  margin: 0;
  color: #e5484d;
  font-size: 12px;
}

.form-ok {
  margin: 0;
  color: var(--coin);
  font-size: 12px;
}

.tool {
  display: grid;
  gap: 4px;
  font-size: 11px;
  color: var(--muted);
  min-width: 110px;
}

.tool.grow {
  flex: 1 1 160px;
  min-width: 160px;
}

.pill {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--panel);
  border: 1px solid var(--line);
}

.pill.active {
  color: var(--coin);
  border-color: color-mix(in srgb, var(--coin) 40%, var(--line));
}

.pill.disabled {
  color: var(--muted);
}

.sub {
  margin-top: 2px;
  font-size: 11px;
  color: var(--muted);
}

.actions {
  text-align: right;
  white-space: nowrap;
}

.actions .ghost-btn,
.actions .danger-btn {
  margin-left: 4px;
}

@media (max-width: 680px) {
  .user-card {
    padding: 12px;
  }

  .tool {
    min-width: 0;
    flex: 1 1 calc(50% - 8px);
  }

  .tool.grow {
    flex: 1 1 100%;
    min-width: 0;
  }

  .actions {
    text-align: left;
    white-space: normal;
  }

  .actions .ghost-btn,
  .actions .danger-btn {
    margin: 4px 4px 0 0;
  }

  .primary-btn {
    width: 100%;
    justify-self: stretch;
  }
}
</style>
