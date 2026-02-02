import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { EnvUsageSummary } from '@/services/logService'
import { getEnvUsageSummary } from '@/services/logService'

export const useUsageStore = defineStore('usage', () => {
  const byEnv = ref<Record<string, EnvUsageSummary>>({})
  const isLoading = ref(false)
  const days = ref(7)

  async function load(nextDays: number = 7) {
    isLoading.value = true
    try {
      days.value = Math.max(1, Math.min(365, Number(nextDays) || 7))
      byEnv.value = await getEnvUsageSummary(days.value)
    } finally {
      isLoading.value = false
    }
  }

  const totalCost = computed(() =>
    Object.values(byEnv.value).reduce((sum, item) => sum + (item.total_cost || 0), 0)
  )

  function getForEnv(name: string): EnvUsageSummary | null {
    return byEnv.value[name] || null
  }

  return {
    byEnv,
    isLoading,
    days,
    totalCost,
    load,
    getForEnv
  }
})

