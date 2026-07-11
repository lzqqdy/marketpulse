import { onMounted, onUnmounted, watch, type Ref } from 'vue'

interface InfiniteScrollOptions {
  rootMargin?: number
  enabled?: () => boolean
}

/**
 * Scroll-based infinite loader. More reliable than IntersectionObserver on mobile,
 * especially when list rows use content-visibility or dynamic heights.
 */
export function useInfiniteScroll(
  sentinel: Ref<HTMLElement | null>,
  onLoadMore: () => void | Promise<void>,
  canLoad: () => boolean,
  options: InfiniteScrollOptions = {},
) {
  const rootMargin = options.rootMargin ?? 120
  let pending = false
  let rafId = 0

  function check() {
    rafId = 0
    if (options.enabled && !options.enabled()) return
    if (pending || !canLoad()) return
    const el = sentinel.value
    if (!el) return
    const rect = el.getBoundingClientRect()
    if (rect.top - rootMargin > window.innerHeight) return

    pending = true
    void Promise.resolve(onLoadMore())
      .catch(() => undefined)
      .finally(() => {
        pending = false
        // Chain while sentinel remains near viewport (fast scroll / short pages).
        scheduleCheck()
      })
  }

  function scheduleCheck() {
    if (rafId) return
    rafId = requestAnimationFrame(check)
  }

  function onScroll() {
    scheduleCheck()
  }

  function observe() {
    scheduleCheck()
  }

  onMounted(() => {
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onScroll, { passive: true })
    observe()
  })

  onUnmounted(() => {
    window.removeEventListener('scroll', onScroll)
    window.removeEventListener('resize', onScroll)
    if (rafId) cancelAnimationFrame(rafId)
  })

  watch(sentinel, (el) => {
    if (el) observe()
  })

  return { reconnect: observe }
}
