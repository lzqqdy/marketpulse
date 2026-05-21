import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ThemeMode = 'dark' | 'light'

const STORAGE_KEY = 'marketpulse-theme'

function savedTheme(): ThemeMode {
  if (typeof window === 'undefined') return 'dark'
  return window.localStorage.getItem(STORAGE_KEY) === 'light' ? 'light' : 'dark'
}

function applyTheme(mode: ThemeMode) {
  if (typeof document === 'undefined') return
  document.documentElement.dataset.theme = mode
}

export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>(savedTheme())

  function setMode(next: ThemeMode) {
    mode.value = next
    applyTheme(next)
    window.localStorage.setItem(STORAGE_KEY, next)
  }

  function toggle() {
    setMode(mode.value === 'dark' ? 'light' : 'dark')
  }

  applyTheme(mode.value)

  return {
    mode,
    setMode,
    toggle,
  }
})
