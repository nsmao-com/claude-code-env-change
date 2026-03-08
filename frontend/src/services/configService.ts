import type { EnvConfig, Config, Provider } from '@/types'
import {
  GetConfig,
  AddEnv,
  UpdateEnv,
  DeleteEnv,
  SwitchToEnv,
  ApplyCurrentEnv,
  ReorderEnvs,
  RefreshConfig,
  TestLatency,
  ClearAllEnv,
  ClearClaudeSettings,
  ClearCodexSettings,
  ClearGeminiSettings,
  ClearOpenclawSettings,
  GetClaudeSettings,
  GetCodexSettings,
  GetGeminiSettings,
  GetOpenclawSettings,
  ExportConfig,
  ImportConfig
} from '../../wailsjs/go/main/App'

export const configService = {
  async getConfig(): Promise<Config> {
    const raw = await GetConfig()
    const environments: EnvConfig[] = (raw.environments || []).map((env) => ({
      ...env,
      provider: normalizeProvider(env.provider)
    }))

    return {
      ...raw,
      environments,
      current_env_openclaw: raw.current_env_openclaw || ''
    }
  },

  async addEnv(config: EnvConfig): Promise<void> {
    return AddEnv(config)
  },

  async updateEnv(oldName: string, config: EnvConfig): Promise<void> {
    return UpdateEnv(oldName, config)
  },

  async deleteEnv(name: string): Promise<void> {
    return DeleteEnv(name)
  },

  async switchToEnv(name: string): Promise<void> {
    return SwitchToEnv(name)
  },

  async applyCurrentEnv(): Promise<string> {
    return ApplyCurrentEnv()
  },

  async reorderEnvs(names: string[]): Promise<void> {
    return ReorderEnvs(names)
  },

  async refreshConfig(): Promise<void> {
    return RefreshConfig()
  },

  async testLatency(url: string): Promise<number> {
    return TestLatency(url)
  },

  async clearAllEnv(): Promise<void> {
    return ClearAllEnv()
  },

  async clearClaudeSettings(): Promise<void> {
    return ClearClaudeSettings()
  },

  async clearCodexSettings(): Promise<void> {
    return ClearCodexSettings()
  },

  async clearGeminiSettings(): Promise<void> {
    return ClearGeminiSettings()
  },

  async clearOpenclawSettings(): Promise<void> {
    return ClearOpenclawSettings()
  },

  async getClaudeSettings(): Promise<Record<string, string>> {
    return GetClaudeSettings()
  },

  async getCodexSettings(): Promise<Record<string, string>> {
    return GetCodexSettings()
  },

  async getGeminiSettings(): Promise<Record<string, string>> {
    return GetGeminiSettings()
  },

  async getOpenclawSettings(): Promise<Record<string, string>> {
    return GetOpenclawSettings()
  },

  async exportConfig(defaultName: string): Promise<string> {
    return ExportConfig(defaultName)
  },

  async importConfig(): Promise<number> {
    return ImportConfig()
  }
}

function normalizeProvider(provider: string | undefined): Provider {
  switch ((provider || '').toLowerCase()) {
    case 'claude':
      return 'claude'
    case 'codex':
      return 'codex'
    case 'gemini':
      return 'gemini'
    case 'openclaw':
      return 'openclaw'
    default:
      return 'claude'
  }
}
