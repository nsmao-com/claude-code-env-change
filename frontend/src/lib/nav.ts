import type { Component } from 'vue'
import {
  Activity,
  ChartColumn,
  Cloud,
  Layers,
  MessageSquareText,
  Plug,
  Route,
  Sparkles,
} from '@lucide/vue'
import type { AppPage } from '@/types'

export interface AppPageMeta {
  id: AppPage
  label: string
  title: string
  subtitle: string
  icon: Component
}

export const APP_PAGES: AppPageMeta[] = [
  { id: 'env', label: '环境', title: '环境', subtitle: '查看并应用各 CLI 的环境配置。', icon: Layers },
  { id: 'mcp', label: 'MCP', title: 'MCP', subtitle: '管理 Model Context Protocol 服务器。', icon: Plug },
  { id: 'skills', label: 'Skills', title: 'Skills', subtitle: '管理各平台的自定义 SKILL.md。', icon: Sparkles },
  { id: 'router', label: '路由', title: '路由', subtitle: '本地协议转换网关，跨工具复用 API。', icon: Route },
  { id: 'uptime', label: '监控', title: '监控', subtitle: '检测可达性，并按轮换组自动切换配置。', icon: Activity },
  { id: 'cloud', label: '云同步', title: '云同步', subtitle: '把配置备份到对象存储，换电脑后拉取。', icon: Cloud },
  { id: 'prompts', label: '提示词', title: '提示词', subtitle: '编辑 Claude / Codex / Gemini 的自定义提示词。', icon: MessageSquareText },
  { id: 'stats', label: '统计', title: '统计', subtitle: '查看各平台请求量、Token 消耗与花费估算。', icon: ChartColumn },
]

export function pageMeta(id: AppPage) {
  return APP_PAGES.find(item => item.id === id) || APP_PAGES[0]
}
