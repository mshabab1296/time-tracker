import { createContext, useContext } from 'react'
import type { ThemePreference } from './theme'

export const ThemeContext = createContext<{
  preference: ThemePreference
  setPreference: (preference: ThemePreference) => void
} | null>(null)

export function useTheme() {
  const context = useContext(ThemeContext)
  if (!context) throw new Error('Theme context is unavailable')
  return context
}
