<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/features/auth/stores/auth'
import { useChartStore } from '@/features/market/stores/chart'
import { useMarketStore } from '@/features/market/stores/market'
import {
  AI_ASSISTANT_MARK,
  AI_ASSISTANT_NAME,
  AI_ASSISTANT_TAGLINE,
} from '../brand'
import { useAiChatStream } from '../composables/useAiChatStream'
import { renderAssistantHtml } from '../utils/renderMarkdown'

const QUICK_PROMPTS = [
  'BTC 现在怎么样？',
  '今天 A 股涨跌家数多少？',
  '最近有什么重要快讯？',
] as const

const open = ref(false)
const input = ref('')
const messagesEl = ref<HTMLElement | null>(null)
const composerEl = ref<HTMLTextAreaElement | null>(null)
const historyOpen = ref(false)
const copiedId = ref('')
const renaming = ref(false)
const renameDraft = ref('')
/** 手机 sheet 贴合 visualViewport（键盘） */
const sheetStyle = ref<Record<string, string>>({})
const isCompact = ref(false)

let lockedScrollY = 0
let bodyLocked = false

const auth = useAuthStore()
const router = useRouter()
const chart = useChartStore()
const market = useMarketStore()

/** 复用看板 healthz 轮询结果；未拉到前默认显示入口 */
const showFab = computed(() => market.ingestHealth?.ai !== 'disabled')

const composerPlaceholder = computed(() =>
  isCompact.value ? '问问行情…' : '问问行情… Enter 发送，Shift+Enter 换行',
)

function updateCompact() {
  isCompact.value = window.matchMedia('(max-width: 680px)').matches
}

function syncSheetToViewport() {
  if (!open.value || !isCompact.value) {
    sheetStyle.value = {}
    return
  }
  const vv = window.visualViewport
  if (!vv) {
    sheetStyle.value = {
      top: '0px',
      left: '0px',
      right: '0px',
      width: '100%',
      height: '100%',
      bottom: 'auto',
      borderRadius: '0',
    }
    return
  }
  // fixed 相对 layout viewport；用 offsetTop/height 对齐可见区（含键盘收起后的可视高度）
  sheetStyle.value = {
    top: `${vv.offsetTop}px`,
    left: `${vv.offsetLeft}px`,
    right: 'auto',
    bottom: 'auto',
    width: `${vv.width}px`,
    height: `${vv.height}px`,
    borderRadius: '0',
    maxHeight: 'none',
  }
}

function lockBodyScroll() {
  if (bodyLocked) return
  bodyLocked = true
  lockedScrollY = window.scrollY || document.documentElement.scrollTop || 0
  document.documentElement.style.overflow = 'hidden'
  document.body.style.overflow = 'hidden'
  document.body.style.position = 'fixed'
  document.body.style.top = `-${lockedScrollY}px`
  document.body.style.left = '0'
  document.body.style.right = '0'
  document.body.style.width = '100%'
}

function unlockBodyScroll() {
  if (!bodyLocked) return
  bodyLocked = false
  document.documentElement.style.overflow = ''
  document.body.style.overflow = ''
  document.body.style.position = ''
  document.body.style.top = ''
  document.body.style.left = ''
  document.body.style.right = ''
  document.body.style.width = ''
  window.scrollTo(0, lockedScrollY)
}

function onViewportChange() {
  updateCompact()
  syncSheetToViewport()
}

function bindViewport() {
  window.visualViewport?.addEventListener('resize', onViewportChange)
  window.visualViewport?.addEventListener('scroll', onViewportChange)
  window.addEventListener('resize', onViewportChange)
}

function unbindViewport() {
  window.visualViewport?.removeEventListener('resize', onViewportChange)
  window.visualViewport?.removeEventListener('scroll', onViewportChange)
  window.removeEventListener('resize', onViewportChange)
}

function onComposerFocus() {
  syncSheetToViewport()
  // 键盘弹起有动画，多拍几次对齐 visualViewport
  window.setTimeout(syncSheetToViewport, 80)
  window.setTimeout(syncSheetToViewport, 220)
  window.setTimeout(syncSheetToViewport, 400)
}

const {
  messages,
  conversationId,
  conversations,
  streaming,
  error,
  toolStatus,
  send,
  stop,
  reset,
  refreshConversations,
  loadConversation,
  removeConversation,
  renameConversation,
} = useAiChatStream()

