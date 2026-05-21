import { onMounted, onUnmounted } from 'vue'

interface WakeLockSentinelLike extends EventTarget {
  released: boolean
  release: () => Promise<void>
}

interface WakeLockNavigator extends Navigator {
  wakeLock?: {
    request: (type: 'screen') => Promise<WakeLockSentinelLike>
  }
}

export function useScreenWakeLock() {
  let sentinel: WakeLockSentinelLike | null = null
  let disposed = false

  async function requestWakeLock() {
    if (disposed || document.visibilityState !== 'visible') return
    const wakeLock = (navigator as WakeLockNavigator).wakeLock
    if (!wakeLock || sentinel) return

    try {
      sentinel = await wakeLock.request('screen')
      sentinel.addEventListener('release', handleRelease)
    } catch {
      sentinel = null
    }
  }

  function handleRelease() {
    sentinel?.removeEventListener('release', handleRelease)
    sentinel = null
  }

  function handleVisibilityChange() {
    if (document.visibilityState === 'visible') {
      void requestWakeLock()
    }
  }

  onMounted(() => {
    void requestWakeLock()
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.addEventListener('pageshow', requestWakeLock)
    window.addEventListener('focus', requestWakeLock)
  })

  onUnmounted(() => {
    disposed = true
    document.removeEventListener('visibilitychange', handleVisibilityChange)
    window.removeEventListener('pageshow', requestWakeLock)
    window.removeEventListener('focus', requestWakeLock)
    const current = sentinel
    sentinel = null
    current?.removeEventListener('release', handleRelease)
    if (current && !current.released) {
      void current.release().catch(() => {})
    }
  })
}
