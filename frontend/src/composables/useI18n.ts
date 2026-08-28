import { computed } from 'vue'
import { formatMessage, lookupMessage, messages, type Locale } from '@/i18n'
import { useSettings } from '@/composables/useSettings'

export function useI18n() {
  const { settings } = useSettings()
  const locale = computed(() => settings.language)

  function t(key: string, vars?: Record<string, string | number>) {
    const tree = messages[locale.value]
    const text = lookupMessage(tree, key) ?? lookupMessage(messages.zh, key) ?? key
    return formatMessage(text, vars)
  }

  function setLocale(next: Locale) {
    settings.language = next
  }

  return { locale, t, setLocale }
}
