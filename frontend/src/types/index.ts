// 环境配置类型
export type UpstreamFormat = '' | 'chat_completions' | 'anthropic_messages' | 'responses'

export interface EnvConfig {
  name: string
  description: string
  variables: Record<string, string>
  provider: Provider
  templates?: Record<string, string>
  icon?: string
  /** 上游 API 格式：空 = 原生直连；其余值需本地路由做协议转换 */
  upstream_format?: UpstreamFormat
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
  current_env_opencode: string
  current_env_grok: string
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
  enabled_in_opencode: boolean
  enabled_in_grok: boolean
  frontmatter_name: string
  description: string
  has_frontmatter: boolean
  has_name: boolean
  has_description: boolean
  frontmatter_error: string
}

// 技能预设（内置技能库）
export interface SkillPreset {
  name: string
  description: string
  content: string
}

// API 路由网关类型
export type APIFormat = 'anthropic' | 'openai'

export interface APIRoute {
  name: string
  description?: string
  source_format: APIFormat
  target_format: APIFormat
  base_url: string
  api_key?: string
  model_mapping?: Record<string, string>
  default_model?: string
  enabled: boolean
}

export interface RouterConfig {
  port: number
  auto_start: boolean
  routes: APIRoute[]
}

export interface RouteStats {
  total_requests: number
  failed_requests: number
  last_error?: string
  last_request_at?: number
}

export interface RouterLogEntry {
  time: string
  route: string
  path: string
  model?: string
  status_code: number
  duration_ms: number
  error?: string
}

export interface RouterLogQuery {
  route?: string
  keyword?: string
  only_errors?: boolean
  limit: number
  offset: number
}

export interface RouterLogPage {
  items: RouterLogEntry[]
  total: number
}

export interface GatewayStatus {
  running: boolean
  port: number
  stats: Record<string, RouteStats>
  logs: RouterLogEntry[]
}

export interface RouterTestResult {
  success: boolean
  message: string
  latency: number
}

export type CloudProvider = 's3' | 'aliyun' | 'tencent' | 'r2' | 'minio' | 'custom'

export interface CloudConfig {
  enabled: boolean
  provider: CloudProvider | string
  endpoint: string
  region: string
  bucket: string
  object_key: string
  access_key: string
  secret_key: string
  path_style: boolean
  passphrase?: string
  auto_push: boolean
  auto_pull_on_start: boolean
  last_push_at?: number
  last_pull_at?: number
  last_error?: string
}

export interface CloudSyncResult {
  success: boolean
  message: string
  latency: number
}

export interface CloudSyncStatus {
  enabled: boolean
  configured: boolean
  pushing: boolean
  last_push_at?: number
  last_pull_at?: number
  last_error?: string
  object_key?: string
  provider?: string
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
  provider: Provider
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
export type Provider = 'claude' | 'codex' | 'gemini' | 'opencode' | 'grok'

export type AppPage = 'env' | 'mcp' | 'skills' | 'router' | 'uptime' | 'cloud' | 'prompts' | 'stats'

export interface UpdateInfo {
  available: boolean
  current_version: string
  latest_version: string
  release_name: string
  release_notes: string
  published_at: string
  download_url: string
  asset_name: string
  asset_size: number
  asset_digest: string
  release_url: string
  can_apply: boolean
  is_dev: boolean
  message: string
}

export interface UpdateProgress {
  phase: string
  percent: number
  received: number
  total: number
  message: string
}

// Toast 类型
export type ToastType = 'success' | 'error' | 'info'

export interface Toast {
  id: number
  message: string
  type: ToastType
}
