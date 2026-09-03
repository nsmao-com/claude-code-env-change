import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { EnvConfig, Provider } from '@/types'
import { configService } from '@/services/configService'

export const useConfigStore = defineStore('config', () => {
  // State
  const environments = ref<EnvConfig[]>([])
  const currentEnvClaude = ref('')
  const currentEnvCodex = ref('')
  const currentEnvAntigravity = ref('')
  const currentEnvOpencode = ref('')
  const currentEnvsOpencode = ref<string[]>([])
  const currentEnvGrok = ref('')
  const currentFilter = ref<Provider | 'all'>('all')
  const currentEnvTab = ref<Provider>('claude') // 当前环境面板的tab
  const isLoading = ref(false)

  // Getters
  const filteredEnvironments = computed(() => {
    if (currentFilter.value === 'all') {
      return environments.value
    }
    return environments.value.filter(env => env.provider === currentFilter.value)
  })

  const activeEnvs = computed(() => ({
    claude: currentEnvClaude.value,
    codex: currentEnvCodex.value,
    antigravity: currentEnvAntigravity.value,
    opencode: currentEnvOpencode.value,
    grok: currentEnvGrok.value,
  }))

  const claudeEnvs = computed(() =>
    environments.value.filter(env => env.provider === 'claude')
  )

  const codexEnvs = computed(() =>
    environments.value.filter(env => env.provider === 'codex')
  )

  const antigravityEnvs = computed(() =>
    environments.value.filter(env => env.provider === 'antigravity')
  )

  const opencodeEnvs = computed(() =>
    environments.value.filter(env => env.provider === 'opencode')
  )

  const grokEnvs = computed(() =>
    environments.value.filter(env => env.provider === 'grok')
  )

  // Actions
  async function loadConfig() {
    isLoading.value = true
    try {
      const config = await configService.getConfig()
      environments.value = config.environments || []
      currentEnvClaude.value = config.current_env_claude || ''
      currentEnvCodex.value = config.current_env_codex || ''
      currentEnvAntigravity.value = config.current_env_antigravity || ''
      currentEnvOpencode.value = config.current_env_opencode || ''
      const listed = Array.isArray(config.current_envs_opencode) ? config.current_envs_opencode.filter(Boolean) : []
      let extra: string[] = []
      try {
        extra = await configService.getOpencodeAppliedNames()
      } catch {
        extra = []
      }
      const seen = new Set<string>()
      currentEnvsOpencode.value = [...listed, ...extra, config.current_env_opencode || ''].filter((name) => {
        if (!name || seen.has(name)) return false
        seen.add(name)
        return true
      })
      currentEnvGrok.value = config.current_env_grok || ''
    } finally {
      isLoading.value = false
    }
  }

  async function addEnv(config: EnvConfig) {
    await configService.addEnv(config)
    await loadConfig()
  }

  async function updateEnv(oldName: string, oldProvider: string, config: EnvConfig) {
    await configService.updateEnv(oldName, oldProvider, config)
    await loadConfig()
  }

  async function deleteEnv(name: string, provider: string) {
    await configService.deleteEnv(name, provider)
    await loadConfig()
  }

  async function applyEnv(name: string, provider: string): Promise<string | undefined> {
    const env = getEnvByName(name, provider)
    if (env?.provider === 'opencode' && isEnvActive(name, 'opencode')) {
      await configService.unapplyEnv(name)
      await loadConfig()
      currentEnvsOpencode.value = currentEnvsOpencode.value.filter(item => item !== name)
      return 'unapplied'
    }
    const kept = env?.provider === 'opencode' ? [...currentEnvsOpencode.value] : []
    await configService.switchToEnv(name, env?.provider || 'claude')
    const message = await configService.applyCurrentEnv()
    await loadConfig()
    if (env?.provider === 'opencode') {
      const seen = new Set<string>()
      currentEnvsOpencode.value = [...kept, name, ...currentEnvsOpencode.value].filter((item) => {
        if (!item || seen.has(item)) return false
        seen.add(item)
        return true
      })
    }
    return message
  }

  async function unapplyEnv(name: string) {
    await configService.unapplyEnv(name)
    await loadConfig()
  }

  async function reorderEnvs(names: string[]) {
    await configService.reorderEnvs(names)
    await loadConfig()
  }

  async function testLatency(url: string): Promise<number> {
    return configService.testLatency(url)
  }

  async function clearAllEnv() {
    await configService.clearAllEnv()
    await loadConfig()
  }

  async function clearClaudeSettings() {
    await configService.clearClaudeSettings()
    await loadConfig()
  }

  async function clearCodexSettings() {
    await configService.clearCodexSettings()
    await loadConfig()
  }

  async function clearAntigravitySettings() {
    await configService.clearAntigravitySettings()
    await loadConfig()
  }

  async function clearOpencodeSettings() {
    await configService.clearOpencodeSettings()
    await loadConfig()
  }

  async function clearGrokSettings() {
    await configService.clearGrokSettings()
    await loadConfig()
  }

  async function exportConfig(defaultName: string): Promise<string> {
    return configService.exportConfig(defaultName)
  }

  async function importConfig(): Promise<number> {
    const count = await configService.importConfig()
    await loadConfig()
    return count
  }

  async function importConfigJSON(payload: string): Promise<number> {
    const count = await configService.importConfigJSON(payload)
    await loadConfig()
    return count
  }

  async function readDroppedFile(path: string): Promise<string> {
    return configService.readDroppedFile(path)
  }

  async function importLocalEnv(provider: Provider | 'all' = 'all'): Promise<EnvConfig[]> {
    const added = await configService.importLocalEnv(provider)
    await loadConfig()
    return added
  }

  async function getCurrentSettings(provider: Provider): Promise<Record<string, string>> {
    switch (provider) {
      case 'claude':
        return configService.getClaudeSettings()
      case 'codex':
        return configService.getCodexSettings()
      case 'antigravity':
        return configService.getAntigravitySettings()
      case 'opencode':
        return configService.getOpencodeSettings()
      case 'grok':
        return configService.getGrokSettings()
      default:
        return {}
    }
  }

  function setFilter(filter: Provider | 'all') {
    currentFilter.value = filter
    // 同步更新当前环境面板的tab（除了'all'）
    if (filter !== 'all') {
      currentEnvTab.value = filter
    }
  }

  function setEnvTab(tab: Provider) {
    currentEnvTab.value = tab
  }

  function getEnvByName(name: string, provider?: string): EnvConfig | undefined {
    return environments.value.find(env => env.name === name && (provider === undefined || env.provider === provider))
  }

  function isEnvActive(name: string, provider: Provider): boolean {
    switch (provider) {
      case 'claude':
        return currentEnvClaude.value === name
      case 'codex':
        return currentEnvCodex.value === name
      case 'antigravity':
        return currentEnvAntigravity.value === name
      case 'opencode':
        return currentEnvsOpencode.value.includes(name) || currentEnvOpencode.value === name
      case 'grok':
        return currentEnvGrok.value === name
      default:
        return false
    }
  }

  return {
    // State
    environments,
    currentEnvClaude,
    currentEnvCodex,
    currentEnvAntigravity,
    currentEnvOpencode,
    currentEnvsOpencode,
    currentEnvGrok,
    currentFilter,
    currentEnvTab,
    isLoading,

    // Getters
    filteredEnvironments,
    activeEnvs,
    claudeEnvs,
    codexEnvs,
    antigravityEnvs,
    opencodeEnvs,
    grokEnvs,

    // Actions
    loadConfig,
    addEnv,
    updateEnv,
    deleteEnv,
    applyEnv,
    unapplyEnv,
    reorderEnvs,
    testLatency,
    clearAllEnv,
    clearClaudeSettings,
    clearCodexSettings,
    clearAntigravitySettings,
    clearOpencodeSettings,
    clearGrokSettings,
    exportConfig,
    importConfig,
    importConfigJSON,
    readDroppedFile,
    importLocalEnv,
    getCurrentSettings,
    setFilter,
    setEnvTab,
    getEnvByName,
    isEnvActive
  }
})
