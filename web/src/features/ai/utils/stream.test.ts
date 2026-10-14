import { describe, expect, it } from 'vitest'
import { readChatEvents } from './stream'
import type { AiStreamEvent } from '../types'

function body(text: string, fragment = 1) {
  const bytes = new TextEncoder().encode(text)
  return new ReadableStream<Uint8Array>({ start(controller) {
    for (let i = 0; i < bytes.length; i += fragment) controller.enqueue(bytes.slice(i, i + fragment))
    controller.close()
  } })
}

describe('AI event stream framing', () => {
  it('handles UTF8 and CRLF split across arbitrary network chunks', async () => {
    const events: AiStreamEvent[] = []
    for await (const event of readChatEvents(body('event: token\r\ndata: {"text":"英伟达"}\r\n\r\nevent: done\r\ndata: {}\r\n\r\n'))) events.push(event)
    expect(events).toEqual([{ event: 'token', data: { text: '英伟达' } }, { event: 'done', data: {} }])
  })
  it('supports multiline data, comments, unknown events and a terminal frame without trailing newline', async () => {
    const text = ': heartbeat\n\nevent: other\ndata: {}\n\nevent: token\ndata: {\ndata: "text":"片段"}\n\nevent: done\ndata: {}'
    const events: AiStreamEvent[] = []
    for await (const event of readChatEvents(body(text, 5))) events.push(event)
    expect(events.map((event) => event.event)).toEqual(['token', 'done'])
  })
  it('preserves received tokens but rejects EOF without done or error', async () => {
    const events: AiStreamEvent[] = []
    await expect((async () => { for await (const event of readChatEvents(body('event: token\ndata: {"text":"部分"}\n\n'))) events.push(event) })()).rejects.toThrow('连接提前中断')
    expect(events).toHaveLength(1)
  })
  it('does not swallow malformed JSON or invalid token payload', async () => {
    for (const data of ['broken', '{"text":2}']) await expect((async () => { for await (const _ of readChatEvents(body(`event: token\ndata: ${data}\n\n`))) { /* consume */ } })()).rejects.toThrow()
  })
  it('ends on error and ignores any later success frame', async () => {
    const events: AiStreamEvent[] = []
    for await (const event of readChatEvents(body('event: error\ndata: {"message":"failed"}\n\nevent: done\ndata: {}\n\n'))) events.push(event)
    expect(events).toHaveLength(1)
    expect(events[0].event).toBe('error')
  })
  it('cancels a reader immediately while waiting for upstream data', async () => {
    const controller = new AbortController()
    let cancelled = false
    const stream = new ReadableStream<Uint8Array>({ cancel() { cancelled = true } })
    const iterator = readChatEvents(stream, controller.signal)
    const next = iterator.next()
    controller.abort()
    await expect(next).rejects.toMatchObject({ name: 'AbortError' })
    expect(cancelled).toBe(true)
    expect(stream.locked).toBe(false)
  })
})