const canUse = computed(() => auth.isLoggedIn)

const focusLabel = computed(() => {
  if (chart.visible) {
    if (chart.kind === 'crypto') return `${chart.displayName || chart.symbol} · 加密`
    if (chart.kind === 'alpha') {
      const name = chart.alphaQuote?.name || chart.displayName || chart.symbol
      return `${name} · 美股参考`
    }
    const name = chart.indexQuote?.name || chart.displayName || chart.symbol
    return `${name} · 指数`
  }
  const top = market.quotes[0]?.symbol
  return top ? `${top} · 看板` : '未选定标的'
})

const focusHint = computed(() => {
  if (chart.visible) return '将用于理解「这个 / 它」'
  return '未打开 K 线时默认取看板首个标的'
})

const knownSymbols = computed(() => {
  const set = new Set<string>()
  for (const q of market.quotes) {
    set.add(q.symbol)
    set.add(`${q.symbol}USDT`)
  }
  for (const s of ['BTC', 'ETH', 'BNB', 'SOL', 'XRP', 'DOGE', 'BTCUSDT', 'ETHUSDT']) {
    set.add(s)
  }
  return [...set]
})

const activeTitle = computed(() => {
  const cur = conversations.value.find((c) => c.conversationId === conversationId.value)
  return cur?.title || (conversationId.value ? '新对话' : '')
})

function buildContext() {
  const focus = chart.visible
    ? chart.kind === 'crypto'
      ? chart.symbol
      : chart.kind === 'alpha'
        ? chart.alphaQuote?.id || chart.symbol
        : chart.indexQuote?.id || chart.symbol
    : market.quotes[0]?.symbol
  return {
    focusSymbol: focus || undefined,
    assetClass: chart.visible ? chart.kind : 'crypto',
    page: 'dashboard',
    visibleSymbols: market.quotes.slice(0, 8).map((q) => q.symbol),
  }
}

function renderBody(content: string, role: string) {
  if (role !== 'assistant' || !content) return ''
  return renderAssistantHtml(content, knownSymbols.value)
}

async function scrollToBottom() {
  await nextTick()
  const el = messagesEl.value
  if (!el) return
  el.scrollTop = el.scrollHeight
}

watch(
  [messages, toolStatus, streaming],
  () => {
    void scrollToBottom()
  },
  { deep: true },
)

async function focusComposer() {
  await nextTick()
  composerEl.value?.focus()
}

async function toggle() {
  open.value = !open.value
  historyOpen.value = false
  renaming.value = false
  if (open.value && auth.token) {
    await refreshConversations(auth.token)
    if (conversationId.value && messages.value.length === 0) {
      try {
        await loadConversation(auth.token, conversationId.value)
      } catch {
        conversationId.value = ''
      }
    }
    await scrollToBottom()
    await focusComposer()
  }
}

function closeDrawer() {
  open.value = false
  historyOpen.value = false
  renaming.value = false
}

async function onSend(text?: string) {
  if (!auth.token) {
    await router.push({ name: 'login' })
    return
  }
  if (streaming.value) return
  const content = (text ?? input.value).trim()
  if (!content) return
  input.value = ''
  autosize()
  historyOpen.value = false
  await send(auth.token, content, buildContext())
  await focusComposer()
}

function onNewChat() {
  reset()
  historyOpen.value = false
  renaming.value = false
  void focusComposer()
}

async function onSelect(id: string) {
  if (!auth.token || streaming.value) return
  await loadConversation(auth.token, id)
  historyOpen.value = false
  renaming.value = false
  await scrollToBottom()
  await focusComposer()
}

async function onDelete(id: string) {
  if (!auth.token) return
  if (!window.confirm('确定删除该会话？删除后不可恢复。')) return
  await removeConversation(auth.token, id)
}

function onStop() {
  stop()
}

function onComposerKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    void onSend()
  }
}

