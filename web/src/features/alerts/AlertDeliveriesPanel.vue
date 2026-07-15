<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useAuthStore } from '@/features/auth/stores/auth'
import * as alertsApi from './api'
import type { AlertDelivery } from './types'

const auth = useAuthStore()

const items = ref<AlertDelivery[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const loading = ref(false)
const error = ref('')

const channelLabel: Record<string, string> = {
  in_app: '站内',
  email: '邮件',
  pushplus: 'PushPlus',
}

onMounted(() => {
  void load()
})

async function load(p = page.value) {
  if (!auth.token) return
  loading.value = true
  error.value = ''
  try {
    const res = await alertsApi.listDeliveries(auth.token, { page: p, pageSize })
    items.value = res.items
    page.value = res.page
    total.value = res.total
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function formatTime(ts: number): string {
  return new Date(ts * 1000).toLocaleString()
}

const totalPages = () => Math.max(1, Math.ceil(total.value / pageSize))

async function prev() {
  if (page.value <= 1) return
  await load(page.value - 1)
}

async function next() {
  if (page.value >= totalPages()) return
  await load(page.value + 1)
}
</script>

<template>
  <section class="user-card">
    <div class="list-head">
      <h2>推送记录</h2>
      <button type="button" class="ghost-btn" :disabled="loading" @click="load()">刷新</button>
    </div>
    <p v-if="error" class="form-error">{{ error }}</p>
    <p v-else-if="loading && !items.length" class="hint">加载中…</p>
    <p v-else-if="!items.length" class="hint">暂无推送记录</p>
    <ul v-else class="del-list">
      <li v-for="d in items" :key="d.id" class="del-row">
        <div>
          <div class="del-title">
            <strong>{{ d.title || d.symbol }}</strong>
            <span class="pill" :class="d.status">{{ d.status }}</span>
          </div>
          <p class="del-body">{{ d.body }}</p>
          <p class="del-meta">
            #{{ d.id }} · 规则 {{ d.ruleId }} · {{ channelLabel[d.channel] || d.channel }} · 触发值
            {{ d.triggerValue }} · {{ formatTime(d.createdAt) }}
            <template v-if="d.errorMsg"> · {{ d.errorMsg }}</template>
          </p>
        </div>
      </li>
    </ul>
    <div v-if="total > pageSize" class="pager">
      <button type="button" class="ghost-btn" :disabled="page <= 1 || loading" @click="prev">上一页</button>
      <span>{{ page }} / {{ totalPages() }}（共 {{ total }}）</span>
      <button
        type="button"
        class="ghost-btn"
        :disabled="page >= totalPages() || loading"
        @click="next"
      >
        下一页
      </button>
    </div>
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

.list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}

.hint {
  margin: 0;
  font-size: 13px;
  color: var(--muted);
}

.form-error {
  margin: 0 0 8px;
  color: #e5484d;
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

.del-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 8px;
}

.del-row {
  padding: 10px 0;
  border-top: 1px solid var(--line);
}

.del-row:first-child {
  border-top: 0;
  padding-top: 0;
}

.del-title {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-strong);
}

.del-body {
  margin: 4px 0;
  font-size: 13px;
  color: var(--text);
}

.del-meta {
  margin: 0;
  font-size: 11px;
  color: var(--muted);
  line-height: 1.4;
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

.pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 12px;
  font-size: 12px;
  color: var(--muted);
}
</style>
