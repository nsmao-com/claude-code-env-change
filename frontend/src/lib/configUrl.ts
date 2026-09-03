import type { EnvConfig, Provider } from '@/types'

const URL_KEYS: Record<string, string[]> = {
  claude: ['ANTHROPIC_BASE_URL', 'API_BASE_URL'],
  codex: ['base_url', 'OPENAI_BASE_URL'],
  antigravity: ['GOOGLE_GEMINI_BASE_URL', 'GEMINI_BASE_URL'],
  opencode: ['OPENCODE_BASE_URL'],
  grok: ['XAI_BASE_URL'],
}

const URL_DEFAULTS: Record<string, string> = {
  claude: 'https://api.anthropic.com',
  codex: 'https://api.openai.com/v1',
  antigravity: 'https://generativelanguage.googleapis.com',
  grok: 'https://api.x.ai/v1',
}

export function configBaseUrl(config: Pick<EnvConfig, 'provider' | 'variables'>): string {
  return withDefaultBaseUrl(config.provider, firstVar(config.variables, URL_KEYS[normalizeProvider(config.provider)] || []))
}

export function withDefaultBaseUrl(provider: string, url: string): string {
  const trimmed = (url || '').trim()
  if (trimmed) return trimmed
  return URL_DEFAULTS[normalizeProvider(provider)] || ''
}

export function formatLatency(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return '失败'
  if (ms > 1000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.round(ms)}ms`
}

export function errorMessage(e: unknown): string {
  if (typeof e === 'string' && e.trim()) return e
  if (e instanceof Error && e.message) return e.message
  if (e && typeof e === 'object' && 'message' in e) {
    const msg = String((e as { message?: unknown }).message ?? '')
    if (msg.trim()) return msg
  }
  return '未知错误'
}

function firstVar(vars: Record<string, string> | undefined, keys: string[]): string {
  if (!vars) return ''
  for (const key of keys) {
    const v = (vars[key] || '').trim()
    if (v) return v
  }
  return ''
}

function normalizeProvider(provider: string | undefined): Provider | string {
  return (provider || 'claude').toLowerCase()
}
