<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { FileText, ExternalLink, ListChecks } from '@lucide/vue'
import type { ChatMessage } from '../types'
import { PHASE_LABELS } from '../utils/toolProgress'
import { CHANGE_BASIS, TIME_BASIS, FACT_LABELS, KIND_LABELS, currentEvidence, evidenceTime, factValue, safeEvidenceUrl } from '../utils/evidencePresentation'

const props = defineProps<{ message: ChatMessage; selectedId?: string }>()
const now = ref(Date.now())
const records = computed(() => (props.message.evidence ?? []).map((item) => currentEvidence(item, now.value)))
const statusLabel = { ok: '可用', partial: '部分可用', stale: '历史参考', unavailable: '不可用' }
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => { timer = setInterval(() => { now.value = Date.now() }, 30_000) })
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>

<template>
  <section v-if="message.toolSteps?.length || records.length || message.interrupted" class="assistant-trace">
    <details v-if="message.toolSteps?.length" class="tool-trace" :open="message.pending || undefined">
      <summary><ListChecks :size="14" />查询记录 · {{ message.toolSteps.length }}</summary>
      <ol>
        <li v-for="step in message.toolSteps" :key="step.id" :class="step.phase">
          <span>{{ step.label }}</span>
          <span class="step-state">{{ PHASE_LABELS[step.phase] }}<small v-if="step.durationMs !== undefined"> · {{ (step.durationMs / 1000).toFixed(1) }}s</small></span>
          <small v-if="step.note" class="tool-note">{{ step.note }}</small>
        </li>
      </ol>
    </details>
    <p v-if="message.interrupted" class="incomplete">回答未完成，已保留生成内容</p>
    <details v-if="records.length" class="evidence-list" :open="!!selectedId || undefined">
      <summary><FileText :size="14" />查询依据 · {{ records.length }}</summary>
      <details v-for="(record, index) in records" :key="record.id" class="evidence-record" :class="record.status" :open="selectedId === record.id || undefined">
        <summary><span>[{{ index + 1 }}] {{ record.symbol || record.facts.title || KIND_LABELS[record.kind] || record.kind }} · {{ record.source || '来源未知' }}</span><span class="record-status">{{ statusLabel[record.status] }}</span></summary>
        <dl class="evidence-meta">
          <dt>{{ TIME_BASIS[record.asOfBasis || 'unknown'] || '数据时点' }}</dt><dd>{{ evidenceTime(record.asOf) }}</dd>
          <dt>采集时间</dt><dd>{{ evidenceTime(record.fetchedAt) }}</dd>
          <template v-if="record.instrumentType"><dt>工具类型</dt><dd>{{ record.instrumentType }}</dd></template>
          <template v-if="record.currency"><dt>货币</dt><dd>{{ record.currency }}</dd></template>
          <template v-if="record.changeBasis"><dt>变化口径</dt><dd>{{ CHANGE_BASIS[record.changeBasis] || record.changeBasis }}</dd></template>
        </dl>
        <dl class="evidence-facts">
          <template v-for="(value, key) in record.facts" :key="key"><dt>{{ FACT_LABELS[key] || key }}</dt><dd>{{ factValue(value) }}</dd></template>
        </dl>
        <ul v-if="record.limitations?.length" class="limitations"><li v-for="note in record.limitations" :key="note">{{ note }}</li></ul>
        <a v-if="safeEvidenceUrl(record.url)" :href="safeEvidenceUrl(record.url)" target="_blank" rel="noopener noreferrer" class="source-link"><ExternalLink :size="13" />原始来源</a>
      </details>
    </details>
  </section>
</template>

<style scoped>
.assistant-trace { margin-top: 6px; font-size: 12px; color: var(--muted, #999); min-width: 0; }
summary { cursor: pointer; padding: 6px 0; overflow-wrap: anywhere; }
.tool-trace > summary, .evidence-list > summary { display: flex; align-items: center; gap: 5px; }
ol { list-style: none; margin: 2px 0 8px; padding: 0; }
ol li { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 3px 12px; padding: 4px 0; }
.tool-note { grid-column: 1 / -1; overflow-wrap: anywhere; color: var(--muted, #999); }
.step-state { white-space: nowrap; }
.running, .partial, .stale, .interrupted, .incomplete { color: var(--warning, #d7ab3f); }
.error, .unavailable { color: var(--up, #e74c3c); }
.evidence-record { border-top: 1px solid var(--border, #333); padding: 2px 0; }
.evidence-record > summary { display: flex; gap: 8px; justify-content: space-between; }
.record-status { flex-shrink: 0; font-size: 11px; }
.evidence-meta, .evidence-facts { display: grid; grid-template-columns: minmax(72px, 30%) minmax(0, 1fr); gap: 5px 10px; margin: 8px 0; color: var(--text, #ddd); }
dt { color: var(--muted, #999); overflow-wrap: anywhere; }
dd { margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; max-height: 220px; overflow: auto; }
.limitations { padding-left: 16px; line-height: 1.6; color: var(--warning, #d7ab3f); }
.source-link { display: inline-flex; gap: 5px; align-items: center; color: var(--coin, #d7ab3f); padding: 4px 0; }
.incomplete { margin: 5px 0; }
</style>
