<script setup lang="ts">
import { computed, onMounted, ref, shallowRef, watch } from 'vue'
import { fetchExpressNews } from '@/features/market/api/expressNews'
import { useInfiniteScroll } from '@/features/market/composables/useInfiniteScroll'
import { useTrendClass } from '@/features/market/composables/useTrendClass'
import { formatPct } from '@/utils/format'
import type { ExpressNewsItem, ExpressNewsTag } from '@/features/market/types/expressNews'
import { EXPRESS_NEWS_TABS } from '@/features/market/types/expressNews'

const PAGE_SIZE = 20

const { priceClass } = useTrendClass()

const activeTag = ref<ExpressNewsTag>('')
const filterWatchlist = ref(false)
const pn = ref(0)
const hasMore = ref(true)
const loading = ref(false)
const loadingMore = ref(false)
const error = ref('')
const fetchedAt = ref(0)

const sentinel = ref<HTMLElement | null>(null)

interface ViewEntity {
  code: string
  name: string
  logoUrl?: string
  changePct?: number
  tagLabel: string
  trendClass: string
}

interface ViewNewsItem {
  id: string
  provider: string
  timeLabel: string
  contentText: string
  relatedLogos: ViewEntity[]
  entities: ViewEntity[]
}

type DisplayRow =
  | { type: 'date'; key: string; label: string }
  | { type: 'item'; key: string; item: ViewNewsItem }

const displayRows = shallowRef<DisplayRow[]>([])

const updatedLabel = computed(() => {
  if (!fetchedAt.value) return ''
  return new Date(fetchedAt.value * 1000).toLocaleTimeString('zh-CN', { hour12: false })
})

function dateKey(ts: number): string {
  const d = new Date(ts * 1000)
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`
}

function formatDateHeader(ts: number): string {
  const d = new Date(ts * 1000)
  const weekdays = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六']
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}年${pad(d.getMonth() + 1)}月${pad(d.getDate())}日 ${weekdays[d.getDay()]}`
}

