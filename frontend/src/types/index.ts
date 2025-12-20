// 环境配置类型
export interface EnvConfig {
  name: string
  description: string
  variables: Record<string, string>
  provider: string  // 'claude' | 'codex' | 'gemini'
  templates?: Record<string, string>
  icon?: string
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

// Provider 类型
export type Provider = string  // 'claude' | 'codex' | 'gemini'

// Toast 类型
export type ToastType = 'success' | 'error' | 'info'

export interface Toast {
  id: number
  message: string
  type: ToastType
}
