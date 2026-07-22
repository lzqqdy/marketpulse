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
          <input v-model="principal" type="number" min="0" step="0.01" />
        </label>
        <button type="button" class="ghost-btn" :disabled="saving || loading" @click="savePrincipal">保存本金</button>
      </div>
    </div>

    <div class="table-wrap">
      <table class="holdings-table">
        <thead>
          <tr>
            <th>Coin</th>
            <th>Num</th>
            <th class="num">Value</th>
            <th class="num">Estimated</th>
            <th></th>
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
            <td class="coin">
              {{ displaySymbol(h) }}
              <span v-if="h.assetType === 'alpha'" class="tag">美股</span>
            </td>
            <td>
              <input
                v-model="draftQty[rowKey(h)]"
                class="qty"
                type="number"
                min="0"
                step="any"
                @keydown.enter.prevent="saveHoldings"
              />
            </td>
            <td class="num">{{ h.missing ? '—' : fmtMoney(h.valueUsdt, '$') }}</td>
            <td class="num estimated">
              <div>{{ h.missing ? '—' : fmtMoney(h.valueCny, '¥') }}</div>
              <div class="chg" :class="trendClass(h.changeCny)">{{ h.missing ? '' : fmtSigned(h.changeCny) }}</div>
            </td>
            <td>
              <button type="button" class="link-btn" @click="removeRow(h)">移除</button>
            </td>
          </tr>
        </tbody>
      </table>
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
      <input v-model="addQty" type="number" min="0" step="any" placeholder="数量" />
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
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
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
}

.principal label {
  display: grid;
  gap: 4px;
  font-size: 12px;
  color: var(--muted);
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
}

.table-wrap {
  overflow-x: auto;
  border: 1px solid var(--line);
  border-radius: 8px;
}

.holdings-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.holdings-table th {
  text-align: left;
  padding: 10px 12px;
  color: var(--coin);
  font-weight: 600;
  border-bottom: 1px solid var(--line);
}

.holdings-table th.num,
.holdings-table td.num {
  text-align: right;
}

.holdings-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--line);
  vertical-align: middle;
}

.holdings-table tr:last-child td {
  border-bottom: none;
}

.coin {
  color: var(--coin);
  font-weight: 700;
  white-space: nowrap;
}

.tag {
  margin-left: 6px;
  font-size: 10px;
  font-weight: 500;
  color: var(--muted);
  border: 1px solid var(--line);
  border-radius: 4px;
  padding: 1px 5px;
}

.qty {
  width: 110px;
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

.add-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
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
}

.link-btn:hover {
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
</style>
