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
  input: number
  output: number
  daily_exceeded: boolean
  monthly_exceeded: boolean
  by_route: Record<string, number>
}
