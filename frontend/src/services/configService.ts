import type { EnvConfig, Config, Provider } from '@/types'
import {
  GetConfig,
  GetOpencodeAppliedNames,
  AddEnv,
  UpdateEnv,
  DeleteEnv,
  SwitchToEnv,
  UnapplyEnv,
  ApplyCurrentEnv,
  ReorderEnvs,
  RefreshConfig,
  TestLatency,
  ClearAllEnv,
  ClearClaudeSettings,
  ClearCodexSettings,
  ClearGeminiSettings,
  ClearOpencodeSettings,
  ClearGrokSettings,
  GetClaudeSettings,
  GetCodexSettings,
  GetGeminiSettings,
  GetOpencodeSettings,
  GetGrokSettings,
  ExportConfig,
  ImportConfig,
  ImportLocalEnv
} from '../../wailsjs/go/main/App'
import { callApp } from '@/services/appBridge'

export const configService = {
  async getOpencodeAppliedNames(): Promise<string[]> {
    try {
      const names = await GetOpencodeAppliedNames()
      return Array.isArray(names) ? names.filter(Boolean) : []
    } catch {
      return []
    }
  },

  async getConfig(): Promise<Config> {
    const raw = await GetConfig()
    const environments: EnvConfig[] = (raw.environments || []).map((env): EnvConfig => ({
      ...env,
      provider: normalizeProvider(env.provider),
      upstream_format: env.upstream_format as EnvConfig['upstream_format'],
    }))

    return {
      ...raw,
      environments,
      current_env_opencode: raw.current_env_opencode || '',
      current_envs_opencode: collectOpencodeApplied(raw),
      current_env_grok: raw.current_env_grok || '',
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

  async unapplyEnv(name: string): Promise<void> {
    return UnapplyEnv(name)
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

  async clearOpencodeSettings(): Promise<void> {
    return ClearOpencodeSettings()
  },

  async clearGrokSettings(): Promise<void> {
    return ClearGrokSettings()
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

  async getOpencodeSettings(): Promise<Record<string, string>> {
    return GetOpencodeSettings()
  },

  async getGrokSettings(): Promise<Record<string, string>> {
    return GetGrokSettings()
  },

  async exportConfig(defaultName: string): Promise<string> {
    return ExportConfig(defaultName)
  },

  async importConfig(): Promise<number> {
    return ImportConfig()
  },

  async importConfigJSON(payload: string): Promise<number> {
    return callApp<number>('ImportConfigJSON', payload)
  },

  async readDroppedFile(path: string): Promise<string> {
    return callApp<string>('ReadDroppedFile', path)
  },

  async importLocalEnv(provider: Provider | 'all'): Promise<EnvConfig[]> {
    const list = await ImportLocalEnv(provider)
    return (list || []).map((env): EnvConfig => ({
      ...env,
      provider: normalizeProvider(env.provider),
      upstream_format: env.upstream_format as EnvConfig['upstream_format'],
    }))
  }
}

function collectOpencodeApplied(raw: {
  current_envs_opencode?: string[]
  current_env_opencode?: string
  CurrentEnvsOpencode?: string[]
  currentEnvsOpencode?: string[]
} | null | undefined): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  const push = (value: unknown) => {
    if (typeof value === 'string' && value.trim() && !seen.has(value.trim())) {
      const name = value.trim()
      seen.add(name)
      out.push(name)
    }
    if (Array.isArray(value)) value.forEach(push)
  }
  if (!raw) return out
  push(raw.current_envs_opencode)
  push(raw.CurrentEnvsOpencode)
  push(raw.currentEnvsOpencode)
  push(raw.current_env_opencode)
  return out
}

function normalizeProvider(provider: string | undefined): Provider {
  switch ((provider || '').toLowerCase()) {
    case 'claude':
      return 'claude'
    case 'codex':
      return 'codex'
    case 'gemini':
      return 'gemini'
    case 'opencode':
      return 'opencode'
    case 'grok':
      return 'grok'
    default:
      return 'claude'
  }
}
