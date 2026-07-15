<script setup lang="ts">
import { computed } from 'vue'
import type { MpColumn, MpSortOrder } from './mpListTable'

const props = withDefaults(
  defineProps<{
    columns: MpColumn[]
    sortBy?: string
    sortOrder?: MpSortOrder
    page?: number
    pageSize?: number
    total?: number
    loading?: boolean
    hasData?: boolean
    emptyText?: string
    pageSizeOptions?: number[]
  }>(),
  {
    sortBy: '',
    sortOrder: 'desc',
    page: 1,
    pageSize: 20,
    total: 0,
    loading: false,
    hasData: false,
    emptyText: '暂无数据',
    pageSizeOptions: () => [10, 20, 50],
  },
)

const emit = defineEmits<{
  sort: [key: string, order: MpSortOrder]
  pageChange: [page: number]
  pageSizeChange: [pageSize: number]
}>()

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

function onSort(col: MpColumn) {
  if (!col.sortable) return
  if (props.sortBy === col.key) {
    emit('sort', col.key, props.sortOrder === 'asc' ? 'desc' : 'asc')
  } else {
    emit('sort', col.key, 'desc')
  }
}

function sortMark(col: MpColumn): string {
  if (!col.sortable) return ''
  if (props.sortBy !== col.key) return '↕'
  return props.sortOrder === 'asc' ? '↑' : '↓'
}
</script>

<template>
  <div class="mp-list">
    <div v-if="$slots.header" class="mp-list-head">
      <slot name="header" />
    </div>
    <div v-if="$slots.toolbar" class="mp-list-toolbar">
      <slot name="toolbar" />
    </div>

    <div class="mp-list-scroll">
      <table class="mp-table">
        <colgroup>
          <col v-for="col in columns" :key="col.key" :style="col.width ? { width: col.width } : undefined" />
        </colgroup>
        <thead>
          <tr>
            <th
              v-for="col in columns"
              :key="col.key"
              :class="{ sortable: col.sortable, active: col.sortable && sortBy === col.key }"
              :style="{ textAlign: col.align || 'left' }"
              @click="onSort(col)"
            >
              <span class="th-inner">
                {{ col.label }}
                <span v-if="col.sortable" class="sort-mark" aria-hidden="true">{{ sortMark(col) }}</span>
              </span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && !hasData">
            <td :colspan="columns.length" class="empty">加载中…</td>
          </tr>
          <tr v-else-if="!hasData">
            <td :colspan="columns.length" class="empty">{{ emptyText }}</td>
          </tr>
          <slot v-else />
        </tbody>
      </table>
    </div>

    <div v-if="total > 0" class="mp-pager">
      <button
        type="button"
        class="ghost-btn"
        :disabled="page <= 1 || loading"
        @click="emit('pageChange', page - 1)"
      >
        上一页
      </button>
      <span>{{ page }} / {{ totalPages }}（共 {{ total }}）</span>
      <button
        type="button"
        class="ghost-btn"
        :disabled="page >= totalPages || loading"
        @click="emit('pageChange', page + 1)"
      >
        下一页
      </button>
      <select
        class="page-size"
        :value="pageSize"
        @change="emit('pageSizeChange', Number(($event.target as HTMLSelectElement).value))"
      >
        <option v-for="n in pageSizeOptions" :key="n" :value="n">{{ n }}/页</option>
      </select>
    </div>
  </div>
</template>

<style scoped>
.mp-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.mp-list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.mp-list-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: end;
}

.mp-list-scroll {
  overflow-x: auto;
  border: 1px solid var(--line);
  border-radius: 8px;
}

.mp-table {
  width: 100%;
  border-collapse: collapse;
  min-width: 640px;
  font-size: 12px;
}

.mp-table th,
.mp-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--line);
  color: var(--text);
  vertical-align: top;
}

.mp-table th {
  background: var(--panel);
  color: var(--muted);
  font-weight: 600;
  white-space: nowrap;
  user-select: none;
}

.mp-table th.sortable {
  cursor: pointer;
}

.mp-table th.sortable:hover,
.mp-table th.active {
  color: var(--coin);
}

.th-inner {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.sort-mark {
  font-size: 10px;
  opacity: 0.7;
}

.mp-table tbody tr:last-child td {
  border-bottom: 0;
}

.mp-table tbody tr:hover td {
  background: var(--hover);
}

.empty {
  text-align: center;
  color: var(--muted);
  padding: 20px 12px !important;
}

.mp-pager {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 10px;
  font-size: 12px;
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

.ghost-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.page-size {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--text);
  border-radius: 6px;
  padding: 6px 8px;
  font-size: 12px;
}
</style>
