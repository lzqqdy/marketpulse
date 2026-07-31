function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function applyInline(escaped: string): string {
  let out = escaped
  out = out.replace(/`([^`]+)`/g, '<code>$1</code>')
  out = out.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  out = out.replace(/(?<!\*)\*([^*]+)\*(?!\*)/g, '<em>$1</em>')
  return out
}

function linkSymbols(html: string, symbols: string[]): string {
  const uniq = [...new Set(symbols.map((s) => s.trim()).filter(Boolean))]
  uniq.sort((a, b) => b.length - a.length)
  if (!uniq.length) return html
  const alt = uniq.map((s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')
  const re = new RegExp(`(?<![\\w/])(${alt})(?![\\w])`, 'gi')
  return html.replace(re, (m) => {
    const canon = uniq.find((s) => s.toLowerCase() === m.toLowerCase()) || m
    return `<button type="button" class="sym-link" data-symbol="${escapeHtml(canon)}">${escapeHtml(m)}</button>`
  })
}

function isTableSep(line: string): boolean {
  const t = line.trim()
  if (!t.includes('|')) return false
  // |---|:---|---:| or ---|---
  return /^\|?[\s:|-]+\|[\s:|-]*\|?$/.test(t) && /-+/.test(t)
}

function isTableRow(line: string): boolean {
  const t = line.trim()
  return t.startsWith('|') && t.includes('|', 1)
}

function splitTableCells(line: string): string[] {
  let t = line.trim()
  if (t.startsWith('|')) t = t.slice(1)
  if (t.endsWith('|')) t = t.slice(0, -1)
  return t.split('|').map((c) => c.trim())
}

function renderTable(rows: string[][]): string {
  if (!rows.length) return ''
  const head = rows[0]
  const body = rows.slice(1)
  const th = head.map((c) => `<th>${applyInline(escapeHtml(c))}</th>`).join('')
  const trs = body
    .map((r) => {
      const cells = head.map((_, i) => r[i] ?? '')
      return `<tr>${cells.map((c) => `<td>${applyInline(escapeHtml(c))}</td>`).join('')}</tr>`
    })
    .join('')
  return `<div class="md-table-wrap"><table><thead><tr>${th}</tr></thead><tbody>${trs}</tbody></table></div>`
}

/** 轻量 Markdown：标题/表格/引用/加粗/斜体/代码/列表/换行 + 可点击标的。 */
export function renderAssistantHtml(text: string, symbols: string[] = []): string {
  if (!text) return ''
  const lines = text.replace(/\r\n/g, '\n').split('\n')
  const blocks: string[] = []
  let listBuf: string[] = []
  let ordered = false
  let quoteBuf: string[] = []
  let i = 0

  const flushList = () => {
    if (!listBuf.length) return
    const tag = ordered ? 'ol' : 'ul'
    blocks.push(`<${tag}>${listBuf.map((li) => `<li>${li}</li>`).join('')}</${tag}>`)
    listBuf = []
  }

  const flushQuote = () => {
    if (!quoteBuf.length) return
    const inner = quoteBuf.map((q) => `<p>${q}</p>`).join('')
    blocks.push(`<blockquote>${inner}</blockquote>`)
    quoteBuf = []
  }

  const flushInlineBlocks = () => {
    flushList()
    flushQuote()
  }

  while (i < lines.length) {
    const raw = lines[i]

    // pipe table: header + separator + body rows
    if (isTableRow(raw) && i + 1 < lines.length && isTableSep(lines[i + 1])) {
      flushInlineBlocks()
      const tableRows: string[][] = [splitTableCells(raw)]
      i += 2 // skip header + separator
      while (i < lines.length && isTableRow(lines[i]) && !isTableSep(lines[i])) {
        tableRows.push(splitTableCells(lines[i]))
        i++
      }
      blocks.push(renderTable(tableRows))
      continue
    }

    const heading = raw.match(/^\s*(#{1,3})\s+(.+)$/)
    if (heading) {
      flushInlineBlocks()
      const level = heading[1].length
      blocks.push(`<h${level}>${applyInline(escapeHtml(heading[2].trim()))}</h${level}>`)
      i++
      continue
    }

    const quote = raw.match(/^\s*>\s?(.*)$/)
    if (quote) {
      flushList()
      quoteBuf.push(applyInline(escapeHtml(quote[1])))
      i++
      continue
    }

    const ul = raw.match(/^\s*[-*]\s+(.+)$/)
    const ol = raw.match(/^\s*\d+\.\s+(.+)$/)
    if (ul) {
      flushQuote()
      if (listBuf.length && ordered) flushList()
      ordered = false
      listBuf.push(applyInline(escapeHtml(ul[1])))
      i++
      continue
    }
    if (ol) {
      flushQuote()
      if (listBuf.length && !ordered) flushList()
      ordered = true
      listBuf.push(applyInline(escapeHtml(ol[1])))
      i++
      continue
    }

    flushInlineBlocks()
    if (!raw.trim()) {
      blocks.push('<br>')
      i++
      continue
    }
    blocks.push(`<p>${applyInline(escapeHtml(raw))}</p>`)
    i++
  }
  flushInlineBlocks()

  return linkSymbols(blocks.join(''), symbols)
}
