import { useI18n } from '@/composables/useI18n'
export interface ImportPreviewItem {
  name: string
  provider: string
  icon: string
  description: string
}

export interface ImportPreview {
  items: ImportPreviewItem[]
  total: number
}

const PROVIDER_LABEL: Record<string, string> = {
  claude: 'Claude Code',
  claude_desktop: 'Claude Desktop',
  codex: 'Codex',
  antigravity: 'Antigravity',
  opencode: 'OpenCode',
  grok: 'Grok',
}

export function providerLabel(provider: string) {
  const key = (provider || 'claude').toLowerCase()
  return PROVIDER_LABEL[key] || provider || 'Claude'
}

export function classifyImportPayload(text: string): 'config' | 'mcp' | 'unknown' {
  const raw = text.replace(/^\uFEFF/, '').trim()
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    if (parsed && Array.isArray(parsed.environments)) return 'config'
    if (parsed && typeof parsed === 'object' && (parsed.mcpServers || parsed.command || parsed.url)) return 'mcp'
  } catch {
    /* ignore */
  }
  return 'unknown'
}

export function parseConfigExport(text: string): { preview: ImportPreview | null, error: string } {
  const raw = text.replace(/^\uFEFF/, '').trim()
  const { t } = useI18n()
  if (!raw) return { preview: null, error: t('ui.importEmpty') }
  try {
    const parsed = JSON.parse(raw) as {
      environments?: Array<{ name?: string, provider?: string, icon?: string, description?: string }>
    }
    const list = parsed.environments || []
    if (list.length === 0) {
      return { preview: null, error: t('ui.importNoEnvs') }
    }
    const items: ImportPreviewItem[] = list.map(item => ({
      name: (item.name || t('ui.untitled')).trim() || t('ui.untitled'),
      provider: (item.provider || 'claude').toLowerCase(),
      icon: item.icon || '⌘',
      description: (item.description || '').trim(),
    }))
    return { preview: { items, total: items.length }, error: '' }
  } catch {
    return { preview: null, error: t('ui.importInvalid') }
  }
}
