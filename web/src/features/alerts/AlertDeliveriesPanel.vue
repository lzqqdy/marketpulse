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
  { key: 'createdAt', label: '时间', sortable: true, width: '13%' },
  { key: 'symbol', label: '标的', sortable: true, width: '10%' },
  { key: 'title', label: '标题 / 内容', width: '36%' },
  { key: 'channel', label: '通道', sortable: true, width: '9%' },
  { key: 'status', label: '状态', sortable: true, width: '9%' },
  { key: 'ruleId', label: '规则', sortable: true, width: '7%' },
  { key: 'id', label: 'ID', sortable: true, width: '7%' },
  { key: 'triggerValue', label: '触发值', width: '9%', align: 'right' },
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
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

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

function assetLabel(t: string) {
  if (t === 'spot') return '现货'
  if (t === 'alpha') return '美股参考'
  return '指数'
}
</script>

<template>
  <section class="user-card">
    <div class="desktop-list">
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
            <div class="sub">{{ assetLabel(d.assetType) }}</div>
          </td>
          <td>
            <div class="title">{{ d.title }}</div>
            <div class="body">{{ d.body }}</div>
            <div v-if="d.errorMsg" class="err">{{ d.errorMsg }}</div>
          </td>
          <td>{{ channelLabel[d.channel] || d.channel }}</td>
          <td><span class="status-pill" :class="d.status">{{ d.status }}</span></td>
          <td>#{{ d.ruleId }}</td>
          <td>#{{ d.id }}</td>
          <td style="text-align: right">{{ d.triggerValue }}</td>
        </tr>
      </MpListTable>
    </div>

    <div class="mobile-cards" aria-label="推送记录">
      <div class="mobile-cards-head">
        <h2>推送记录</h2>
        <button type="button" class="ghost-btn" :disabled="loading" @click="load()">刷新</button>
      </div>
      <div class="mobile-filters">
        <select v-model="filters.channel" @change="applyFilters">
          <option value="">全部通道</option>
          <option value="in_app">站内</option>
          <option value="email">邮件</option>
          <option value="pushplus">PushPlus</option>
        </select>
        <select v-model="filters.status" @change="applyFilters">
          <option value="">全部状态</option>
          <option value="success">success</option>
          <option value="failed">failed</option>
          <option value="skipped">skipped</option>
        </select>
      </div>
      <p v-if="loading && !items.length" class="loading-state">加载中…</p>
      <p v-else-if="!items.length" class="empty-state">暂无推送记录</p>
      <article v-for="d in items" :key="'m-' + d.id" class="delivery-card">
        <div class="delivery-top">
          <div>
            <strong>{{ d.symbol }}</strong>
            <span class="sub"> {{ assetLabel(d.assetType) }}</span>
          </div>
          <span class="status-pill" :class="d.status">{{ d.status }}</span>
        </div>
        <div class="title">{{ d.title }}</div>
        <div class="body">{{ d.body }}</div>
        <div v-if="d.errorMsg" class="err">{{ d.errorMsg }}</div>
        <div class="delivery-foot">
          <span>{{ channelLabel[d.channel] || d.channel }} · {{ formatTime(d.createdAt) }}</span>
          <span>触发 {{ d.triggerValue }}</span>
        </div>
      </article>
      <div v-if="total > pageSize" class="mobile-pager">
        <button type="button" class="ghost-btn" :disabled="page <= 1 || loading" @click="load(page - 1)">上一页</button>
        <span>{{ page }} / {{ totalPages }}</span>
        <button type="button" class="ghost-btn" :disabled="page >= totalPages || loading" @click="load(page + 1)">下一页</button>
      </div>
    </div>

    <p v-if="error" class="form-error">{{ error }}</p>
  </section>
</template>

<style scoped>
.user-card {
  background: var(--card);
  border-radius: var(--radius);
  padding: 16px;
}

.user-card h2 {
  margin: 0;
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
  border-radius: var(--radius-sm);
  padding: 6px 8px;
  font-size: 12px;
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

@media (min-width: 681px) {
  .desktop-list :deep(.mp-table) {
    min-width: 920px;
  }

  .desktop-list :deep(.mp-table td) {
    line-height: 1.35;
  }

  .body {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    white-space: normal;
    max-width: 42rem;
  }
}

.err {
  margin-top: 4px;
  color: var(--danger);
  font-size: 11px;
  overflow-wrap: anywhere;
}

.form-error {
  margin: 8px 0 0;
}

.mobile-cards {
  display: none;
}

.mobile-cards-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}

.mobile-filters {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-bottom: 10px;
}

.mobile-filters select {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--text);
  border-radius: var(--radius-sm);
  padding: 8px 10px;
  font-size: 13px;
}

.delivery-card {
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 12px;
  background: color-mix(in srgb, var(--panel) 90%, transparent);
  display: grid;
  gap: 6px;
  margin-bottom: 10px;
}

.delivery-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.delivery-foot {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 6px;
  font-size: 11px;
  color: var(--muted);
}

.mobile-pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  font-size: 12px;
  color: var(--muted);
}

@media (max-width: 680px) {
  .user-card {
    padding: 12px;
  }

  .desktop-list {
    display: none;
  }

  .mobile-cards {
    display: block;
  }

  .body {
    overflow-wrap: anywhere;
  }
}
</style>
