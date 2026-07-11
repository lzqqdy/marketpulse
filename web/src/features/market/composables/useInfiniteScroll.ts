import { onMounted, onUnmounted, watch, type Ref } from 'vue'

interface InfiniteScrollOptions {
  rootMargin?: string
  enabled?: () => boolean
}

/**
 * Observes a sentinel and loads the next page when it enters the viewport.
 * Unobserves during fetch to avoid chain-loading that causes scroll jank.
 */
export function useInfiniteScroll(
  sentinel: Ref<HTMLElement | null>,
  onLoadMore: () => void | Promise<void>,
  canLoad: () => boolean,
  options: InfiniteScrollOptions = {},
) {
  const rootMargin = options.rootMargin ?? '0px 0px 120px 0px'
  let observer: IntersectionObserver | null = null
  let pending = false

  function observe() {
    observer?.disconnect()
    const el = sentinel.value
    if (!el) return

    observer = new IntersectionObserver(
      (entries) => {
        if (options.enabled && !options.enabled()) return
        const hit = entries.some((entry) => entry.isIntersecting)
        if (!hit || pending || !canLoad()) return

        pending = true
        observer?.unobserve(el)

        void Promise.resolve(onLoadMore())
          .catch(() => undefined)
          .finally(() => {
            pending = false
            requestAnimationFrame(() => {
              if (!canLoad() || !sentinel.value) return
              observer?.observe(sentinel.value)
            })
          })
      },
      { root: null, rootMargin, threshold: 0 },
    )
    observer.observe(el)
  }

  onMounted(() => observe())
  onUnmounted(() => {
    observer?.disconnect()
    observer = null
  })

  watch(sentinel, (el) => {
    if (el) observe()
  })

  return { reconnect: observe }
}
