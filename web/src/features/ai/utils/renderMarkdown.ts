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

/** 轻量 Markdown：加粗/斜体/代码/列表/换行 + 可点击标的。 */
export function renderAssistantHtml(text: string, symbols: string[] = []): string {
  if (!text) return ''
  const lines = text.replace(/\r\n/g, '\n').split('\n')
  const blocks: string[] = []
  let listBuf: string[] = []
  let ordered = false

  const flushList = () => {
    if (!listBuf.length) return
    const tag = ordered ? 'ol' : 'ul'
    blocks.push(`<${tag}>${listBuf.map((li) => `<li>${li}</li>`).join('')}</${tag}>`)
    listBuf = []
  }

  for (const raw of lines) {
    const ul = raw.match(/^\s*[-*]\s+(.+)$/)
    const ol = raw.match(/^\s*\d+\.\s+(.+)$/)
    if (ul) {
      if (listBuf.length && ordered) flushList()
      ordered = false
      listBuf.push(applyInline(escapeHtml(ul[1])))
      continue
    }
    if (ol) {
      if (listBuf.length && !ordered) flushList()
      ordered = true
      listBuf.push(applyInline(escapeHtml(ol[1])))
      continue
    }
    flushList()
    if (!raw.trim()) {
      blocks.push('<br>')
      continue
    }
    blocks.push(`<p>${applyInline(escapeHtml(raw))}</p>`)
  }
  flushList()

  return linkSymbols(blocks.join(''), symbols)
}
