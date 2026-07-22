<script setup lang="ts">
import { computed } from 'vue'
import type { AllocationItem } from './types'

const props = defineProps<{
  items: AllocationItem[]
  totalCny: number
}>()

const COLORS = [
  '#f0b90b',
  '#f6465d',
  '#0ecb81',
  '#3b82f6',
  '#a78bfa',
  '#f97316',
  '#22d3ee',
  '#e879f9',
  '#84cc16',
  '#fb7185',
]

const slices = computed(() => {
  const total = props.items.reduce((s, i) => s + i.valueCny, 0)
  if (total <= 0) return []
  let angle = -Math.PI / 2
  const r = 42
  const cx = 50
  const cy = 50
  return props.items.map((item, idx) => {
    const frac = item.valueCny / total
    const sweep = frac * Math.PI * 2
    const x1 = cx + r * Math.cos(angle)
    const y1 = cy + r * Math.sin(angle)
    angle += sweep
    const x2 = cx + r * Math.cos(angle)
    const y2 = cy + r * Math.sin(angle)
    const large = sweep > Math.PI ? 1 : 0
    const d =
      frac >= 0.999
        ? `M ${cx} ${cy - r} A ${r} ${r} 0 1 1 ${cx - 0.01} ${cy - r} Z`
        : `M ${cx} ${cy} L ${x1} ${y1} A ${r} ${r} 0 ${large} 1 ${x2} ${y2} Z`
    return {
      ...item,
      color: COLORS[idx % COLORS.length],
      d,
    }
  })
})

function fmtMoney(n: number): string {
  return n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}
</script>

<template>
  <div class="donut-wrap">
    <div v-if="!slices.length" class="empty">暂无持仓分布</div>
    <template v-else>
      <svg class="donut" viewBox="0 0 100 100" aria-label="资产分布">
        <circle cx="50" cy="50" r="42" fill="transparent" stroke="var(--line)" stroke-width="0.5" />
        <path v-for="s in slices" :key="s.symbol + s.assetType" :d="s.d" :fill="s.color" opacity="0.92" />
        <circle cx="50" cy="50" r="24" fill="var(--panel)" />
        <text x="50" y="48" text-anchor="middle" class="center-label">总资产</text>
        <text x="50" y="56" text-anchor="middle" class="center-value">¥{{ fmtMoney(totalCny) }}</text>
      </svg>
      <ul class="legend">
        <li v-for="s in slices" :key="s.symbol + s.assetType">
          <span class="dot" :style="{ background: s.color }" />
          <span class="sym">{{ s.symbol.toUpperCase() }}</span>
          <span class="pct">{{ s.weightPct.toFixed(2) }}%</span>
          <span class="amt">¥{{ fmtMoney(s.valueCny) }}</span>
        </li>
      </ul>
    </template>
  </div>
</template>

<style scoped>
.donut-wrap {
  display: grid;
  grid-template-columns: minmax(160px, 220px) 1fr;
  gap: 16px;
  align-items: center;
  min-width: 0;
}

.donut {
  width: 100%;
  max-width: 220px;
  aspect-ratio: 1;
}

.center-label {
  fill: var(--muted);
  font-size: 4px;
}

.center-value {
  fill: var(--text);
  font-size: 4.2px;
  font-weight: 600;
}

.legend {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 8px;
  min-width: 0;
}

.legend li {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr) auto auto;
  gap: 8px;
  align-items: center;
  font-size: 12px;
  color: var(--text);
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}

.sym {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
}

.pct,
.amt {
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.empty {
  color: var(--muted);
  font-size: 13px;
  grid-column: 1 / -1;
}

@media (max-width: 680px) {
  .donut-wrap {
    grid-template-columns: 1fr;
    justify-items: center;
  }

  .legend {
    width: 100%;
  }
}
</style>
