import { useSettings } from '@/composables/useSettings'

export function useTheme() {
  const { isDark, setDark, toggleDark } = useSettings()

  return {
    isDark,
    toggle: toggleDark,
    setDark,
  }
}
