import type { Provider } from '@/types'

export type WorkspaceTool = Provider | 'all'

export const WORKSPACE_TOOLS: { id: WorkspaceTool; label: string }[] = [
  { id: 'all', label: '全部' },
  { id: 'claude', label: 'Claude' },
  { id: 'codex', label: 'Codex' },
  { id: 'gemini', label: 'Gemini' },
  { id: 'openclaw', label: 'OpenClaw' },
]

export function toolLabel(tool: WorkspaceTool) {
  return WORKSPACE_TOOLS.find(item => item.id === tool)?.label || '全部'
}

export function toolToPlatform(tool: WorkspaceTool): string {
  if (tool === 'all') return 'all'
  if (tool === 'claude') return 'claude-code'
  return tool
}
