import { computed, onMounted, onUnmounted, ref, type CSSProperties } from 'vue'

const STORAGE_KEY = 'marketpulse.ai.fabPos.v1'
const FAB_SIZE = 48
const EDGE = 8
const LONG_PRESS_MS = 420
const MOVE_CANCEL_PX = 10

export type FabPos = { left: number; top: number }

function clamp(left: number, top: number): FabPos {
  const maxL = Math.max(EDGE, window.innerWidth - FAB_SIZE - EDGE)
  const maxT = Math.max(EDGE, window.innerHeight - FAB_SIZE - EDGE)
  return {
    left: Math.min(maxL, Math.max(EDGE, left)),
    top: Math.min(maxT, Math.max(EDGE, top)),
  }
}

function readStored(): FabPos | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<FabPos>
    if (typeof parsed.left !== 'number' || typeof parsed.top !== 'number') return null
    if (!Number.isFinite(parsed.left) || !Number.isFinite(parsed.top)) return null
    return clamp(parsed.left, parsed.top)
  } catch {
    return null
  }
}

function writeStored(pos: FabPos) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(pos))
  } catch {
    /* ignore quota / private mode */
  }
}

/** Long-press drag for the AI FAB; persists left/top in localStorage. */
export function useDraggableFab() {
  const pos = ref<FabPos | null>(null)
  const dragging = ref(false)
  const suppressClick = ref(false)

  let pressTimer: ReturnType<typeof setTimeout> | null = null
  let longPressed = false
  let pointerId: number | null = null
  let startClientX = 0
  let startClientY = 0
  let originLeft = 0
  let originTop = 0
  let activeEl: HTMLElement | null = null

  const fabStyle = computed<CSSProperties>(() => {
    if (!pos.value) return {}
    return {
      left: `${pos.value.left}px`,
      top: `${pos.value.top}px`,
      right: 'auto',
      bottom: 'auto',
    }
  })

  function clearPressTimer() {
    if (pressTimer != null) {
      clearTimeout(pressTimer)
      pressTimer = null
    }
  }

  function rectPos(el: HTMLElement): FabPos {
    const r = el.getBoundingClientRect()
    return clamp(r.left, r.top)
  }

  function onFabPointerDown(e: PointerEvent) {
    if (e.button != null && e.button !== 0) return
    const el = e.currentTarget as HTMLElement | null
    if (!el) return

    clearPressTimer()
    longPressed = false
    suppressClick.value = false
    activeEl = el
    pointerId = e.pointerId
    startClientX = e.clientX
    startClientY = e.clientY

    const base = pos.value ?? rectPos(el)
    originLeft = base.left
    originTop = base.top

    pressTimer = setTimeout(() => {
      longPressed = true
      dragging.value = true
      if (!pos.value) pos.value = { left: originLeft, top: originTop }
      try {
        el.setPointerCapture(e.pointerId)
      } catch {
        /* ignore */
      }
      if (navigator.vibrate) {
        try {
          navigator.vibrate(12)
        } catch {
          /* ignore */
        }
      }
    }, LONG_PRESS_MS)
  }

  function onFabPointerMove(e: PointerEvent) {
    if (pointerId != null && e.pointerId !== pointerId) return
    const dx = e.clientX - startClientX
    const dy = e.clientY - startClientY
    const dist = Math.hypot(dx, dy)

    if (!longPressed) {
      if (dist > MOVE_CANCEL_PX) clearPressTimer()
      return
    }

    e.preventDefault()
    suppressClick.value = true
    pos.value = clamp(originLeft + dx, originTop + dy)
  }

  function onFabPointerUp(e: PointerEvent) {
    if (pointerId != null && e.pointerId !== pointerId) return
    clearPressTimer()

    if (activeEl && pointerId != null) {
      try {
        activeEl.releasePointerCapture(pointerId)
      } catch {
        /* ignore */
      }
    }

    if (longPressed) {
      suppressClick.value = true
      if (pos.value) {
        pos.value = clamp(pos.value.left, pos.value.top)
        writeStored(pos.value)
      }
      window.setTimeout(() => {
        suppressClick.value = false
      }, 280)
    }

    pointerId = null
    activeEl = null
    longPressed = false
    dragging.value = false
  }

  function onFabClick(e: MouseEvent, open: () => void) {
    if (suppressClick.value) {
      e.preventDefault()
      e.stopPropagation()
      return
    }
    open()
  }

  function onResize() {
    if (pos.value) pos.value = clamp(pos.value.left, pos.value.top)
  }

  onMounted(() => {
    pos.value = readStored()
    window.addEventListener('resize', onResize)
  })

  onUnmounted(() => {
    clearPressTimer()
    window.removeEventListener('resize', onResize)
  })

  return {
    fabStyle,
    fabDragging: dragging,
    fabHasCustomPos: computed(() => pos.value != null),
    onFabPointerDown,
    onFabPointerMove,
    onFabPointerUp,
    onFabClick,
  }
}
