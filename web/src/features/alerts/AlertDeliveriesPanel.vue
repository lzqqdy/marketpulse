<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import MpListTable from '@/components/MpListTable.vue'
import type { MpColumn, MpSortOrder } from '@/components/mpListTable'
import { useAuthStore } from '@/features/auth/stores/auth'
import * as alertsApi from './api'
import type { AlertDelivery } from './types'
import { useAlertSymbols } from './useAlertSymbols'

const RULE_TYPE_OPTIONS = [
  { value: 0, label: '全部规则类型' },
  { value: 1, label: '上涨触达' },
  { value: 2, label: '下跌触达' },
  { value: 3, label: '区间触达' },
  { value: 4, label: '相对振幅' },
  { value: 5, label: '5分钟剧震' },
]

const COLUMNS: MpColumn[] = [
  { key: 'createdAt', label: '时间', sortable: true, width: '14%' },
  { key: 'symbol', label: '标的', sortable: true, width: '9%' },
  { key: 'title', label: '标题 / 内容', width: '34%' },
  { key: 'channel', label: '通道', sortable: true, width: '10%' },
  { key: 'status', label: '状态', sortable: true, width: '9%' },
  { key: 'ruleId', label: '规则', sortable: true, width: '8%' },
  { key: 'id', label: 'ID', sortable: true, width: '8%' },
  { key: 'triggerValue', label: '触发值', width: '8%', align: 'right' },
]

const auth = useAuthStore()
const { optionsForAssetType, loadSymbols } = useAlertSymbols()

const items = ref<AlertDelivery[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const loading = ref(false)
const error = ref('')

const filters = reactive({
  channel: '',
  status: '',
  assetType: '',
  symbol: '',
  ruleType: 0,
  ruleId: '',
  sortBy: 'createdAt',
  sortOrder: 'desc' as MpSortOrder,
})

const channelLabel: Record<string, string> = {
  in_app: '站内',
  email: '邮件',
  pushplus: 'PushPlus',
}

const filterSymbolOptions = computed(() => optionsForAssetType(filters.assetType))

onMounted(async () => {
  await loadSymbols()
  await load()
})

async function load(p = page.value) {
  if (!auth.token) return
  loading.value = true
  error.value = ''
  try {
    const ruleIdNum = Number(filters.ruleId)
    const res = await alertsApi.listDeliveries(auth.token, {
      page: p,
      pageSize: pageSize.value,
      channel: filters.channel || undefined,
      status: filters.status || undefined,
      assetType: filters.assetType || undefined,
      symbol: filters.symbol || undefined,
      ruleType: filters.ruleType || undefined,
      ruleId: Number.isFinite(ruleIdNum) && ruleIdNum > 0 ? ruleIdNum : undefined,
      sortBy: filters.sortBy,
      sortOrder: filters.sortOrder,
    })
    items.value = res.items
    page.value = res.page
    total.value = res.total
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  page.value = 1
  void load(1)
}

function onFilterAssetTypeChange() {
  filters.symbol = ''
  applyFilters()
}

function resetFilters() {
  filters.channel = ''
  filters.status = ''
  filters.assetType = ''
  filters.symbol = ''
  filters.ruleType = 0
  filters.ruleId = ''
  filters.sortBy = 'createdAt'
  filters.sortOrder = 'desc'
  page.value = 1
  void load(1)
}

function onSort(key: string, order: MpSortOrder) {
  filters.sortBy = key
  filters.sortOrder = order
  page.value = 1
  void load(1)
}

function formatTime(ts: number): string {
  return new Date(ts * 1000).toLocaleString()
}
</script>

<template>
  <section class="user-card">
    <MpListTable
      :columns="COLUMNS"
      :sort-by="filters.sortBy"
      :sort-order="filters.sortOrder"
      :page="page"
      :page-size="pageSize"
      :total="total"
      :loading="loading"
      :has-data="items.length > 0"
      empty-text="暂无推送记录"
      @sort="onSort"
      @page-change="load"
      @page-size-change="(n) => { pageSize = n; applyFilters() }"
    >
      <template #header>
        <h2>推送记录</h2>
        <button type="button" class="ghost-btn" :disabled="loading" @click="load()">刷新</button>
      </template>

      <template #toolbar>
        <label class="tool">
          <span>通道</span>
          <select v-model="filters.channel" @change="applyFilters">
            <option value="">全部</option>
            <option value="in_app">站内</option>
            <option value="email">邮件</option>
            <option value="pushplus">PushPlus</option>
          </select>
        </label>
        <label class="tool">
          <span>状态</span>
          <select v-model="filters.status" @change="applyFilters">
            <option value="">全部</option>
            <option value="success">success</option>
            <option value="failed">failed</option>
            <option value="skipped">skipped</option>
          </select>
        </label>
        <label class="tool">
          <span>资产</span>
          <select v-model="filters.assetType" @change="onFilterAssetTypeChange">
            <option value="">全部</option>
            <option value="spot">现货</option>
            <option value="index">指数</option>
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
          <span>规则类型</span>
          <select v-model.number="filters.ruleType" @change="applyFilters">
            <option v-for="o in RULE_TYPE_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</option>
          </select>
        </label>
        <label class="tool">
          <span>规则 ID</span>
          <input v-model="filters.ruleId" type="number" min="1" placeholder="可选" @keyup.enter="applyFilters" />
        </label>
        <button type="button" class="ghost-btn" @click="resetFilters">重置</button>
      </template>

      <tr v-for="d in items" :key="d.id">
        <td>{{ formatTime(d.createdAt) }}</td>
        <td>
          <strong>{{ d.symbol }}</strong>
          <div class="sub">{{ d.assetType === 'spot' ? '现货' : '指数' }}</div>
        </td>
        <td>
          <div class="title">{{ d.title }}</div>
          <div class="body">{{ d.body }}</div>
          <div v-if="d.errorMsg" class="err">{{ d.errorMsg }}</div>
        </td>
        <td>{{ channelLabel[d.channel] || d.channel }}</td>
        <td><span class="pill" :class="d.status">{{ d.status }}</span></td>
        <td>#{{ d.ruleId }}</td>
        <td>#{{ d.id }}</td>
        <td style="text-align: right">{{ d.triggerValue }}</td>
      </tr>
    </MpListTable>
    <p v-if="error" class="form-error">{{ error }}</p>
  </section>
</template>

<style scoped>
.user-card {
  background: var(--card);
  border-radius: 8px;
  padding: 16px;
}

.user-card h2 {
  margin: 0;
  font-size: 15px;
  color: var(--text-strong);
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

.tool input,
.tool select {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--text);
  border-radius: 6px;
  padding: 6px 8px;
  font-size: 12px;
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

.ghost-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.sub {
  margin-top: 2px;
  font-size: 11px;
  color: var(--muted);
}

.title {
  font-weight: 600;
  color: var(--text-strong);
}

.body {
  margin-top: 4px;
  white-space: pre-line;
  color: var(--text);
  line-height: 1.4;
}

.err {
  margin-top: 4px;
  color: #e5484d;
  font-size: 11px;
}

.pill {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--panel);
  border: 1px solid var(--line);
  text-transform: lowercase;
}

.pill.success {
  color: var(--coin);
}

.pill.failed {
  color: #e5484d;
}

.pill.skipped {
  color: var(--muted);
}

.form-error {
  margin: 8px 0 0;
  color: #e5484d;
  font-size: 12px;
}
</style>
