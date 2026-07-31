<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { fetchEvent, fetchEventTimeline, fetchEvents } from '../api'
import { connectEventsStream } from '../useEventsStream'
import type {
  EventTimelineItem,
  MarketEventDetail,
  MarketEventSummary,
} from '../types'
import { useMarketStore } from '@/features/market/stores/market'
import {
  formatSignalValue,
  severityLabel,
  signalLabel,
  statusLabel,
  subtypeLabel,
  symbolDisplay,
} from '../labels'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const store = useMarketStore()

const enabled = computed(() => store.ingestHealth?.event === 'enabled')
const loading = ref(false)
const error = ref('')
const items = ref<MarketEventSummary[]>([])
const selected = ref<MarketEventDetail | null>(null)
const timeline = ref<EventTimelineItem[]>([])
const detailOpen = ref(false)
const activeCount = computed(
  () => items.value.filter((e) => e.status === 'ACTIVE' || e.status === 'DETECTED').length,
)

defineExpose({ enabled, activeCount })

let disconnect: (() => void) | null = null

async function loadList() {
  if (!enabled.value) {
    items.value = []
    return
  }
  loading.value = true
  error.value = ''
  try {
    const res = await fetchEvents({ limit: 20 })
    items.value = res.items
  } catch (e) {
    const err = e as Error & { code?: string }
    if (err.code === 'event_disabled') {
      error.value = ''
      items.value = []
    } else {
      error.value = err.message || '加载失败'
    }
  } finally {
    loading.value = false
  }
}

async function openDetail(id: string) {
  detailOpen.value = true
  selected.value = null
  timeline.value = []
  try {
    const [detail, tl] = await Promise.all([fetchEvent(id), fetchEventTimeline(id)])
    selected.value = detail
    timeline.value = tl.items
  } catch (e) {
    error.value = (e as Error).message || '详情加载失败'
  }
}

function upsertFromWs(partial: Partial<MarketEventSummary> & { id: string }) {
  const idx = items.value.findIndex((x) => x.id === partial.id)
  if (idx >= 0) {
    items.value[idx] = { ...items.value[idx], ...partial } as MarketEventSummary
  } else if (partial.title) {
    items.value = [partial as MarketEventSummary, ...items.value].slice(0, 40)
  }
}

function severityClass(sev: string) {
  const s = sev.toUpperCase()
  if (s === 'EXTREME' || s === 'HIGH') return 'sev-high'
  if (s === 'MEDIUM') return 'sev-mid'
  return 'sev-low'
}

