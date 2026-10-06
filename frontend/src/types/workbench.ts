export interface ModelProfile {
  provider: string
  environment: string
  model: string
  context: number
  output: number
  compact: number
  input_price: number
  output_price: number
  cache_read_price: number
  cache_write_price: number
}
export interface PromptPreset {
  name: string
  content: string
}
export interface ProjectPreset {
  name: string
  directory: string
  provider: string
  environment: string
  model: string
  mcp: string[]
  skills: string[]
  prompt: string
}
export interface BalanceSource {
  provider: string
  environment: string
  adapter: string
  url?: string
  path?: string
  divisor?: number
  currency?: string
  alert_below?: number
}
export interface CostSettings {
  daily_budget: number
  monthly_budget: number
  multipliers: Record<string, number>
  balance_sources: BalanceSource[]
}
export interface WorkbenchConfig {
  models: ModelProfile[]
  prompts: PromptPreset[]
  projects: ProjectPreset[]
  costs: CostSettings
  quota?: QuotaSettings
}
export interface FileChange {
  path: string
  before: string
  after: string
  changed: boolean
}
export interface HistoryPreview {
  token: string
  changes: FileChange[]
}
export interface HistorySummary {
  id: string
  at: number
  reason: string
  paths: string[]
}
export interface DiagnosticItem {
  name: string
  status: string
  message: string
  action: string
}
export interface SessionSummary {
  id: string
  session_id: string
  provider: string
  title: string
  project: string
  model: string
  updated: number
  messages: number
  archived: boolean
  path: string
  warning?: string
}
export interface SessionMessage {
  role: string
  text: string
  at: string
}
export interface SessionPage {
  items: SessionSummary[]
  total: number
  warnings: string[]
}
export interface SessionDetail {
  session: SessionSummary
  messages: SessionMessage[]
  total: number
}
export interface Result {
  success: boolean
  message: string
  latency: number
}
export interface CostOverview {
  today: number
  month: number
  requests: number
  unpriced: number
  list_estimate: number
  input: number
  output: number
  daily_exceeded: boolean
  monthly_exceeded: boolean
  by_route: Record<string, number>
}
export interface QuotaWindow {
  name: string
  used: number
  resets_at?: number
  span_hours?: number
  display?: string
  unlimited?: boolean
}
export type QuotaStatus = 'ok' | 'pending' | 'signed_out' | 'not_installed' | 'expired' | 'api_billing' | 'error'
export interface SubscriptionQuota {
  provider: 'claude' | 'codex' | 'copilot'
  name: string
  account?: string
  plan?: string
  windows: QuotaWindow[]
  credits?: string
  until?: number
  status: QuotaStatus
  error?: string
  read_at?: number
}
export interface BalanceCard {
  id: string
  adapter: string
  vendor: string
  host: string
  key_hint: string
  environments: string[]
  amount: number
  currency: string
  display: string
  unlimited?: boolean
  alert_below?: number
  low?: boolean
  configured?: boolean
  error?: string
  read_at?: number
}
export interface WarmupSettings {
  claude: boolean
  codex: boolean
  times?: string[]
  on_reset: boolean
}
export interface QuotaSettings {
  alert_percent: number
  monitor_minutes: number
  desktop_notify: boolean
  saved?: boolean
  warmup: WarmupSettings
}
export interface WarmupStatus {
  provider: string
  last_run?: number
  error?: string
}
export interface QuotaAlert {
  kind: 'quota' | 'balance'
  title: string
  message: string
}
export interface InsightCount {
  name: string
  count: number
  tokens?: number
}
export interface InsightDay {
  date: string
  sessions: number
  prompts: number
  tokens: number
}
export interface InsightSession {
  provider: string
  session_id: string
  title: string
  project: string
  tokens: number
  prompts: number
  tool_calls: number
  active_minutes: number
  last: number
}
export interface SessionInsights {
  from: string
  to: string
  sessions: number
  prompts: number
  replies: number
  tool_calls: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  active_minutes: number
  median_tokens: number
  p90_tokens: number
  days: InsightDay[] | null
  hours: number[]
  weekdays: number[]
  projects: InsightCount[] | null
  models: InsightCount[] | null
  tools: InsightCount[] | null
  skills: InsightCount[] | null
  mcp: InsightCount[] | null
  lengths: InsightCount[] | null
  top: InsightSession[] | null
  scanned: number
  warnings: string[] | null
}
export interface TrashedSession {
  id: string
  provider: string
  title: string
  project: string
  session_id: string
  original: string
  extras?: string[]
  deleted_at: number
  size: number
}
export interface CatalogModel {
  id: string
  name: string
  maker: string
  context?: number
  output?: number
  input?: number
  output_cost?: number
  cache_read?: number
  cache_write?: number
  priced?: boolean
  reasoning?: boolean
  images?: boolean
  released?: string
}
export interface CatalogStatus {
  count: number
  updated_at: number
  syncing: boolean
  error?: string
}
