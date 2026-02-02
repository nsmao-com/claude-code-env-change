import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { RotationGroup, UptimeSettings, UptimeSnapshot, UptimeCheck } from '@/types'
import { uptimeService } from '@/services/uptimeService'
import { useConfigStore } from '@/stores/configStore'

export const useUptimeStore = defineStore('uptime', () => {
  const snapshot = ref<UptimeSnapshot | null>(null)
  const isLoading = ref(false)
  const isRunning = ref(false)

  let timer: number | null = null

  const settings = computed<UptimeSettings>(() => {
    return snapshot.value?.settings || { enabled: false, interval_seconds: 300, timeout_seconds: 8, keep_last: 10 }
  })

  const groups = computed<RotationGroup[]>(() => snapshot.value?.groups || [])
  const history = computed<Record<string, UptimeCheck[]>>(() => snapshot.value?.history || {})
  const urls = computed<Record<string, string>>(() => snapshot.value?.urls || {})

  async function loadSnapshot() {
    isLoading.value = true
    try {
      snapshot.value = await uptimeService.getSnapshot()
      syncTimer()
    } finally {
      isLoading.value = false
    }
  }

  async function saveSettings(next: UptimeSettings) {
    await uptimeService.saveSettings(next)
    await loadSnapshot()
  }

  async function saveGroup(group: RotationGroup) {
    await uptimeService.saveRotationGroup(group)
    await loadSnapshot()
  }

  async function deleteGroup(name: string) {
    await uptimeService.deleteRotationGroup(name)
    await loadSnapshot()
  }

  async function runOnce() {
    if (isRunning.value) return
    isRunning.value = true
    try {
      snapshot.value = await uptimeService.runOnce()
      await useConfigStore().loadConfig()
      syncTimer()
    } finally {
      isRunning.value = false
    }
  }

  function getHistory(name: string): UptimeCheck[] {
    return history.value[name] || []
  }

  function syncTimer() {
    stopTimer()
    if (!settings.value.enabled) return
    const intervalMs = Math.max(30, settings.value.interval_seconds) * 1000
    timer = window.setInterval(() => {
      runOnce()
    }, intervalMs)
  }

  function stopTimer() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  return {
    snapshot,
    settings,
    groups,
    history,
    urls,
    isLoading,
    isRunning,
    loadSnapshot,
    saveSettings,
    saveGroup,
    deleteGroup,
    runOnce,
    getHistory,
    syncTimer,
    stopTimer
  }
})
