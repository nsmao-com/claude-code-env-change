import { useI18n } from '@/composables/useI18n'
import type { Provider } from '@/types'

export type WorkspaceTool = Provider | 'all'

export const WORKSPACE_TOOLS: { id: WorkspaceTool; label: string }[] = [
  { id: 'all', label: '全部' },
  { id: 'claude', label: 'Claude' },
  { id: 'claude_desktop', label: 'Claude Desktop' },
  { id: 'codex', label: 'Codex' },
  { id: 'antigravity', label: 'Antigravity' },
  { id: 'opencode', label: 'OpenCode' },
  { id: 'grok', label: 'Grok' },
]

export function toolLabel(tool: WorkspaceTool) {
  if (tool === 'all') return useI18n().t('ui.all')
  return WORKSPACE_TOOLS.find(item => item.id === tool)?.label || useI18n().t('ui.all')
}

export function toolToPlatform(tool: WorkspaceTool) {
  if (tool === 'all') return 'all'
  if (tool === 'claude') return 'claude-code'
  if (tool === 'claude_desktop') return 'claude-desktop'
  return tool
}