function autosize() {
  const el = composerEl.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 120)}px`
}

function onBodyClick(e: MouseEvent) {
  const t = e.target as HTMLElement | null
  if (!t?.classList?.contains('sym-link')) return
  const sym = t.getAttribute('data-symbol')
  if (!sym) return
  const base = sym.replace(/USDT$/i, '')
  const hit = market.quotes.find((q) => q.symbol === sym || q.symbol === base)
  chart.open(hit?.symbol || base)
}

async function onCopy(content: string, id: string) {
  try {
    await navigator.clipboard.writeText(content)
    copiedId.value = id
    window.setTimeout(() => {
      if (copiedId.value === id) copiedId.value = ''
    }, 1500)
  } catch {
    /* ignore */
  }
}

function startRename() {
  if (!conversationId.value || !auth.token) return
  renameDraft.value = activeTitle.value === '新对话' ? '' : activeTitle.value
  renaming.value = true
}

async function commitRename() {
  if (!auth.token || !conversationId.value || !renaming.value) return
  const title = renameDraft.value.trim()
  renaming.value = false
  if (!title || title === activeTitle.value) return
  try {
    await renameConversation(auth.token, conversationId.value, title)
  } catch (e) {
    /* surface via alert; list stays */
    window.alert(e instanceof Error ? e.message : '改标题失败')
  }
}

function onGlobalKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && open.value) {
    if (historyOpen.value) {
      historyOpen.value = false
      return
    }
    if (renaming.value) {
      renaming.value = false
      return
    }
    closeDrawer()
  }
}

watch(open, async (v) => {
  updateCompact()
  if (v) {
    lockBodyScroll()
    bindViewport()
    syncSheetToViewport()
    await nextTick()
    syncSheetToViewport()
  } else {
    unbindViewport()
    sheetStyle.value = {}
    unlockBodyScroll()
  }
})

onMounted(() => {
  updateCompact()
  window.addEventListener('keydown', onGlobalKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onGlobalKeydown)
  unbindViewport()
  unlockBodyScroll()
})

watch(
  () => auth.isLoggedIn,
  (ok) => {
    if (!ok) reset()
  },
)
</script>

<template>
  <Teleport to="body">
    <button
      v-if="showFab && !open"
      type="button"
      class="ai-fab"
      :title="`${AI_ASSISTANT_NAME} · ${AI_ASSISTANT_TAGLINE}`"
      @click="toggle"
    >
      <img :src="AI_ASSISTANT_MARK" :alt="AI_ASSISTANT_NAME" width="48" height="48" />
    </button>

    <div
      v-if="open && showFab"
      class="ai-backdrop"
      :class="{ 'ai-backdrop--sheet': isCompact }"
      aria-hidden="true"
      @click="closeDrawer"
      @touchmove.prevent
    />

    <div
      v-if="open && showFab"
      class="ai-drawer"
      :class="{ 'ai-drawer--sheet': isCompact }"
      :style="sheetStyle"
      role="dialog"
      aria-modal="true"
      :aria-label="AI_ASSISTANT_NAME"
    >
      <header class="ai-head">
        <div class="brand">
          <img class="brand-mark" :src="AI_ASSISTANT_MARK" :alt="AI_ASSISTANT_NAME" width="28" height="28" />
          <div class="brand-text">
            <strong>{{ AI_ASSISTANT_NAME }}</strong>
            <span>{{ AI_ASSISTANT_TAGLINE }}</span>
          </div>
        </div>
        <div class="ai-actions">
          <button
            v-if="canUse"
            type="button"
            class="link mobile-only"
            @click="historyOpen = !historyOpen"
          >
            {{ historyOpen ? '对话' : '历史' }}
          </button>
          <button type="button" class="link" @click="onNewChat">新对话</button>
          <button type="button" class="link" @click="closeDrawer">关闭</button>
        </div>
      </header>

    <div v-if="canUse" class="focus-bar" :title="focusHint">
      <span class="focus-k">当前</span>
      <span class="focus-v">{{ focusLabel }}</span>
      <button
        v-if="conversationId && !renaming"
        type="button"
        class="link rename-btn"
        title="修改会话标题"
        @click="startRename"
      >
        改标题
      </button>
      <form
        v-else-if="renaming"
        class="rename-form"
        @submit.prevent="commitRename"
      >
        <input
          v-model="renameDraft"
          type="text"
          maxlength="64"
          placeholder="会话标题"
          @keydown.esc.prevent="renaming = false"
        />
        <button type="submit" class="link">保存</button>
      </form>
    </div>

    <p v-if="!canUse" class="ai-hint">
      请先
      <router-link to="/login">登录</router-link>
      后使用 {{ AI_ASSISTANT_NAME }}。
    </p>

    <div v-else class="ai-body">
      <aside class="ai-sessions desktop-sessions">
        <button
          v-for="c in conversations"
          :key="c.conversationId"
          type="button"
          class="session"
          :class="{ active: c.conversationId === conversationId }"
          @click="onSelect(c.conversationId)"
        >
          <span class="session-title">{{ c.title || '新对话' }}</span>
          <button
            type="button"
            class="session-del"
            title="删除"
            @click.stop="onDelete(c.conversationId)"
          >
            ×
          </button>
        </button>
        <p v-if="!conversations.length" class="empty">暂无历史会话</p>
      </aside>

      <div
        v-if="historyOpen"
        class="history-sheet"
        @click.self="historyOpen = false"
      >
        <div class="history-panel">
          <div class="history-head">
            <span>历史会话</span>
            <button type="button" class="link" @click="historyOpen = false">关闭</button>
          </div>
          <button
            v-for="c in conversations"
            :key="c.conversationId"
            type="button"
            class="session"
            :class="{ active: c.conversationId === conversationId }"
            @click="onSelect(c.conversationId)"
          >
            <span class="session-title">{{ c.title || '新对话' }}</span>
            <button
              type="button"
              class="session-del"
              title="删除"
              @click.stop="onDelete(c.conversationId)"
            >
              ×
            </button>
          </button>
          <p v-if="!conversations.length" class="empty">暂无历史会话</p>
        </div>
      </div>

      <div class="ai-main">
        <div ref="messagesEl" class="ai-messages" @click="onBodyClick">
          <div v-if="!messages.length" class="empty-state">
            <p class="empty-title">试试这样问</p>
            <div class="chips">
              <button
                v-for="q in QUICK_PROMPTS"
                :key="q"
                type="button"
                class="chip"
                :disabled="streaming"
                @click="onSend(q)"
              >
                {{ q }}
              </button>
            </div>
          </div>

          <div v-for="m in messages" :key="m.id" class="bubble" :class="m.role">
            <div class="bubble-meta">
              <span class="role">{{ m.role === 'user' ? '我' : '助手' }}</span>
              <button
                v-if="m.role === 'assistant' && m.content && !m.pending"
                type="button"
                class="copy-btn"
                @click="onCopy(m.content, m.id)"
              >
                {{ copiedId === m.id ? '已复制' : '复制' }}
              </button>
            </div>
            <div
              v-if="m.role === 'assistant'"
              class="body md"
              v-html="
                renderBody(m.content, m.role) ||
                (m.pending && !toolStatus ? '…' : '')
              "
            />
            <div v-else class="body">
              {{ m.content || (m.pending && !toolStatus ? '…' : '') }}
            </div>
          </div>

          <div
            v-if="toolStatus"
            class="tool-status"
            :class="toolStatus.phase"
          >
            <template v-if="toolStatus.phase === 'running'">查询中：{{ toolStatus.label }}</template>
            <template v-else-if="toolStatus.phase === 'done'">已引用：{{ toolStatus.label }}</template>
            <template v-else>查询失败：{{ toolStatus.label }}</template>
          </div>
        </div>

        <p v-if="error" class="ai-error">{{ error }}</p>

        <form class="ai-composer" @submit.prevent="onSend()">
          <textarea
            ref="composerEl"
            v-model="input"
            rows="1"
            :placeholder="composerPlaceholder"
            enterkeyhint="send"
            :disabled="streaming"
            @keydown="onComposerKeydown"
            @focus="onComposerFocus"
            @input="autosize"
          />
          <button
            v-if="streaming"
            type="button"
            class="btn-stop"
            @click="onStop"
          >
            停止
          </button>
          <button v-else type="submit" :disabled="!input.trim()">发送</button>
        </form>
      </div>
    </div>
  </div>
  </Teleport>
</template>

<style scoped>
.ai-fab {
  position: fixed;
  right: 18px;
  bottom: 22px;
  z-index: 1100;
  width: 48px;
  height: 48px;
  padding: 0;
  border-radius: 50%;
  border: 1px solid var(--border, #333);
  background: var(--panel, #1a1a1a);
  cursor: pointer;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.25);
  overflow: hidden;
}

.ai-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1090;
  border: none;
  padding: 0;
  margin: 0;
  background: rgba(8, 10, 14, 0.48);
  cursor: pointer;
  animation: ai-backdrop-in 0.18s ease-out;
  touch-action: none;
}

.ai-backdrop--sheet {
  background: rgba(8, 10, 14, 0.55);
}

/* 桌面端轻模糊；移动端只用遮罩，避免 backdrop-filter 掉帧 */
@media (min-width: 681px) {
  .ai-backdrop {
    background: rgba(8, 10, 14, 0.38);
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
  }
}

@keyframes ai-backdrop-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

.ai-fab img {
  display: block;
  width: 100%;
  height: 100%;
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.brand-mark {
  border-radius: 50%;
  border: 1px solid var(--border, #333);
  flex-shrink: 0;
}

.brand-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.2;
}

.brand-text strong {
  font-size: 14px;
}

.brand-text span {
  font-size: 11px;
  color: var(--muted, #999);
}

.ai-drawer {
  position: fixed;
  right: 16px;
  bottom: 80px;
  z-index: 1095;
  width: min(560px, calc(100vw - 24px));
  height: min(560px, calc(100vh - 120px));
  display: flex;
  flex-direction: column;
  background: var(--panel, #141414);
  border: 1px solid var(--border, #333);
  border-radius: 12px;
  overflow: hidden;
  overscroll-behavior: none;
}

.ai-drawer--sheet {
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  width: 100%;
  height: 100%;
  max-height: none;
  border-radius: 0;
  border: none;
  box-shadow: none;
}

.ai-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border, #333);
}

.ai-actions {
  display: flex;
  gap: 10px;
}

.link {
  background: none;
  border: none;
  color: var(--muted, #999);
  cursor: pointer;
  font-size: 12px;
}

.focus-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  font-size: 12px;
  border-bottom: 1px solid var(--border, #333);
  background: rgba(240, 185, 11, 0.06);
  flex-wrap: wrap;
}

.focus-k {
  color: var(--muted, #999);
}

.focus-v {
  color: var(--coin, #f0b90b);
  font-weight: 600;
  flex: 1;
  min-width: 0;
}

.rename-btn {
  margin-left: auto;
}

.rename-form {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-left: auto;
}

.rename-form input {
  width: 120px;
  padding: 2px 6px;
  border-radius: 4px;
  border: 1px solid var(--border, #333);
  background: transparent;
  color: inherit;
  font-size: 12px;
}

.ai-hint,
.ai-error,
.empty {
  margin: 8px 12px;
  font-size: 12px;
  color: var(--muted, #999);
}

.ai-error {
  color: var(--up, #e74c3c);
}

.ai-body {
  flex: 1;
  min-height: 0;
  display: flex;
  position: relative;
}

.ai-sessions {
  width: 140px;
  border-right: 1px solid var(--border, #333);
  overflow: auto;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.session {
  display: flex;
  align-items: center;
  gap: 4px;
  text-align: left;
  padding: 6px 8px;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font-size: 12px;
  width: 100%;
}

.session.active {
  background: rgba(240, 185, 11, 0.12);
}

.session-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-del {
  border: none;
  background: none;
  color: var(--muted, #888);
  cursor: pointer;
  padding: 0 2px;
}

.history-sheet {
  display: none;
}

.ai-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.ai-messages {
  flex: 1;
  overflow: auto;
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  overscroll-behavior: contain;
  -webkit-overflow-scrolling: touch;
  touch-action: pan-y;
}

.empty-state {
  margin: auto 0;
  padding: 12px 4px;
}

.empty-title {
  margin: 0 0 10px;
  font-size: 12px;
  color: var(--muted, #999);
}

.chips {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.chip {
  text-align: left;
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid var(--border, #333);
  background: rgba(255, 255, 255, 0.04);
  color: inherit;
  font-size: 13px;
  cursor: pointer;
}

.chip:hover:not(:disabled) {
  border-color: var(--coin, #f0b90b);
}

.chip:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.tool-status {
  align-self: flex-start;
  font-size: 12px;
  padding: 4px 8px;
  border-radius: 6px;
  color: var(--muted, #999);
  background: rgba(255, 255, 255, 0.05);
}

.tool-status.running {
  color: var(--coin, #f0b90b);
}

.tool-status.done {
  color: var(--muted, #aaa);
}

.tool-status.error {
  color: var(--up, #e74c3c);
}

.bubble {
  max-width: 92%;
  font-size: 13px;
  line-height: 1.45;
}

.bubble.user {
  align-self: flex-end;
}

.bubble.assistant {
  align-self: flex-start;
}

.bubble-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 2px;
}

.role {
  font-size: 11px;
  color: var(--muted, #888);
}

.copy-btn {
  font-size: 11px;
  background: none;
  border: none;
  color: var(--muted, #888);
  cursor: pointer;
  padding: 0;
}

.copy-btn:hover {
  color: var(--coin, #f0b90b);
}

.body {
  padding: 8px 10px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.06);
  white-space: pre-wrap;
  word-break: break-word;
  min-height: 1em;
}

.body.md {
  white-space: normal;
}

.body.md :deep(p) {
  margin: 0 0 0.55em;
}

.body.md :deep(p:last-child) {
  margin-bottom: 0;
}

.body.md :deep(ul),
.body.md :deep(ol) {
  margin: 0.35em 0;
  padding-left: 1.25em;
}

.body.md :deep(li) {
  margin: 0.15em 0;
}

.body.md :deep(code) {
  font-size: 0.92em;
  padding: 0.1em 0.35em;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.25);
}

.body.md :deep(.sym-link) {
  display: inline;
  padding: 0;
  margin: 0;
  border: none;
  background: none;
  color: var(--coin, #f0b90b);
  font: inherit;
  font-weight: 600;
  text-decoration: underline;
  text-underline-offset: 2px;
  cursor: pointer;
}

.bubble.user .body {
  background: rgba(240, 185, 11, 0.15);
}

.ai-composer {
  display: flex;
  gap: 8px;
  padding: 10px;
  padding-bottom: max(10px, env(safe-area-inset-bottom, 0px));
  border-top: 1px solid var(--border, #333);
  align-items: flex-end;
  flex-shrink: 0;
  background: var(--panel, #141414);
}

.ai-composer textarea {
  flex: 1;
  min-width: 0;
  min-height: 38px;
  max-height: 120px;
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid var(--border, #333);
  background: transparent;
  color: inherit;
  resize: none;
  font: inherit;
  line-height: 1.4;
  /* iOS 聚焦时减少页面被浏览器强行滚动 */
  font-size: 16px;
}

.ai-composer button {
  padding: 8px 12px;
  border-radius: 8px;
  border: none;
  background: var(--coin, #f0b90b);
  color: #111;
  font-weight: 600;
  cursor: pointer;
  flex-shrink: 0;
}

.ai-composer button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-stop {
  background: transparent !important;
  color: var(--up, #e74c3c) !important;
  border: 1px solid var(--up, #e74c3c) !important;
}

.mobile-only {
  display: none;
}

/* 与 ProviderStatusWidget 移动端断点对齐：避开右下角 dock */
@media (max-width: 680px) {
  .ai-fab {
    /* dock 宽 40px，贴右；FAB 放左侧留 10px 间距 */
    right: 50px;
    bottom: max(72px, calc(56px + env(safe-area-inset-bottom, 0px)));
  }

  .ai-drawer:not(.ai-drawer--sheet) {
    right: 8px;
    left: 8px;
    width: auto;
    bottom: max(132px, calc(116px + env(safe-area-inset-bottom, 0px)));
    height: min(560px, calc(100dvh - 160px));
  }

  .ai-head {
    padding-top: max(10px, env(safe-area-inset-top, 0px));
  }
}

@media (max-width: 520px) {
  .desktop-sessions {
    display: none;
  }

  .mobile-only {
    display: inline;
  }

  .history-sheet {
    display: block;
    position: absolute;
    inset: 0;
    z-index: 5;
    background: rgba(0, 0, 0, 0.45);
  }

  .history-panel {
    position: absolute;
    inset: 0 auto 0 0;
    width: min(260px, 78vw);
    background: var(--panel, #141414);
    border-right: 1px solid var(--border, #333);
    padding: 8px;
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .history-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 4px 4px 8px;
    font-size: 12px;
    color: var(--muted, #999);
  }
}
</style>
