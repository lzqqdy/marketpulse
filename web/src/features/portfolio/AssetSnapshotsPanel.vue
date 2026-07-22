<script setup lang="ts">
import { onMounted, ref } from 'vue'
import MpListTable from '@/components/MpListTable.vue'
import type { MpColumn, MpSortOrder } from '@/components/mpListTable'
import { useAuthStore } from '@/features/auth/stores/auth'
import * as api from './api'
import type { PortfolioSnapshot } from './types'

const COLUMNS: MpColumn[] = [
  { key: 'date', label: 'Date', sortable: true, width: '16%' },
  { key: 'totalValueCny', label: 'Total', sortable: true, width: '16%', align: 'right' },
  { key: 'dailyProfit', label: 'Daily', sortable: true, width: '16%', align: 'right' },
  { key: 'dailyProfitRate', label: 'D PNL', sortable: false, width: '14%', align: 'right' },
  { key: 'totalProfit', label: 'Profits', sortable: false, width: '18%', align: 'right' },
  { key: 'totalProfitRate', label: 'P PNL', sortable: false, width: '14%', align: 'right' },
]

const auth = useAuthStore()
const items = ref<PortfolioSnapshot[]>([])
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const loading = ref(false)
const error = ref('')
const sortBy = ref('date')
const sortOrder = ref<MpSortOrder>('desc')

function fmtMoney(n: number): string {
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function fmtSigned(n: number): string {
  if (!Number.isFinite(n)) return '—'
  const body = Math.abs(n).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
  if (n > 0) return `+${body}`
  if (n < 0) return `-${body}`
  return body
}

/** rate is decimal (0.0164 => 1.64%) */
function fmtRate(rate: number): string {
  if (!Number.isFinite(rate)) return '—'
  const pct = rate * 100
  const body = Math.abs(pct).toFixed(2)
  if (pct > 0) return `+${body}%`
  if (pct < 0) return `-${body}%`
  return `${body}%`
}

function trendClass(n: number) {
  if (n > 0) return 'up'
  if (n < 0) return 'down'
  return 'flat'
}

async function load(p = page.value) {
  if (!auth.token) return
  loading.value = true
  error.value = ''
  try {
    const res = await api.listSnapshots(auth.token, {
      page: p,
      pageSize: pageSize.value,
      sort: sortBy.value,
      order: sortOrder.value,
    })
    items.value = res.items ?? []
    total.value = res.total
    page.value = res.page
    pageSize.value = res.pageSize
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function onSort(key: string, order: MpSortOrder) {
  sortBy.value = key
  sortOrder.value = order
  void load(1)
}

onMounted(() => load(1))

defineExpose({ reload: () => load(page.value) })
</script>

<template>
  <section class="snapshots-panel">
    <h2>详情</h2>
    <p v-if="error" class="err">{{ error }}</p>
    <MpListTable
      :columns="COLUMNS"
      :page="page"
      :page-size="pageSize"
      :total="total"
      :loading="loading"
      :has-data="items.length > 0"
      empty-text="暂无日快照"
      :sort-by="sortBy"
      :sort-order="sortOrder"
      @sort="onSort"
      @page-change="load"
      @page-size-change="(n) => { pageSize = n; load(1) }"
    >
      <template #header>
        <span />
        <button type="button" class="ghost-btn" :disabled="loading" @click="load(page)">刷新</button>
      </template>
      <tr v-for="(row, idx) in items" :key="row.date" :class="{ zebra: idx % 2 === 1 }">
        <td>{{ row.date }}</td>
        <td class="num">{{ fmtMoney(row.totalValueCny) }}</td>
        <td class="num" :class="trendClass(row.dailyProfit)">{{ fmtSigned(row.dailyProfit) }}</td>
        <td class="num" :class="trendClass(row.dailyProfit)">{{ fmtRate(row.dailyProfitRate) }}</td>
        <td class="num" :class="trendClass(row.totalProfit)">{{ fmtSigned(row.totalProfit) }}</td>
        <td class="num" :class="trendClass(row.totalProfit)">{{ fmtRate(row.totalProfitRate) }}</td>
      </tr>
    </MpListTable>
  </section>
</template>

<style scoped>
.snapshots-panel h2 {
  margin: 0 0 10px;
  font-size: 15px;
  color: var(--coin);
}

.err {
  color: var(--warning);
  font-size: 13px;
}

.num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.up {
  color: var(--up);
}
.down {
  color: var(--down);
}
.flat {
  color: var(--text);
}

.ghost-btn {
  border: 1px solid var(--line);
  background: transparent;
  color: var(--text);
  border-radius: 6px;
  padding: 6px 12px;
  font-size: 12px;
  cursor: pointer;
}

.ghost-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

:deep(tr.zebra td) {
  background: color-mix(in srgb, var(--panel) 70%, transparent);
}
</style>
