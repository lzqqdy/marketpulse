<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useAuthStore } from '@/features/auth/stores/auth'
import * as alertsApi from './api'
import type { AlertInboxItem } from './types'
import { connectAlertsStream } from './useAlertsStream'

interface ToastItem extends AlertInboxItem {
  key: string
}

const auth = useAuthStore()
const toasts = ref<ToastItem[]>([])
let disconnect: (() => void) | null = null
let seq = 0

function pushToast(item: AlertInboxItem) {
  const key = `${item.deliveryId}-${++seq}`
  toasts.value = [{ ...item, key }, ...toasts.value].slice(0, 5)
  window.setTimeout(() => {
    dismiss(key, false)
  }, 12_000)
}

async function dismiss(key: string, ack: boolean) {
  const idx = toasts.value.findIndex((t) => t.key === key)
  if (idx < 0) return
  const [removed] = toasts.value.splice(idx, 1)
  if (ack && auth.token && removed.deliveryId > 0) {
    try {
      await alertsApi.ackInbox(auth.token, [removed.deliveryId])
    } catch {
      /* ignore */
    }
  }
}

function connect(token: string) {
  disconnect?.()
  disconnect = connectAlertsStream(token, {
    onAlert: pushToast,
  })
}

watch(
  () => auth.token,
  (token) => {
    disconnect?.()
    disconnect = null
    toasts.value = []
    if (token) connect(token)
  },
  { immediate: true },
)

onUnmounted(() => {
  disconnect?.()
  disconnect = null
})
</script>

<template>
  <div v-if="toasts.length" class="toast-host" aria-live="polite">
    <div v-for="t in toasts" :key="t.key" class="toast" role="status">
      <div class="toast-head">
        <strong>{{ t.title || t.symbol }}</strong>
        <button type="button" class="toast-x" aria-label="关闭" @click="dismiss(t.key, true)">×</button>
      </div>
      <p>{{ t.body }}</p>
      <span class="toast-meta">{{ t.symbol }}</span>
    </div>
  </div>
</template>

<style scoped>
.toast-host {
  position: fixed;
  right: 16px;
  bottom: 16px;
  z-index: 80;
  display: flex;
  flex-direction: column-reverse;
  gap: 8px;
  max-width: min(360px, calc(100vw - 32px));
  pointer-events: none;
}

.toast {
  pointer-events: auto;
  background: var(--card);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 12px 14px;
  box-shadow: 0 8px 24px var(--shadow);
  animation: toast-in 0.28s ease;
}

.toast-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  color: var(--text-strong);
  font-size: 13px;
}

.toast p {
  margin: 6px 0 4px;
  font-size: 12px;
  color: var(--text);
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.toast-meta {
  font-size: 11px;
  color: var(--muted);
}

.toast-x {
  border: 0;
  background: transparent;
  color: var(--muted);
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
  padding: 0 2px;
  min-width: 28px;
  min-height: 28px;
}

.toast-x:active {
  color: var(--text);
}

@keyframes toast-in {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 680px) {
  .toast-host {
    top: max(12px, env(safe-area-inset-top, 0px));
    bottom: auto;
    right: 12px;
    left: 12px;
    max-width: none;
    flex-direction: column;
  }

  @keyframes toast-in {
    from {
      opacity: 0;
      transform: translateY(-8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
}
</style>