function formatTime(ts: number) {
  if (!ts) return '—'
  const d = new Date(ts * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

watch(
  enabled,
  (on) => {
    disconnect?.()
    disconnect = null
    if (!on) {
      items.value = []
      return
    }
    void loadList()
    disconnect = connectEventsStream({
      onMessage: (msg) => {
        if (msg.type === 'pong') return
        if (!('event' in msg) || !msg.event?.id) return
        upsertFromWs({
          id: msg.event.id,
          type: msg.event.type,
          subType: msg.event.subType,
          title: msg.event.title,
          severity: msg.event.severity as MarketEventSummary['severity'],
          score: msg.event.score ?? 0,
          status: msg.event.status as MarketEventSummary['status'],
          symbols: msg.event.symbols ?? [],
          markets: msg.event.markets ?? [],
          startTime: msg.event.startTime ?? 0,
          endTime: msg.event.endTime ?? null,
          peakScore: msg.event.score ?? 0,
          createdAt: msg.event.startTime ?? 0,
          updatedAt: msg.event.updatedAt ?? Date.now() / 1000,
        })
        if (selected.value?.id === msg.event.id && detailOpen.value) {
          void openDetail(msg.event.id)
        }
      },
    })
  },
  { immediate: true },
)

watch(
  () => props.open,
  (on) => {
    if (on && enabled.value) void loadList()
    if (!on) detailOpen.value = false
  },
)

onUnmounted(() => {
  disconnect?.()
  disconnect = null
})
</script>

<template>
  <div v-if="enabled && open" class="event-drawer" role="dialog" aria-label="市场异动">
    <header class="event-drawer-head">
      <div>
        <h2>市场异动</h2>
        <p>加密异常事件 · 公开实时</p>
      </div>
      <div class="head-actions">
        <button type="button" class="ghost-btn" :disabled="loading" @click="loadList">刷新</button>
        <button type="button" class="icon-btn" aria-label="关闭市场异动" @click="emit('close')">×</button>
      </div>
    </header>

    <div class="event-drawer-body">
      <p v-if="error" class="err">{{ error }}</p>
      <p v-else-if="loading && !items.length" class="empty-state">加载中…</p>
      <p v-else-if="!items.length" class="empty-state">暂无异动事件</p>

      <ul v-else class="event-list">
        <li
          v-for="ev in items"
          :key="ev.id"
          class="event-row"
          @click="openDetail(ev.id)"
        >
          <div class="row-top">
            <span class="title">{{ ev.title }}</span>
            <span class="score" :class="severityClass(String(ev.severity))">
              {{ Math.round(ev.score) }}
            </span>
          </div>
          <div class="row-meta">
            <span class="status">{{ statusLabel(String(ev.status)) }}</span>
            <span class="sev-tag" :class="severityClass(String(ev.severity))">{{ severityLabel(String(ev.severity)) }}</span>
            <span class="symbols">{{ (ev.symbols || []).map(symbolDisplay).join(' · ') }}</span>
            <span class="time">{{ formatTime(ev.startTime) }}</span>
          </div>
        </li>
      </ul>
    </div>

    <div v-if="detailOpen" class="detail-mask" @click.self="detailOpen = false">
      <div class="detail-card">
        <header class="detail-head">
          <h3>{{ selected?.title || '事件详情' }}</h3>
          <button type="button" class="ghost-btn" @click="detailOpen = false">关闭</button>
        </header>
        <template v-if="selected">
          <p class="detail-sub">
            强度 {{ selected.score.toFixed(1) }}
            · {{ severityLabel(String(selected.severity)) }}
            · {{ statusLabel(String(selected.status)) }}
            <span v-if="selected.subType" class="muted"> · {{ subtypeLabel(String(selected.subType)) }}</span>
          </p>
          <p class="detail-desc">{{ selected.description }}</p>
          <h4>触发指标</h4>
          <ul class="sig-list">
            <li v-for="s in selected.signals" :key="s.id">
              <div class="sig-main">
                <strong>{{ signalLabel(s.signalType) }}</strong>
                <span class="sig-code">{{ s.signalType }}</span>
              </div>
              <div class="sig-meta">
                <span>{{ symbolDisplay(s.symbol) }}</span>
                <span v-if="formatSignalValue(s)" class="sig-val">{{ formatSignalValue(s) }}</span>
                <span v-if="s.window" class="muted">窗口 {{ s.window }}</span>
              </div>
            </li>
          </ul>
          <h4>时间线</h4>
          <ul class="tl-list">
            <li v-for="(t, i) in timeline" :key="i">
              <span class="time">{{ formatTime(t.ts) }}</span>
              {{ t.label }}
            </li>
          </ul>
        </template>
        <p v-else class="empty-state">加载中…</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.event-drawer {
  position: fixed;
  top: 16px;
  right: 62px;
  z-index: 51;
  width: min(420px, calc(100vw - 84px));
  max-height: min(72vh, 680px);
  display: flex;
  flex-direction: column;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--panel);
  box-shadow: 0 18px 40px var(--shadow);
  pointer-events: auto;
}

.event-drawer-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
  padding: 12px 12px 10px;
  border-bottom: 1px solid var(--line);
  flex-shrink: 0;
}

.event-drawer-head h2 {
  margin: 0 0 4px;
  font-size: 16px;
  color: var(--coin);
}

.event-drawer-head p {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
}

.head-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.icon-btn {
  width: 28px;
  height: 28px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--card);
  color: var(--muted);
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
}

.event-drawer-body {
  padding: 10px 12px 14px;
  overflow-y: auto;
  min-height: 120px;
}

.event-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.event-row {
  cursor: pointer;
  padding: 10px 8px;
  border-radius: 6px;
  border: 1px solid transparent;
}

.event-row:hover {
  border-color: var(--line);
  background: color-mix(in srgb, var(--card) 70%, transparent);
}

.row-top {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: baseline;
}

.title {
  font-size: 14px;
  font-weight: 600;
}

.score {
  font-variant-numeric: tabular-nums;
  font-weight: 700;
}

.sev-high { color: var(--up, #f6465d); }
.sev-mid { color: #f0b90b; }
.sev-low { color: var(--muted, #848e9c); }

.row-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 4px;
  font-size: 12px;
  color: var(--muted, #848e9c);
}

.err {
  color: var(--up, #f6465d);
  font-size: 12px;
}

.detail-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  z-index: 60;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding: 16px;
}

.detail-card {
  width: min(560px, 100%);
  max-height: 80vh;
  overflow: auto;
  background: var(--panel, #12161c);
  border: 1px solid var(--line, #2b3139);
  border-radius: 10px;
  padding: 14px 16px 18px;
}

.detail-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.detail-sub,
.detail-desc {
  font-size: 13px;
  color: var(--muted, #848e9c);
}

.sig-list,
.tl-list {
  list-style: none;
  padding: 0;
  margin: 0 0 12px;
  font-size: 13px;
}

.sig-list li,
.tl-list li {
  padding: 8px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.sig-main {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.sig-code {
  font-size: 11px;
  color: var(--muted, #848e9c);
  font-weight: 400;
}

.sig-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 4px;
  font-size: 12px;
  color: var(--text, #eaecef);
}

.sig-val {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}

.sev-tag {
  font-weight: 600;
}

.muted {
  color: var(--muted, #848e9c);
  margin-left: 0;
}

@media (max-width: 680px) {
  .event-drawer {
    inset: auto 10px 10px;
    right: 10px;
    top: auto;
    width: auto;
    max-height: 78vh;
  }

  .detail-mask {
    align-items: center;
  }
}
</style>
