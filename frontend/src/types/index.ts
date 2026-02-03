// 环境配置类型
export interface EnvConfig {
  name: string
  description: string
  variables: Record<string, string>
  provider: string  // 'claude' | 'codex' | 'gemini'
  templates?: Record<string, string>
  icon?: string
  // Claude Code 特有配置 (值为 "0" 或 "1"，空字符串表示不设置)
  attribution_header: string
  disable_nonessential_traffic: string
}

// 应用配置类型
export interface Config {
  current_env: string
  current_env_claude: string
  current_env_codex: string
  current_env_gemini: string
  environments: EnvConfig[]
}

// MCP 服务器类型
export interface MCPServer {
  name: string
  type: string  // 'stdio' | 'http'
  command?: string
  args?: string[]
  env?: Record<string, string>
  url?: string
  website?: string
  tips?: string
  enable_platform: string[]
  enabled_in_claude: boolean
  enabled_in_codex: boolean
  enabled_in_gemini: boolean
  missing_placeholders: string[]
}

// MCP 测试结果
export interface MCPTestResult {
  success: boolean
  message: string
  latency: number
}

// Skills 类型
export interface Skill {
  name: string
  content: string
  enable_platform: string[]
  enabled_in_claude: boolean
  enabled_in_codex: boolean
  enabled_in_gemini: boolean
  frontmatter_name: string
  description: string
  has_frontmatter: boolean
  has_name: boolean
  has_description: boolean
  frontmatter_error: string
}

// Uptime / 轮换
export interface UptimeSettings {
  enabled: boolean
  interval_seconds: number
  timeout_seconds: number
  keep_last: number
}

export interface RotationGroup {
  name: string
  provider: string // 'claude' | 'codex' | 'gemini'
  env_names: string[]
  enabled: boolean
  failure_threshold: number
}

export interface UptimeCheck {
  at: number
  success: boolean
  status_code: number
  latency_ms: number
  error?: string
}

export interface UptimeSnapshot {
  settings: UptimeSettings
  groups: RotationGroup[]
  history: Record<string, UptimeCheck[]>
  urls: Record<string, string>
  now: number
}

// Provider 类型
export type Provider = string  // 'claude' | 'codex' | 'gemini'

// Toast 类型
export type ToastType = 'success' | 'error' | 'info'

export interface Toast {
  id: number
  message: string
  type: ToastType
}