function formatNewsTime(ts: number): string {
  const d = new Date(ts * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function marketPrefix(market: string): string {
  const m = market.toLowerCase()
  if (m === 'hk') return 'HK'
  if (m === 'us') return 'US'
  if (m === 'ab' || m === 'cn' || m === 'sh' || m === 'sz') return 'A'
  return market.toUpperCase()
}

function newsContent(item: ExpressNewsItem): string {
  const title = item.title.trim()
  const body = item.body.trim()
  if (!title) return body
  if (body.startsWith(`【${title}】`) || body.startsWith(title)) return body
  return `【${title}】${body}`
}

function toViewEntity(entity: NonNullable<ExpressNewsItem['entities']>[number]): ViewEntity {
  const prefix = marketPrefix(entity.market)
  const name = entity.name.length > 12 ? `${entity.name.slice(0, 10)}…` : entity.name
  const changePct = entity.changePct ?? 0
  return {
    code: entity.code,
    name: entity.name,
    logoUrl: entity.logoUrl,
    changePct,
    tagLabel: `${prefix} ${name} ${formatPct(changePct)}`,
    trendClass: priceClass(changePct),
  }
}

function toViewItem(item: ExpressNewsItem): ViewNewsItem {
  const entities = (item.entities ?? []).map(toViewEntity)
  return {
    id: item.id,
    provider: item.provider || '--',
    timeLabel: formatNewsTime(item.publishTime),
    contentText: newsContent(item),
    relatedLogos: entities.slice(0, 3).filter((e) => !!e.logoUrl),
    entities,
  }
}

function buildRows(items: ExpressNewsItem[]): DisplayRow[] {
  const rows: DisplayRow[] = []
  let lastDate = ''
  for (const raw of items) {
    const key = dateKey(raw.publishTime)
    if (key !== lastDate) {
      rows.push({ type: 'date', key: `date-${key}`, label: formatDateHeader(raw.publishTime) })
      lastDate = key
    }
    rows.push({ type: 'item', key: raw.id, item: toViewItem(raw) })
  }
  return rows
}

function appendRows(rows: DisplayRow[], fresh: ExpressNewsItem[], prevLastDateKey: string): string {
  let lastDate = prevLastDateKey
  for (const raw of fresh) {
    const key = dateKey(raw.publishTime)
    if (key !== lastDate) {
      rows.push({ type: 'date', key: `date-${key}`, label: formatDateHeader(raw.publishTime) })
      lastDate = key
    }
    rows.push({ type: 'item', key: raw.id, item: toViewItem(raw) })
  }
  return lastDate
}

let lastDateKey = ''

const seenIds = new Set<string>()

async function loadPage(page: number, append: boolean) {
  if (page === 0) {
    loading.value = true
    error.value = ''
  } else {
    loadingMore.value = true
  }

  try {
    const resp = await fetchExpressNews({
      tag: activeTag.value,
      pn: page,
      rn: PAGE_SIZE,
      filterByUserStocks: filterWatchlist.value,
    })
    fetchedAt.value = resp.fetchedAt
    hasMore.value = resp.hasMore
    pn.value = page

    if (append) {
      const fresh = resp.items.filter((item) => !seenIds.has(item.id))
      for (const item of fresh) seenIds.add(item.id)
      const nextRows = displayRows.value.slice()
      lastDateKey = appendRows(nextRows, fresh, lastDateKey)
      displayRows.value = nextRows
    } else {
      seenIds.clear()
      for (const item of resp.items) seenIds.add(item.id)
      displayRows.value = buildRows(resp.items)
      const tail = resp.items[resp.items.length - 1]
      lastDateKey = tail ? dateKey(tail.publishTime) : ''
    }
  } catch (e) {
    const msg = e instanceof Error ? e.message : '快讯加载失败'
    if (!append) {
      error.value = msg
      displayRows.value = []
      seenIds.clear()
      lastDateKey = ''
    }
  } finally {
    loading.value = false
    loadingMore.value = false
    reconnect()
  }
}

function resetAndLoad() {
  pn.value = 0
  hasMore.value = true
  void loadPage(0, false)
}

function loadMore() {
  if (loading.value || loadingMore.value || !hasMore.value) return
  return loadPage(pn.value + 1, true)
}

function canLoadMore() {
  return !loading.value && !loadingMore.value && hasMore.value && displayRows.value.length > 0
}

const { reconnect } = useInfiniteScroll(sentinel, loadMore, canLoadMore, {
  rootMargin: '0px 0px 80px 0px',
})

watch([activeTag, filterWatchlist], () => resetAndLoad())

onMounted(() => {
  void loadPage(0, false)
})
</script>

<template>
  <section class="express-news">
    <header class="en-head">
      <div class="en-title">
        <h2>7*24快讯</h2>
        <p v-if="updatedLabel" class="en-meta">更新于 {{ updatedLabel }}</p>
      </div>
      <div class="en-toolbar">
        <div class="en-tabs" role="tablist" aria-label="快讯分类">
          <button
            v-for="tab in EXPRESS_NEWS_TABS"
            :key="tab.label"
            type="button"
            class="en-tab"
            :class="{ active: activeTag === tab.value }"
            :disabled="loading"
            @click="activeTag = tab.value"
          >
            {{ tab.label }}
          </button>
        </div>
        <label class="en-watchlist">
          <input v-model="filterWatchlist" type="checkbox" :disabled="loading" />
          <span>自选</span>
        </label>
      </div>
    </header>

    <div class="en-table-head" aria-hidden="true">
      <span>时间</span>
      <span>关联</span>
      <span>内容</span>
      <span>来源</span>
    </div>

    <p v-if="loading && !displayRows.length" class="en-status">快讯加载中…</p>
    <p v-else-if="error && !displayRows.length" class="en-status en-error">{{ error }}</p>
    <p v-else-if="!displayRows.length" class="en-status">暂无快讯</p>

    <div v-else class="en-list">
      <template v-for="row in displayRows" :key="row.key">
        <div v-if="row.type === 'date'" class="en-date-row">{{ row.label }}</div>
        <article v-else class="en-row">
          <time class="en-col-time">{{ row.item.timeLabel }}</time>
          <div class="en-col-related">
            <img
              v-for="entity in row.item.relatedLogos"
              :key="`${row.item.id}-${entity.code}`"
              class="en-logo"
              :src="entity.logoUrl"
              :alt="entity.name"
              width="22"
              height="22"
              decoding="async"
              loading="lazy"
            />
          </div>
          <div class="en-col-content">
            <p class="en-text">{{ row.item.contentText }}</p>
            <div v-if="row.item.entities.length" class="en-entity-tags">
              <span
                v-for="entity in row.item.entities"
                :key="`${row.item.id}-tag-${entity.code}`"
                class="en-entity-tag"
                :class="entity.trendClass"
              >
                {{ entity.tagLabel }}
              </span>
            </div>
          </div>
          <span class="en-col-source">{{ row.item.provider }}</span>
        </article>
      </template>
    </div>

    <div class="en-footer">
      <div ref="sentinel" class="en-sentinel" aria-hidden="true" />
      <p v-if="loadingMore" class="en-status">加载更多…</p>
      <p v-else-if="!hasMore && displayRows.length" class="en-status en-end">已加载全部</p>
    </div>
  </section>
</template>

<style scoped>
.express-news {
  width: 100%;
  margin-top: 16px;
  padding: 10px 12px;
  background: var(--card);
  border-radius: 6px;
  text-align: left;
  contain: layout style;
}

.en-head {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 10px;
}

.en-title h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-strong);
}

