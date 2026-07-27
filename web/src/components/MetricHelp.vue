<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

defineProps<{
  title: string
  text: string
  docUrl?: string
  docLabel?: string
}>()

const open = ref(false)

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

watch(open, (v) => {
  document.body.style.overflow = v ? 'hidden' : ''
})

onMounted(() => {
  window.addEventListener('keydown', onKey)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  document.body.style.overflow = ''
})
</script>

<template>
  <span class="metric-help">
    <button
      type="button"
      class="metric-help-btn"
      :aria-label="`${title}说明`"
      :aria-expanded="open"
      @click.stop.prevent="toggle"
    >
      ?
    </button>
    <Teleport to="body">
      <div v-if="open" class="metric-help-layer" @click="close">
        <div class="metric-help-panel" role="dialog" :aria-label="title" @click.stop>
          <header class="metric-help-head">
            <h3>{{ title }}</h3>
            <button type="button" class="metric-help-close" aria-label="关闭" @click="close">×</button>
          </header>
          <p class="metric-help-text">{{ text }}</p>
          <a
            v-if="docUrl"
            class="metric-help-link"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            @click.stop
          >
            {{ docLabel || '查看文档' }}
          </a>
          <button type="button" class="metric-help-ok" @click="close">知道了</button>
        </div>
      </div>
    </Teleport>
  </span>
</template>

<style scoped>
.metric-help {
  display: inline-flex;
  align-items: center;
  vertical-align: middle;
}

.metric-help-btn {
  display: inline-grid;
  place-items: center;
  width: 14px;
  height: 14px;
  margin-left: 3px;
  padding: 0;
  border: 1px solid color-mix(in srgb, var(--muted) 55%, var(--line));
  border-radius: 50%;
  background: transparent;
  color: var(--muted);
  font-size: 10px;
  font-weight: 700;
  line-height: 1;
  cursor: pointer;
  flex-shrink: 0;
}

.metric-help-btn:hover,
.metric-help-btn:focus-visible {
  color: var(--coin);
  border-color: color-mix(in srgb, var(--coin) 55%, var(--line));
}

.metric-help-layer {
  position: fixed;
  inset: 0;
  z-index: 2400;
  display: grid;
  place-items: center;
  padding: 20px 16px;
  background: rgba(8, 10, 14, 0.5);
  box-sizing: border-box;
}

.metric-help-panel {
  width: min(360px, 100%);
  display: grid;
  gap: 12px;
  padding: 14px 14px 12px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--panel, var(--card));
  box-shadow: 0 16px 40px var(--shadow, rgba(0, 0, 0, 0.35));
}

.metric-help-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.metric-help-head h3 {
  margin: 0;
  font-size: 15px;
  color: var(--coin, var(--text-strong));
}

.metric-help-close {
  border: 0;
  background: transparent;
  color: var(--muted);
  font-size: 20px;
  line-height: 1;
  cursor: pointer;
  padding: 0 2px;
}

.metric-help-text {
  margin: 0;
  font-size: 13px;
  line-height: 1.55;
  color: var(--text);
  white-space: pre-wrap;
}

.metric-help-link {
  font-size: 13px;
  color: var(--coin);
  word-break: break-all;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.metric-help-link:hover {
  filter: brightness(1.08);
}

.metric-help-ok {
  justify-self: end;
  border: 0;
  border-radius: 6px;
  background: var(--coin);
  color: #111;
  font-weight: 700;
  font-size: 13px;
  padding: 8px 14px;
  cursor: pointer;
}

@media (max-width: 680px) {
  .metric-help-btn {
    width: 16px;
    height: 16px;
    font-size: 11px;
  }

  .metric-help-layer {
    align-items: end;
    padding: 12px 12px max(12px, env(safe-area-inset-bottom, 0px));
  }

  .metric-help-panel {
    width: 100%;
    border-radius: 12px 12px 10px 10px;
  }

  .metric-help-ok {
    width: 100%;
    min-height: 44px;
    justify-self: stretch;
  }
}
</style>
