import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Provider, RouterConfig, GatewayStatus } from '@/types'
import { routerService } from '@/services/routerService'
import { callApp } from '@/services/appBridge'

const emptyAppRouting = (): Record<string, boolean> => ({
  claude: false,
  codex: false,
  gemini: false,
  opencode: false,
  grok: false,
})

function withAppRouting(cfg: RouterConfig): RouterConfig {
  return {
    ...cfg,
    routes: cfg.routes || [],
    app_routing: { ...emptyAppRouting(), ...(cfg.app_routing || {}) },
  }
}

export const useRouterStore = defineStore('router', () => {
  const config = ref<RouterConfig>({
    port: 8790,
    auto_start: true,
    routes: [],
    app_routing: emptyAppRouting(),
  })
  const status = ref<GatewayStatus | null>(null)
  const isLoading = ref(false)
  const isToggling = ref(false)
  const togglingApp = ref('')

  async function loadConfig() {
    isLoading.value = true
    try {
      config.value = withAppRouting(await routerService.getConfig())
    } finally {
      isLoading.value = false
    }
  }

  async function refreshStatus() {
    status.value = await routerService.getStatus()
  }

  async function saveConfig(cfg: RouterConfig) {
    await routerService.saveConfig(cfg)
    config.value = withAppRouting(cfg)
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

  function isAppRoutingOn(provider: string) {
    return !!config.value.app_routing?.[provider]
  }

  async function setAppRouting(provider: Provider, enabled: boolean) {
    togglingApp.value = provider
    try {
      await callApp('SetProviderRouting', provider, enabled)
      await loadConfig()
      await refreshStatus()
    } finally {
      togglingApp.value = ''
    }
  }

  async function refreshRoutedProviders() {
    await callApp('RefreshRoutedProviders')
    await loadConfig()
    await refreshStatus()
  }

  return {
    config,
    status,
    isLoading,
    isToggling,
    togglingApp,
    loadConfig,
    refreshStatus,
    saveConfig,
    start,
    stop,
    isAppRoutingOn,
    setAppRouting,
    refreshRoutedProviders,
  }
})