.en-meta {
  margin: 4px 0 0;
  font-size: 11px;
  color: var(--muted);
}

.en-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.en-tabs {
  display: flex;
  gap: 4px;
  flex: 1 1 auto;
  min-width: 0;
}

.en-tab {
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--text);
  padding: 6px 10px;
  font-size: 13px;
  cursor: pointer;
  flex: 1 1 0;
  min-width: 0;
}

.en-tab.active {
  color: var(--coin);
  border-bottom-color: var(--coin);
  font-weight: 600;
}

.en-tab:disabled {
  opacity: 0.6;
  cursor: default;
}

.en-watchlist {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--muted);
  cursor: pointer;
  flex-shrink: 0;
  user-select: none;
}

.en-table-head,
.en-row {
  display: grid;
  grid-template-columns: 72px 56px 1fr 64px;
  gap: 8px;
  align-items: start;
}

.en-table-head {
  padding: 8px 10px;
  background: var(--card-soft);
  border-radius: 6px;
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 4px;
}

.en-list {
  contain: layout style;
}

.en-date-row {
  padding: 10px 10px 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-strong);
  border-top: 1px solid var(--line);
  content-visibility: auto;
  contain-intrinsic-size: auto 36px;
}

.en-list > .en-date-row:first-child {
  border-top: 0;
  padding-top: 4px;
}

.en-row {
  padding: 10px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
  content-visibility: auto;
  contain-intrinsic-size: auto 72px;
  contain: layout style paint;
}

.en-col-time {
  font-size: 12px;
  color: var(--muted);
  white-space: nowrap;
}

.en-col-related {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: center;
  min-height: 22px;
}

.en-logo {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  object-fit: contain;
  background: var(--card-soft);
  flex-shrink: 0;
}

.en-col-content {
  min-width: 0;
}

.en-text {
  margin: 0;
  font-size: 13px;
  line-height: 1.55;
  color: var(--text);
  word-break: break-word;
}

.en-entity-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 6px;
}

.en-entity-tag {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--card-soft);
  border: 1px solid var(--line);
  white-space: nowrap;
}

.en-entity-tag.up {
  color: var(--up);
  border-color: rgba(239, 68, 68, 0.25);
  background: rgba(239, 68, 68, 0.06);
}

.en-entity-tag.down {
  color: var(--down);
  border-color: rgba(34, 197, 94, 0.25);
  background: rgba(34, 197, 94, 0.06);
}

.en-entity-tag.flat {
  color: var(--muted);
}

.en-col-source {
  font-size: 12px;
  color: var(--muted);
  text-align: right;
  word-break: break-all;
}

.en-footer {
  min-height: 40px;
}

.en-status {
  text-align: center;
  font-size: 12px;
  color: var(--muted);
  padding: 12px 0;
  margin: 0;
}

.en-error {
  color: var(--up);
}

.en-end {
  padding-bottom: 4px;
}

.en-sentinel {
  height: 1px;
  width: 100%;
  pointer-events: none;
}

@media (max-width: 640px) {
  .en-table-head {
    display: none;
  }

  .en-row {
    grid-template-columns: 64px 1fr;
    grid-template-areas:
      'time source'
      'related related'
      'content content';
  }

  .en-col-time {
    grid-area: time;
  }

  .en-col-source {
    grid-area: source;
  }

  .en-col-related {
    grid-area: related;
    flex-direction: row;
    justify-content: flex-start;
  }

  .en-col-content {
    grid-area: content;
  }
}
</style>
