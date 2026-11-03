import { useEffect, useState, type ReactNode } from 'react'
import { ThemeContext } from './ThemeContext'
import { applyTheme, darkScheme, readPreference, storageKey, type ThemePreference } from './theme'

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreferenceState] = useState(readPreference)

  useEffect(() => {
    applyTheme(preference)
    const media = window.matchMedia?.(darkScheme)
    if (preference !== 'system' || !media) return
    const update = () => applyTheme('system')
    media.addEventListener('change', update)
    return () => media.removeEventListener('change', update)
  }, [preference])

  function setPreference(value: ThemePreference) {
    setPreferenceState(value)
    applyTheme(value)
    try {
      if (value === 'system') window.localStorage.removeItem(storageKey)
      else window.localStorage.setItem(storageKey, value)
    } catch {
      // The choice remains active for this page when storage is unavailable.
    }
  }

  return (
    <ThemeContext.Provider value={{ preference, setPreference }}>{children}</ThemeContext.Provider>
  )
}
