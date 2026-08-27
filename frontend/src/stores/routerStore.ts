import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { RouterConfig, GatewayStatus } from '@/types'
import { routerService } from '@/services/routerService'

export const useRouterStore = defineStore('router', () => {
  const config = ref<RouterConfig>({ port: 8790, auto_start: true, routes: [] })
  const status = ref<GatewayStatus | null>(null)
  const isLoading = ref(false)
  const isToggling = ref(false)

  async function loadConfig() {
    isLoading.value = true
    try {
      config.value = await routerService.getConfig()
    } finally {
      isLoading.value = false
    }
  }

  async function refreshStatus() {
    status.value = await routerService.getStatus()
  }

  async function saveConfig(cfg: RouterConfig) {
    await routerService.saveConfig(cfg)
    config.value = cfg
    await refreshStatus()
  }

  async function start() {
    isToggling.value = true
    try {
      await routerService.startGateway()
      await refreshStatus()
    } finally {
      isToggling.value = false
    }
  }

  async function stop() {
    isToggling.value = true
    try {
      await routerService.stopGateway()
      await refreshStatus()
    } finally {
      isToggling.value = false
    }
  }

  return {
    config,
    status,
    isLoading,
    isToggling,
    loadConfig,
    refreshStatus,
    saveConfig,
    start,
    stop
  }
})
