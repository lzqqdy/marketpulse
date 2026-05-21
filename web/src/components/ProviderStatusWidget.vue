<script setup lang="ts">
import { computed, ref } from 'vue'
import { useProviderStore } from '@/stores/providers'
import { useThemeStore } from '@/stores/theme'
import type { ProviderHealth, ProviderState } from '@/types/providers'

const store = useProviderStore()
const themeStore = useThemeStore()
const open = ref(false)

const statusText: Record<ProviderState, string> = {
  healthy: '正常',
  stale: '延迟',
  circuit_open: '熔断中',
  unavailable: '不可用',
  disabled: '已关闭',
  degraded: '降级',
}

const roleText: Record<string, string> = {
  primary: '主用',
  fallback: '备用',
  auxiliary: '辅助',
}

const summaryText = computed(() => {
  const overall = store.overall
  if (!overall) return store.loading ? '数据源：加载中' : '数据源：--'
  const abnormal = store.abnormalCount
  const suffix = abnormal > 0 ? ` · ${abnormal} 个异常` : ''
  return `数据源：${overall.healthy}/${overall.total} 正常${suffix}`
})

const updatedAtLabel = computed(() => {
  const ts = store.overall?.updated_at
  if (!ts) return '--'
  return new Date(ts * 1000).toLocaleTimeString('zh-CN', { hour12: false })
})

const overallStatusLabel = computed(() => {
  const state = store.overall?.status ?? 'unavailable'
  return statusText[state] ?? state
})

const themeLabel = computed(() => (themeStore.mode === 'dark' ? '切换浅色模式' : '切换深色模式'))

function latencyLabel(ms: number) {
  return ms > 0 ? `${ms}ms` : '-'
}

