// Theme management. applyStoredTheme() is invoked from main.tsx BEFORE React
// renders so the first paint already matches the user's saved preference.

export type Theme = 'light' | 'dark' | 'system'

const THEME_KEY = 'scribe.theme'
const SYSTEM_QUERY = '(prefers-color-scheme: dark)'

function resolve(theme: Theme): 'light' | 'dark' {
  if (theme === 'system') {
    return window.matchMedia(SYSTEM_QUERY).matches ? 'dark' : 'light'
  }
  return theme
}

function applyResolved(resolved: 'light' | 'dark') {
  const root = document.documentElement
  root.classList.remove('dark', 'light')
  root.classList.add(resolved)
  root.style.colorScheme = resolved
}

export function getStoredTheme(): Theme {
  const raw = (typeof localStorage !== 'undefined' && localStorage.getItem(THEME_KEY)) || ''
  return raw === 'light' || raw === 'dark' || raw === 'system' ? raw : 'light'
}

export function setStoredTheme(theme: Theme) {
  localStorage.setItem(THEME_KEY, theme)
}

export function applyTheme(theme: Theme) {
  applyResolved(resolve(theme))
}

let mediaUnsub: (() => void) | null = null

export function trackSystemTheme(active: boolean) {
  if (mediaUnsub) {
    mediaUnsub()
    mediaUnsub = null
  }
  if (!active) return
  const mq = window.matchMedia(SYSTEM_QUERY)
  const handler = () => applyResolved(mq.matches ? 'dark' : 'light')
  mq.addEventListener('change', handler)
  mediaUnsub = () => mq.removeEventListener('change', handler)
}

export function applyStoredTheme() {
  const t = getStoredTheme()
  applyTheme(t)
  trackSystemTheme(t === 'system')
}
