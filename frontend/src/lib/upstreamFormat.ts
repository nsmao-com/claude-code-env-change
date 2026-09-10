import type { Provider, UpstreamFormat } from '@/types'

const PROVIDER_SHORT: Record<string, string> = {
  claude: 'Claude',
  claude_desktop: 'Claude Desktop',
  codex: 'Codex',
  antigravity: 'Antigravity',
  opencode: 'OpenCode',
  grok: 'Grok',
}

export function nativeProtocolLabel(provider: string): string {
  switch (provider) {
    case 'claude':
    case 'claude_desktop':
      return 'Anthropic Messages'
    case 'codex':
    case 'grok':
      return 'Responses'
    case 'antigravity':
      return 'Antigravity'
    case 'opencode':
      return 'Chat Completions'
    default:
      return '原生'
  }
}

export function upstreamFormatOptions(provider: string): { value: string; label: string }[] {
  const native = { value: 'native', label: `原生直连（${nativeProtocolLabel(provider)}）` }
  const chat = { value: 'chat_completions', label: 'Chat Completions（需开路由，OpenAI 兼容）' }
  const anthropic = { value: 'anthropic_messages', label: 'Anthropic Messages（需开路由，如 Claude）' }
  const responses = { value: 'responses', label: 'Responses（需开路由，如 Codex）' }
	// 六个平台最终都通过网关归一到三种公开协议，可互相转换。
  return [native, chat, anthropic, responses]
}

export function needsUpstreamRouting(_provider: string, format?: string): boolean {
  const value = (format || '').trim()
  if (!value || value === 'native') return false
  return value === 'chat_completions' || value === 'anthropic_messages' || value === 'responses'
}

export function upstreamFormatShortLabel(format?: string): string {
  if (format === 'chat_completions') return 'Chat Completions'
  if (format === 'anthropic_messages') return 'Anthropic Messages'
  if (format === 'responses') return 'Responses'
  return ''
}

export function conversionTagLabel(provider: string, format?: string): string {
  const from = upstreamFormatShortLabel(format)
  if (!from) return ''
  const dest = PROVIDER_SHORT[provider] || provider
  return `${from} → ${dest}`
}

export function asUpstreamFormat(value: string): UpstreamFormat {
  if (value === 'chat_completions' || value === 'anthropic_messages' || value === 'responses') return value
  return ''
}

export function providerShortName(provider: Provider | string): string {
  return PROVIDER_SHORT[provider] || provider
}
