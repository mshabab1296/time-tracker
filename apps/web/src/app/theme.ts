export type ThemePreference = 'system' | 'light' | 'dark'

export const storageKey = 'timetracker.theme'
export const darkScheme = '(prefers-color-scheme: dark)'

export function readPreference(): ThemePreference {
  try {
    const saved = window.localStorage.getItem(storageKey)
    if (saved === 'light' || saved === 'dark') return saved
  } catch {
    // Storage may be unavailable; the system preference still works.
  }
  return 'system'
}

export function applyTheme(preference: ThemePreference) {
  const systemIsDark = window.matchMedia?.(darkScheme).matches ?? false
  document.documentElement.dataset.theme =
    preference === 'system' ? (systemIsDark ? 'dark' : 'light') : preference
}

export function initializeTheme() {
  applyTheme(readPreference())
}
