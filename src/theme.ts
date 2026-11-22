import { useCallback, useEffect, useState } from 'react'

export type Theme = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'mks-theme'
const media = () => window.matchMedia('(prefers-color-scheme: dark)')

function apply(theme: Theme) {
  const dark = theme === 'dark' || (theme === 'system' && media().matches)
  document.documentElement.classList.toggle('dark', dark)
}

function stored(): Theme | null {
  try {
    return localStorage.getItem(STORAGE_KEY) as Theme | null
  } catch {
    return null
  }
}

/**
 * Light, dark or follow the OS. Stored per browser; until the user picks one, fallback
 * (the company's look) decides.
 */
export function useTheme(fallback: Theme = 'system'): [Theme, (t: Theme) => void] {
  const [theme, setThemeState] = useState<Theme>(() => stored() ?? fallback)

  useEffect(() => {
    if (!stored()) setThemeState(fallback)
  }, [fallback])

  useEffect(() => {
    apply(theme)
    if (theme !== 'system') return
    const m = media()
    const onChange = () => apply('system')
    m.addEventListener('change', onChange)
    return () => m.removeEventListener('change', onChange)
  }, [theme])

  const setTheme = useCallback((t: Theme) => {
    localStorage.setItem(STORAGE_KEY, t)
    setThemeState(t)
  }, [])

  return [theme, setTheme]
}
