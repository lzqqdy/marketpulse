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
  gap: 12px;
  min-width: 0;
  max-width: 100%;
}

.mp-list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
}

.mp-list-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: end;
}

.mp-list-scroll {
  overflow: auto;
  -webkit-overflow-scrolling: touch;
  border: 1px solid var(--line);
  border-radius: var(--radius, 8px);
  min-width: 0;
  max-width: 100%;
  background: color-mix(in srgb, var(--panel) 55%, transparent);
}

.mp-table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  min-width: 720px;
  font-size: 13px;
}

@media (min-width: 681px) {
  .mp-list-toolbar {
    padding: 12px 14px;
    border: 1px solid var(--line);
    border-radius: var(--radius, 8px);
    background: color-mix(in srgb, var(--card-soft, var(--panel)) 78%, transparent);
  }

  .mp-list-scroll {
    max-height: min(64vh, 760px);
  }

  .mp-pager {
    justify-content: space-between;
    padding: 2px 4px;
  }

  .mp-pager > span {
    margin-right: auto;
  }
}

@media (max-width: 680px) {
  .mp-table {
    min-width: 0;
    font-size: 12px;
  }

  .mp-pager {
    gap: 8px;
    font-size: 11px;
  }

  .mp-list-toolbar {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
}

.mp-table th,
.mp-table td {
  padding: 12px 14px;
  border-bottom: 1px solid var(--line);
  color: var(--text);
  vertical-align: middle;
}

.mp-table th {
  position: sticky;
  top: 0;
  z-index: 1;
  background: color-mix(in srgb, var(--panel) 94%, var(--card, transparent));
  color: var(--muted);
  font-weight: 600;
  font-size: 11px;
  letter-spacing: 0.02em;
  white-space: nowrap;
  user-select: none;
  box-shadow: 0 1px 0 var(--line);
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

.mp-table tbody tr:nth-child(even) td {
  background: color-mix(in srgb, var(--hover) 42%, transparent);
}

.mp-table tbody tr:last-child td {
  border-bottom: 0;
}

.mp-table tbody tr:hover td {
  background: var(--hover-strong, var(--hover));
}

.empty {
  text-align: center;
  color: var(--muted);
  padding: 28px 12px !important;
  font-size: 13px;
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
  border-radius: var(--radius-sm, 6px);
  padding: 6px 12px;
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
  border-radius: var(--radius-sm, 6px);
  padding: 6px 8px;
  font-size: 12px;
}
</style>
