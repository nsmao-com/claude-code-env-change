import { useI18n } from '@/composables/useI18n'
import { computed, ref } from 'vue'
import type { EnvConfig } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'
import { configBaseUrl, errorMessage, formatLatency } from '@/lib/configUrl'

export function useConfigLatency() {
  const configStore = useConfigStore()
  const toast = useToast()
  const { t } = useI18n()
  const testing = ref(false)
  const latencyMs = ref<number | null>(null)
  const failed = ref(false)

  const latencyLabel = computed(() => {
    if (testing.value) return t('ui.testing')
    if (failed.value) return t('ui.failed')
    if (latencyMs.value == null) return ''
    return formatLatency(latencyMs.value)
  })

  async function test(config: EnvConfig) {
    if (testing.value) return
    const url = configBaseUrl(config)
    if (!url) {
      failed.value = true
      toast.error(t('ui.emptyBaseUrl'))
      return
    }
    testing.value = true
    failed.value = false
    try {
      const ms = await configStore.testLatency(url)
      latencyMs.value = ms
      toast.success(t('ui.latency', { value: formatLatency(ms) }))
    } catch (e: unknown) {
      failed.value = true
      latencyMs.value = null
      toast.error(t('ui.latencyFailed', { error: errorMessage(e) }))
    } finally {
      testing.value = false
    }
  }

  return { testing, latencyLabel, test }
}