function agoLabel(seconds: number, fallbackTs: number) {
  if (!fallbackTs) return '-'
  const sec = seconds > 0 ? seconds : Math.max(0, Math.floor(Date.now() / 1000 - fallbackTs))
  if (sec < 60) return `${sec}秒前`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}分钟前`
  const hour = Math.floor(min / 60)
  return `${hour}小时前`
}

function usageLabel(row: ProviderHealth) {
  const role = roleText[row.role] ?? row.role
  if (row.current_used) return `${role} · 当前使用`
  return role || '-'
}

function togglePanel() {
  open.value = !open.value
  if (open.value) void store.refresh()
}
</script>

<template>
  <aside class="provider-dock" aria-label="页面工具">
    <div class="dock-rail">
      <button
        type="button"
        class="dock-btn provider-trigger"
        :class="[store.overall?.status ?? 'unavailable', { active: open }]"
        :aria-label="summaryText"
        @click="togglePanel"
      >
        <span class="chip-dot"></span>
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M4 7h11" />
          <path d="M4 12h16" />
          <path d="M4 17h11" />
          <path d="m17 7 3 3-3 3" />
        </svg>
      </button>
      <button type="button" class="dock-btn theme-trigger" :aria-label="themeLabel" @click="themeStore.toggle">
        <svg v-if="themeStore.mode === 'dark'" viewBox="0 0 24 24" aria-hidden="true">
          <circle cx="12" cy="12" r="4" />
          <path d="M12 2v2" />
          <path d="M12 20v2" />
          <path d="m4.93 4.93 1.41 1.41" />
          <path d="m17.66 17.66 1.41 1.41" />
          <path d="M2 12h2" />
          <path d="M20 12h2" />
          <path d="m6.34 17.66-1.41 1.41" />
          <path d="m19.07 4.93-1.41 1.41" />
        </svg>
        <svg v-else viewBox="0 0 24 24" aria-hidden="true">
          <path d="M20.4 14.4A7.6 7.6 0 0 1 9.6 3.6 8.4 8.4 0 1 0 20.4 14.4Z" />
        </svg>
      </button>
    </div>

    <div v-if="open" class="provider-panel">
      <header class="provider-panel-head">
        <div>
          <h2>数据源健康度</h2>
          <p>
            整体状态：{{ overallStatusLabel }}
            <span v-if="store.overall">
              · 平均延迟 {{ store.overall.avg_latency_ms }}ms · 最后更新 {{ updatedAtLabel }}
            </span>
          </p>
        </div>
        <button type="button" class="icon-btn" aria-label="关闭数据源状态" @click="open = false">
          ×
        </button>
      </header>

      <p v-if="store.lastError" class="provider-error">{{ store.lastError }}</p>

      <section v-for="group in store.groups" :key="group.category" class="provider-group">
        <h3>{{ group.label }}</h3>
        <div class="provider-table-scroll">
          <div class="provider-table">
            <div class="provider-row provider-row-head">
              <span>数据源</span>
              <span>状态</span>
              <span>延迟</span>
              <span>最近成功</span>
              <span>失败</span>
              <span>当前用途</span>
            </div>
            <div v-for="row in group.rows" :key="row.name" class="provider-row">
              <span class="provider-name">
                <strong>{{ row.label }}</strong>
                <small>{{ row.category }}</small>
              </span>
              <span class="state-pill" :class="row.status">{{ statusText[row.status] ?? row.status }}</span>
              <span>{{ latencyLabel(row.latency_ms) }}</span>
              <span>{{ agoLabel(row.stale_seconds, row.last_success_at) }}</span>
              <span>{{ row.fail_count }}</span>
              <span class="usage-text">{{ usageLabel(row) }}</span>
            </div>
          </div>
        </div>
      </section>
    </div>
  </aside>
</template>

<style scoped>
.provider-dock {
  position: fixed;
  top: 36vh;
  right: 0;
  z-index: 50;
  pointer-events: none;
}

.dock-rail {
  display: grid;
  width: 40px;
  border: 1px solid color-mix(in srgb, var(--line) 55%, transparent);
  border-right: 0;
  border-radius: 7px 0 0 7px;
  overflow: hidden;
  background: var(--dock-bg);
  box-shadow: 0 12px 28px var(--shadow);
  backdrop-filter: blur(8px);
  pointer-events: auto;
}

.dock-btn {
  position: relative;
  display: grid;
  place-items: center;
  width: 40px;
  height: 44px;
  border: 0;
  border-bottom: 1px solid color-mix(in srgb, var(--line) 55%, transparent);
  background: var(--dock-btn);
  color: var(--dock-icon);
  cursor: pointer;
}

.dock-btn:last-child {
  border-bottom: 0;
}

.dock-btn:hover,
.dock-btn.active {
  background: var(--dock-btn-active);
  color: var(--text-strong);
}

.dock-btn svg {
  width: 23px;
  height: 23px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2.2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.provider-trigger {
  align-items: center;
}

.chip-dot {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--muted);
}

.provider-trigger.healthy .chip-dot {
  background: var(--down);
}

.provider-trigger.degraded .chip-dot,
.provider-trigger.stale .chip-dot {
  background: var(--warning);
}

.provider-trigger.unavailable .chip-dot,
.provider-trigger.circuit_open .chip-dot {
  background: var(--up);
}

.provider-panel {
  position: fixed;
  top: 16px;
  right: 62px;
  width: min(720px, calc(100vw - 84px));
  max-height: min(72vh, 680px);
  overflow-y: auto;
  overflow-x: hidden;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--panel);
  box-shadow: 0 18px 40px var(--shadow);
  padding: 12px;
  text-align: left;
  pointer-events: auto;
}

.provider-panel-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--line);
}

.provider-panel-head h2 {
  margin: 0 0 4px;
  font-size: 16px;
}

.provider-panel-head p {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
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

.provider-error {
  color: var(--up);
  font-size: 12px;
}

.provider-group {
  margin-top: 12px;
}

.provider-group h3 {
  margin: 0 0 8px;
  font-size: 13px;
  color: var(--coin);
}

.provider-table-scroll {
  width: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: thin;
}

.provider-table {
  display: grid;
  gap: 1px;
  min-width: 640px;
  border: 1px solid var(--line);
  border-radius: 8px;
  overflow: hidden;
  background: var(--line);
}

.provider-row {
  display: grid;
  grid-template-columns: minmax(116px, 1.5fr) 74px 62px 84px 44px minmax(92px, 1fr);
  gap: 8px;
  align-items: center;
  min-height: 42px;
  background: var(--panel-soft);
  padding: 7px 9px;
  font-size: 12px;
  color: var(--text);
}

.provider-row-head {
  min-height: 34px;
  background: var(--card);
  color: var(--muted);
  font-size: 11px;
}

.provider-name {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.provider-name strong,
.usage-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-name small {
  color: var(--muted);
}

.state-pill {
  justify-self: start;
  border-radius: 5px;
  padding: 3px 6px;
  color: var(--bg);
  background: var(--muted);
  font-size: 11px;
}

.state-pill.healthy {
  background: var(--down);
}

.state-pill.stale,
.state-pill.degraded {
  background: var(--warning);
}

.state-pill.circuit_open,
.state-pill.unavailable {
  background: var(--up);
  color: #fff;
}

.state-pill.disabled {
  color: var(--text);
  background: var(--muted-2);
}

@media (max-width: 680px) {
  .provider-dock {
    top: auto;
    right: 0;
    bottom: 72px;
    transform: none;
  }

  .dock-rail,
  .dock-btn {
    width: 38px;
  }

  .dock-btn {
    height: 42px;
  }

  .provider-panel {
    position: fixed;
    inset: auto 10px 10px;
    width: auto;
    max-height: 78vh;
  }

  .provider-table {
    border-radius: 7px;
    scroll-snap-type: x proximity;
  }

  .provider-row {
    grid-template-columns: minmax(136px, 1.5fr) 74px 62px 84px 44px minmax(116px, 1fr);
    gap: 6px;
    min-width: 640px;
  }
}
</style>
