<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useAuthStore } from '@/features/auth/stores/auth'
import * as api from './api'
import type { EligibleSymbol, HoldingInput, HoldingView, PortfolioAssetType } from './types'

const emit = defineEmits<{ saved: [] }>()

const auth = useAuthStore()
const rows = ref<HoldingView[]>([])
const draftQty = reactive<Record<string, string>>({})
const principal = ref('0')
const eligible = ref<{ crypto: EligibleSymbol[]; alpha: EligibleSymbol[] }>({ crypto: [], alpha: [] })
const addType = ref<PortfolioAssetType>('crypto')
const addSymbol = ref('')
const addQty = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const msg = ref('')

const addOptions = computed(() => (addType.value === 'crypto' ? eligible.value.crypto : eligible.value.alpha))

function rowKey(h: { assetType: string; symbol: string }) {
  return `${h.assetType}:${h.symbol}`
}

function displaySymbol(h: HoldingView) {
  if (h.assetType === 'crypto') {
    return h.symbol === 'USDT' ? 'USDT' : h.symbol
  }
  const hit = eligible.value.alpha.find((a) => a.symbol === h.symbol)
  return hit?.name || h.symbol.toUpperCase()
}

function fmtMoney(n: number, prefix = ''): string {
  if (!Number.isFinite(n)) return '—'
  const body = n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
  return `${prefix}${body}`
}

function fmtSigned(n: number): string {
  if (!Number.isFinite(n)) return '—'
  const body = Math.abs(n).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
  if (n > 0) return `+${body}`
  if (n < 0) return `-${body}`
  return body
}

function trendClass(n: number) {
  if (n > 0) return 'up'
  if (n < 0) return 'down'
  return 'flat'
}

function syncDraft(list: HoldingView[]) {
  for (const key of Object.keys(draftQty)) delete draftQty[key]
  for (const h of list) {
    draftQty[rowKey(h)] = String(h.quantity)
  }
}

