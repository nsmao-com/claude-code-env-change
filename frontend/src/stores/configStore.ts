import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { EnvConfig, Provider } from '@/types'
import { configService } from '@/services/configService'

export const useConfigStore = defineStore('config', () => {
  // State
  const environments = ref<EnvConfig[]>([])
  const currentEnvClaude = ref('')
  const currentEnvCodex = ref('')
  const currentEnvGemini = ref('')
  const currentEnvOpencode = ref('')
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
    gemini: currentEnvGemini.value,
    opencode: currentEnvOpencode.value,
    grok: currentEnvGrok.value,
  }))

  const claudeEnvs = computed(() =>
    environments.value.filter(env => env.provider === 'claude')
  )

  const codexEnvs = computed(() =>
    environments.value.filter(env => env.provider === 'codex')
  )

  const geminiEnvs = computed(() =>
    environments.value.filter(env => env.provider === 'gemini')
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
      currentEnvGemini.value = config.current_env_gemini || ''
      currentEnvOpencode.value = config.current_env_opencode || ''
      currentEnvGrok.value = config.current_env_grok || ''
    } finally {
      isLoading.value = false
    }
  }

  async function addEnv(config: EnvConfig) {
    await configService.addEnv(config)
    await loadConfig()
  }

  async function updateEnv(oldName: string, config: EnvConfig) {
    await configService.updateEnv(oldName, config)
    await loadConfig()
  }

  async function deleteEnv(name: string) {
    await configService.deleteEnv(name)
    await loadConfig()
  }

  async function applyEnv(name: string): Promise<string | undefined> {
    await configService.switchToEnv(name)
    const message = await configService.applyCurrentEnv()
    await loadConfig()
    return message
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

  async function clearGeminiSettings() {
    await configService.clearGeminiSettings()
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
      case 'gemini':
        return configService.getGeminiSettings()
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

  function getEnvByName(name: string): EnvConfig | undefined {
    return environments.value.find(env => env.name === name)
  }

  function isEnvActive(name: string, provider: Provider): boolean {
    switch (provider) {
      case 'claude':
        return currentEnvClaude.value === name
      case 'codex':
        return currentEnvCodex.value === name
      case 'gemini':
        return currentEnvGemini.value === name
      case 'opencode':
        return currentEnvOpencode.value === name
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
    currentEnvGemini,
    currentEnvOpencode,
    currentEnvGrok,
    currentFilter,
    currentEnvTab,
    isLoading,

    // Getters
    filteredEnvironments,
    activeEnvs,
    claudeEnvs,
    codexEnvs,
    geminiEnvs,
    opencodeEnvs,
    grokEnvs,

    // Actions
    loadConfig,
    addEnv,
    updateEnv,
    deleteEnv,
    applyEnv,
    reorderEnvs,
    testLatency,
    clearAllEnv,
    clearClaudeSettings,
    clearCodexSettings,
    clearGeminiSettings,
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
