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
  claude: 'Claude',
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
  if (!raw) return { preview: null, error: '文件是空的' }
  try {
    const parsed = JSON.parse(raw) as {
      environments?: Array<{ name?: string, provider?: string, icon?: string, description?: string }>
    }
    const list = parsed.environments || []
    if (list.length === 0) {
      return { preview: null, error: '这个 JSON 里没有 environments 配置。如果是 MCP，请到 MCP 页用 JSON 导入。' }
    }
    const items: ImportPreviewItem[] = list.map(item => ({
      name: (item.name || '未命名').trim() || '未命名',
      provider: (item.provider || 'claude').toLowerCase(),
      icon: item.icon || '⌘',
      description: (item.description || '').trim(),
    }))
    return { preview: { items, total: items.length }, error: '' }
  } catch {
    return { preview: null, error: '不是有效的 JSON 文件' }
  }
}
