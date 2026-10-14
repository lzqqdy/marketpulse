import type { AiStreamEvent } from '../types'

const EVENTS = new Set(['meta', 'token', 'tool_start', 'tool_result', 'done', 'error'])

export async function* readChatEvents(body: ReadableStream<Uint8Array>, signal?: AbortSignal): AsyncGenerator<AiStreamEvent> {
  const reader = body.getReader()
  const onAbort = () => { void reader.cancel().catch(() => {}) }
  signal?.addEventListener('abort', onAbort, { once: true })
  const decoder = new TextDecoder()
  let buffer = ''
  let terminal = false
  function parse(frame: string): AiStreamEvent | null {
    let name = 'message'
    const data: string[] = []
    for (const line of frame.split('\n')) {
      if (line.startsWith('event:')) name = line.slice(6).trim()
      if (line.startsWith('data:')) data.push(line.slice(5).trimStart())
    }
    if (!data.length || !EVENTS.has(name)) return null
    const value = JSON.parse(data.join('\n'))
    if (!value || typeof value !== 'object') throw new Error('AI 流返回了无效数据')
    if (name === 'token' && typeof value.text !== 'string') throw new Error('AI 文本片段无效')
    if ((name === 'tool_start' || name === 'tool_result') && typeof value.name !== 'string') throw new Error('AI 工具事件无效')
    return { event: name, data: value } as AiStreamEvent
  }
  try {
    if (signal?.aborted) throw new DOMException('Stopped', 'AbortError')
    while (!terminal) {
      const { done, value } = await reader.read()
      if (signal?.aborted) throw new DOMException('Stopped', 'AbortError')
      buffer += done ? decoder.decode() : decoder.decode(value, { stream: true })
      buffer = buffer.replace(/\r\n/g, '\n')
      if (buffer.length > 2 * 1024 * 1024) throw new Error('AI 流数据过大')
      let end: number
      while ((end = buffer.indexOf('\n\n')) >= 0 || (done && buffer.length > 0)) {
        if (end < 0) end = buffer.length
        const event = parse(buffer.slice(0, end))
        buffer = buffer.slice(end + 2)
        if (event) {
          terminal = event.event === 'done' || event.event === 'error'
          yield event
          if (terminal) return
        }
      }
      if (done) break
    }
    if (!terminal) throw new Error('连接提前中断，回答可能不完整')
  } finally {
    signal?.removeEventListener('abort', onAbort)
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
}
