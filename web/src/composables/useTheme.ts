import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

export type Theme = 'system' | 'light' | 'dark'
export type EffectiveTheme = 'light' | 'dark'

const STORAGE_KEY = 'bookshelf:theme'

function readStored(): Theme {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'light' || v === 'dark' || v === 'system') return v
  } catch {
    // ignore
  }
  return 'system'
}

function systemPrefersDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function applyTheme(effective: EffectiveTheme): void {
  document.documentElement.classList.toggle('dark', effective === 'dark')
  // ensure native form controls (selects, scrollbars, date pickers) follow the
  // app theme rather than the OS color scheme.
  document.documentElement.style.colorScheme = effective
}

const theme = ref<Theme>(readStored())

export function useTheme() {
  const effectiveTheme = computed<EffectiveTheme>(() => {
    if (theme.value === 'system') return systemPrefersDark() ? 'dark' : 'light'
    return theme.value
  })

  function setTheme(next: Theme): void {
    theme.value = next
    try {
      localStorage.setItem(STORAGE_KEY, next)
    } catch {
      // ignore
    }
  }

  watch(
    effectiveTheme,
    (v) => {
      applyTheme(v)
    },
    { immediate: true },
  )

  let mql: MediaQueryList | null = null
  const onSystemChange = (): void => {
    if (theme.value === 'system') {
      applyTheme(systemPrefersDark() ? 'dark' : 'light')
    }
  }

  onMounted(() => {
    mql = window.matchMedia('(prefers-color-scheme: dark)')
    mql.addEventListener('change', onSystemChange)
  })

  onBeforeUnmount(() => {
    mql?.removeEventListener('change', onSystemChange)
  })

  return { theme, effectiveTheme, setTheme }
}