async function load() {
  if (!auth.token) return
  loading.value = true
  error.value = ''
  try {
    const [holdings, symbols] = await Promise.all([
      api.getHoldings(auth.token),
      api.getEligibleSymbols(auth.token).catch(() => ({ crypto: [], alpha: [] })),
    ])
    rows.value = holdings.holdings ?? []
    principal.value = String(holdings.principalCny ?? 0)
    eligible.value = symbols
    syncDraft(rows.value)
    if (!addSymbol.value && addOptions.value[0]) {
      addSymbol.value = addOptions.value[0].symbol
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function collectInputs(): HoldingInput[] {
  const out: HoldingInput[] = []
  for (const h of rows.value) {
    const raw = draftQty[rowKey(h)]
    const qty = Number(raw)
    if (!Number.isFinite(qty) || qty < 0) {
      throw new Error(`${h.symbol} 数量无效`)
    }
    if (qty === 0) continue
    out.push({ assetType: h.assetType, symbol: h.symbol, quantity: qty })
  }
  return out
}

async function saveHoldings() {
  if (!auth.token) return
  saving.value = true
  error.value = ''
  msg.value = ''
  try {
    const holdings = collectInputs()
    const res = await api.putHoldings(auth.token, holdings)
    rows.value = res.holdings ?? []
    syncDraft(rows.value)
    msg.value = '持仓已保存'
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败'
  } finally {
    saving.value = false
  }
}

async function savePrincipal() {
  if (!auth.token) return
  const n = Number(principal.value)
  if (!Number.isFinite(n) || n < 0) {
    error.value = '本金须为 ≥ 0 的数字'
    return
  }
  saving.value = true
  error.value = ''
  msg.value = ''
  try {
    const res = await api.putSettings(auth.token, n)
    principal.value = String(res.principalCny)
    msg.value = '本金已保存'
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败'
  } finally {
    saving.value = false
  }
}

function addRow() {
  const symbol = addSymbol.value.trim()
  const qty = Number(addQty.value)
  if (!symbol) {
    error.value = '请选择标的'
    return
  }
  if (!Number.isFinite(qty) || qty <= 0) {
    error.value = '请填写有效数量'
    return
  }
  const key = `${addType.value}:${addType.value === 'crypto' ? symbol.toUpperCase() : symbol.toLowerCase()}`
  const exists = rows.value.some((h) => rowKey(h) === key || rowKey(h).toLowerCase() === key.toLowerCase())
  if (exists) {
    error.value = '该标的已在持仓中，请直接改数量'
    return
  }
  const sym = addType.value === 'crypto' ? symbol.toUpperCase().replace(/USDT$/i, '') || 'USDT' : symbol.toLowerCase()
  const finalSym = addType.value === 'crypto' && symbol.toUpperCase() === 'USDT' ? 'USDT' : sym
  const view: HoldingView = {
    assetType: addType.value,
    symbol: finalSym,
    quantity: qty,
    priceUsdt: 0,
    valueUsdt: 0,
    valueCny: 0,
    changeCny: 0,
    missing: true,
  }
  rows.value = [...rows.value, view]
  draftQty[rowKey(view)] = String(qty)
  addQty.value = ''
  error.value = ''
}

function removeRow(h: HoldingView) {
  rows.value = rows.value.filter((x) => rowKey(x) !== rowKey(h))
  delete draftQty[rowKey(h)]
}

watch(addType, () => {
  addSymbol.value = addOptions.value[0]?.symbol ?? ''
})

onMounted(load)

defineExpose({ reload: load })
</script>

<template>
  <section class="holdings-panel">
    <div class="toolbar">
      <h2>持仓设置</h2>
      <div class="principal">
        <label>
          本金 (¥)
          <input v-model="principal" type="number" min="0" step="0.01" inputmode="decimal" />
        </label>
        <button type="button" class="ghost-btn" :disabled="saving || loading" @click="savePrincipal">保存本金</button>
      </div>
    </div>

    <div class="table-wrap desktop-only">
      <table class="holdings-table">
        <thead>
          <tr>
            <th class="col-coin">币种</th>
            <th class="col-num">数量</th>
            <th class="col-val num">价值(U)</th>
            <th class="col-est num">
              估值(¥)
              <span class="th-sub">日涨跌估算</span>
            </th>
            <th class="col-act"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && !rows.length">
            <td colspan="5" class="empty">加载中…</td>
          </tr>
          <tr v-else-if="!rows.length">
            <td colspan="5" class="empty">暂无持仓，请在下方添加</td>
          </tr>
          <tr v-for="h in rows" :key="rowKey(h)">
            <td class="coin col-coin">
              <div class="coin-cell">
                <span class="coin-name">{{ displaySymbol(h) }}</span>
                <span v-if="h.assetType === 'alpha'" class="tag">美股</span>
              </div>
            </td>
            <td class="col-num">
              <input
                v-model="draftQty[rowKey(h)]"
                class="qty"
                type="number"
                min="0"
                step="any"
                @keydown.enter.prevent="saveHoldings"
              />
            </td>
            <td class="num col-val">{{ h.missing ? '—' : fmtMoney(h.valueUsdt, '$') }}</td>
            <td class="num estimated col-est">
              <div>{{ h.missing ? '—' : fmtMoney(h.valueCny, '¥') }}</div>
              <div
                class="chg"
                :class="trendClass(h.changeCny)"
                title="按标的日涨跌幅估算，非相对昨日快照"
              >
                {{ h.missing ? '' : fmtSigned(h.changeCny) }}
              </div>
            </td>
            <td class="col-act">
              <button type="button" class="link-btn" @click="removeRow(h)">移除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="mobile-cards mobile-only" aria-label="持仓列表">
      <p v-if="loading && !rows.length" class="empty soft">加载中…</p>
      <p v-else-if="!rows.length" class="empty soft">暂无持仓，请在下方添加</p>
      <article v-for="h in rows" :key="'m-' + rowKey(h)" class="holding-card">
        <div class="holding-card-top">
          <div class="coin-cell">
            <span class="coin-name">{{ displaySymbol(h) }}</span>
            <span v-if="h.assetType === 'alpha'" class="tag">美股</span>
          </div>
          <button type="button" class="link-btn danger" @click="removeRow(h)">移除</button>
        </div>
        <label class="holding-qty">
          <span>数量</span>
          <input
            v-model="draftQty[rowKey(h)]"
            class="qty"
            type="number"
            min="0"
            step="any"
            inputmode="decimal"
            @keydown.enter.prevent="saveHoldings"
          />
        </label>
        <div class="holding-metrics">
          <div>
            <span class="m-label">价值(U)</span>
            <span class="m-val">{{ h.missing ? '—' : fmtMoney(h.valueUsdt, '$') }}</span>
          </div>
          <div>
            <span class="m-label">估值(¥)</span>
            <span class="m-val">{{ h.missing ? '—' : fmtMoney(h.valueCny, '¥') }}</span>
          </div>
          <div>
            <span class="m-label">日涨跌估算</span>
            <span class="m-val" :class="trendClass(h.changeCny)">{{ h.missing ? '—' : fmtSigned(h.changeCny) }}</span>
          </div>
        </div>
      </article>
    </div>

    <div class="add-row">
      <select v-model="addType">
        <option value="crypto">币价</option>
        <option value="alpha">美股参考</option>
      </select>
      <select v-model="addSymbol">
        <option v-for="o in addOptions" :key="o.symbol" :value="o.symbol">
          {{ o.name && o.name !== o.symbol ? `${o.name} (${o.symbol})` : o.symbol }}
        </option>
      </select>
      <input v-model="addQty" type="number" min="0" step="any" inputmode="decimal" placeholder="数量" />
      <button type="button" class="ghost-btn" @click="addRow">添加</button>
      <button type="button" class="primary-btn" :disabled="saving || loading" @click="saveHoldings">保存持仓</button>
    </div>

    <p v-if="error" class="err">{{ error }}</p>
    <p v-else-if="msg" class="ok">{{ msg }}</p>
  </section>
</template>

<style scoped>
.holdings-panel {
  display: grid;
  gap: 12px;
  min-width: 0;
  max-width: 100%;
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
}

.toolbar h2 {
  margin: 0;
  font-size: 15px;
  color: var(--coin);
}

.principal {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
  min-width: 0;
}

.principal label {
  display: grid;
  gap: 4px;
  font-size: 12px;
  color: var(--muted);
  min-width: 0;
}

.principal input,
.add-row input,
.add-row select,
.qty {
  background: var(--panel);
  border: 1px solid var(--line);
  color: var(--text);
  border-radius: 6px;
  padding: 7px 10px;
  font-size: 13px;
  box-sizing: border-box;
  max-width: 100%;
}

.principal input {
  width: 140px;
}

.table-wrap {
  overflow: auto;
  -webkit-overflow-scrolling: touch;
  border: 1px solid var(--line);
  border-radius: var(--radius, 8px);
  min-width: 0;
  max-width: 100%;
  background: color-mix(in srgb, var(--panel) 55%, transparent);
}

.holdings-table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  font-size: 13px;
  table-layout: fixed;
}

.holdings-table th {
  position: sticky;
  top: 0;
  z-index: 1;
  text-align: left;
  padding: 12px 14px;
  color: var(--muted);
  font-weight: 600;
  font-size: 11px;
  letter-spacing: 0.02em;
  border-bottom: 1px solid var(--line);
  white-space: nowrap;
  background: color-mix(in srgb, var(--panel) 94%, transparent);
  box-shadow: 0 1px 0 var(--line);
}

.holdings-table th .th-sub {
  display: block;
  margin-top: 2px;
  font-weight: 500;
  font-size: 10px;
  opacity: 0.85;
  white-space: nowrap;
}

.holdings-table th.num,
.holdings-table td.num {
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.holdings-table td {
  padding: 12px 14px;
  border-bottom: 1px solid var(--line);
  vertical-align: middle;
}

.holdings-table tbody tr:nth-child(even) td {
  background: color-mix(in srgb, var(--hover) 42%, transparent);
}

.holdings-table tbody tr:hover td {
  background: var(--hover-strong, var(--hover));
}

.holdings-table tr:last-child td {
  border-bottom: none;
}

@media (min-width: 681px) {
  .table-wrap {
    max-height: min(52vh, 520px);
  }

  .qty {
    max-width: 140px;
  }

  .add-row {
    padding: 12px 14px;
    border: 1px solid var(--line);
    border-radius: var(--radius, 8px);
    background: color-mix(in srgb, var(--card-soft, var(--panel)) 78%, transparent);
  }
}

.col-coin {
  width: 18%;
}
.col-num {
  width: 26%;
}
.col-val {
  width: 24%;
}
.col-est {
  width: 24%;
}
.col-act {
  width: 8%;
  text-align: center;
}

.coin {
  color: var(--coin);
  font-weight: 700;
}

.coin-cell {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.coin-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--coin);
  font-weight: 700;
}

.tag {
  margin-left: 2px;
  font-size: 10px;
  font-weight: 500;
  color: var(--muted);
  border: 1px solid var(--line);
  border-radius: 4px;
  padding: 1px 4px;
  flex-shrink: 0;
}

.qty {
  width: 100%;
  max-width: 120px;
  min-width: 0;
  appearance: textfield;
  -moz-appearance: textfield;
}

.qty::-webkit-outer-spin-button,
.qty::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.estimated .chg {
  font-size: 12px;
  margin-top: 2px;
}

.empty {
  text-align: center;
  color: var(--muted);
  padding: 20px !important;
}

.empty.soft {
  margin: 0;
  padding: 16px 8px !important;
  font-size: 13px;
}

.mobile-only {
  display: none;
}

.add-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  min-width: 0;
}

.add-row select {
  flex: 1 1 120px;
  min-width: 0;
}

.add-row input {
  flex: 1 1 88px;
  min-width: 0;
  width: auto;
}

.primary-btn,
.ghost-btn,
.link-btn {
  border-radius: 6px;
  font-size: 13px;
  cursor: pointer;
}

.primary-btn {
  border: none;
  background: var(--coin);
  color: #111;
  padding: 8px 14px;
  font-weight: 600;
}

.ghost-btn {
  border: 1px solid var(--line);
  background: transparent;
  color: var(--text);
  padding: 7px 12px;
}

.link-btn {
  border: none;
  background: transparent;
  color: var(--muted);
  padding: 0;
  white-space: nowrap;
}

.link-btn:hover,
.link-btn.danger {
  color: var(--warning);
}

.primary-btn:disabled,
.ghost-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.err {
  margin: 0;
  color: var(--warning);
  font-size: 13px;
  overflow-wrap: anywhere;
}

.ok {
  margin: 0;
  color: var(--up);
  font-size: 13px;
}

.up {
  color: var(--up);
}
.down {
  color: var(--down);
}
.flat {
  color: var(--muted);
}

.holding-card {
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 12px;
  background: color-mix(in srgb, var(--panel) 90%, transparent);
  display: grid;
  gap: 10px;
}

.holding-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.holding-qty {
  display: grid;
  gap: 4px;
  font-size: 12px;
  color: var(--muted);
}

.holding-qty .qty {
  max-width: none;
  font-size: 16px;
  padding: 10px 12px;
}

.holding-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.holding-metrics .m-label {
  display: block;
  font-size: 11px;
  color: var(--muted);
  margin-bottom: 2px;
}

.holding-metrics .m-val {
  display: block;
  font-size: 13px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
  color: var(--text);
  overflow-wrap: anywhere;
}

.holding-metrics .m-val.up {
  color: var(--up);
}

.holding-metrics .m-val.down {
  color: var(--down);
}

.holding-metrics .m-val.flat {
  color: var(--muted);
}

@media (max-width: 680px) {
  .desktop-only {
    display: none;
  }

  .mobile-only {
    display: grid;
    gap: 10px;
  }

  .toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .principal {
    width: 100%;
  }

  .principal label {
    flex: 1 1 auto;
  }

  .principal input {
    width: 100%;
  }

  .principal .ghost-btn {
    flex: 0 0 auto;
  }

  .add-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .add-row select,
  .add-row input,
  .add-row .ghost-btn,
  .add-row .primary-btn {
    flex: none;
    width: 100%;
  }

  .add-row .ghost-btn,
  .add-row .primary-btn {
    min-height: 40px;
  }
}
</style>
