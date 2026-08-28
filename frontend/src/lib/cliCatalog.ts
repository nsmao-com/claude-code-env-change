import type { CliToolStatus, ConfigDirFile, ConfigDirInfo } from '@/types'

export const CLI_TOOL_DEFAULTS: CliToolStatus[] = [
  { id: 'claude', name: 'Claude Code', command: 'claude', installed: false, runnable: false, current_version: '', latest_version: '', install_path: '', install_method: '', config_dir: '~/.claude', config_exists: false, platform: 'Win', upgradable: false, error: '', extra_paths: [], npm_package: '@anthropic-ai/claude-code' },
  { id: 'codex', name: 'Codex', command: 'codex', installed: false, runnable: false, current_version: '', latest_version: '', install_path: '', install_method: '', config_dir: '~/.codex', config_exists: false, platform: 'Win', upgradable: false, error: '', extra_paths: [], npm_package: '@openai/codex' },
  { id: 'gemini', name: 'Gemini CLI', command: 'gemini', installed: false, runnable: false, current_version: '', latest_version: '', install_path: '', install_method: '', config_dir: '~/.gemini', config_exists: false, platform: 'Win', upgradable: false, error: '', extra_paths: [], npm_package: '@google/gemini-cli' },
  { id: 'opencode', name: 'OpenCode', command: 'opencode', installed: false, runnable: false, current_version: '', latest_version: '', install_path: '', install_method: '', config_dir: '~/.config/opencode', config_exists: false, platform: 'Win', upgradable: false, error: '', extra_paths: [], npm_package: 'opencode-ai' },
  { id: 'grok', name: 'Grok', command: 'grok', installed: false, runnable: false, current_version: '', latest_version: '', install_path: '', install_method: '', config_dir: '~/.grok', config_exists: false, platform: 'Win', upgradable: false, error: '', extra_paths: [], npm_package: '@xai-official/grok' },
]

function files(dir: string, names: string[]): ConfigDirFile[] {
  return names.map(name => ({ name, path: `${dir}/${name}`, exists: false }))
}

function pick<T>(obj: Record<string, unknown>, ...keys: string[]): T | undefined {
  for (const key of keys) {
    if (obj[key] !== undefined && obj[key] !== null) return obj[key] as T
  }
  return undefined
}

export function normalizeCliTools(raw: unknown): CliToolStatus[] {
  if (!Array.isArray(raw)) return []
  return raw.map((item) => {
    const row = (item || {}) as Record<string, unknown>
    return {
      id: String(pick(row, 'id', 'ID') || ''),
      name: String(pick(row, 'name', 'Name') || ''),
      command: String(pick(row, 'command', 'Command') || ''),
      installed: Boolean(pick(row, 'installed', 'Installed')),
      runnable: Boolean(pick(row, 'runnable', 'Runnable')),
      current_version: String(pick(row, 'current_version', 'CurrentVersion') || ''),
      latest_version: String(pick(row, 'latest_version', 'LatestVersion') || ''),
      install_path: String(pick(row, 'install_path', 'InstallPath') || ''),
      install_method: String(pick(row, 'install_method', 'InstallMethod') || ''),
      config_dir: String(pick(row, 'config_dir', 'ConfigDir') || ''),
      config_exists: Boolean(pick(row, 'config_exists', 'ConfigExists')),
      platform: String(pick(row, 'platform', 'Platform') || ''),
      upgradable: Boolean(pick(row, 'upgradable', 'Upgradable')),
      error: String(pick(row, 'error', 'Error') || ''),
      extra_paths: Array.isArray(pick(row, 'extra_paths', 'ExtraPaths'))
        ? (pick<unknown[]>(row, 'extra_paths', 'ExtraPaths') || []).map(item => String(item))
        : [],
      npm_package: String(pick(row, 'npm_package', 'NpmPackage') || ''),
    }
  }).filter(item => item.id)
}

export function normalizeConfigDirs(raw: unknown): ConfigDirInfo[] {
  if (!Array.isArray(raw)) return []
  return raw.map((item) => {
    const row = (item || {}) as Record<string, unknown>
    const files = (pick<unknown[]>(row, 'files', 'Files') || []).map((file) => {
      const f = (file || {}) as Record<string, unknown>
      return {
        name: String(pick(f, 'name', 'Name') || ''),
        path: String(pick(f, 'path', 'Path') || ''),
        exists: Boolean(pick(f, 'exists', 'Exists')),
      }
    })
    return {
      id: String(pick(row, 'id', 'ID') || ''),
      name: String(pick(row, 'name', 'Name') || ''),
      dir: String(pick(row, 'dir', 'Dir') || ''),
      exists: Boolean(pick(row, 'exists', 'Exists')),
      files,
    }
  }).filter(item => item.id)
}

export const CONFIG_DIR_DEFAULTS: ConfigDirInfo[] = [
  { id: 'claude', name: 'Claude Code', dir: '~/.claude', exists: false, files: files('~/.claude', ['settings.json']) },
  { id: 'codex', name: 'Codex', dir: '~/.codex', exists: false, files: files('~/.codex', ['config.toml', 'auth.json']) },
  { id: 'gemini', name: 'Gemini CLI', dir: '~/.gemini', exists: false, files: files('~/.gemini', ['.env', 'settings.json']) },
  { id: 'opencode', name: 'OpenCode', dir: '~/.config/opencode', exists: false, files: files('~/.config/opencode', ['opencode.json']) },
  { id: 'grok', name: 'Grok', dir: '~/.grok', exists: false, files: files('~/.grok', ['config.toml']) },
]
