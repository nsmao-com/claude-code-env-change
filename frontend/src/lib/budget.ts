import type { BudgetRule, BudgetStatus } from '@/types'

type Translate = (key: string, vars?: Record<string, string | number>) => string

export const BUDGET_PROVIDERS: { id: string, label: string }[] = [
  { id: 'claude', label: 'Claude Code' },
  { id: 'claude_desktop', label: 'Claude Desktop' },
  { id: 'codex', label: 'Codex' },
  { id: 'antigravity', label: 'Antigravity' },
  { id: 'opencode', label: 'OpenCode' },
  { id: 'grok', label: 'Grok' },
]

export function budgetProviderName(id?: string) {
  return BUDGET_PROVIDERS.find(item => item.id === id)?.label || id || ''
}

export function budgetScopeLabel(t: Translate, rule: BudgetRule) {
  if (rule.scope === 'provider') return budgetProviderName(rule.provider)
  if (rule.scope === 'env') return `${budgetProviderName(rule.provider)} / ${rule.env_name || ''}`
  return t('budget.scopeAll')
}

export function budgetPeriodLabel(t: Translate, period: string) {
  return period === 'month' ? t('budget.month') : t('budget.day')
}

export function formatUsd(value: number) {
  return `$${(Number.isFinite(value) ? value : 0).toFixed(2)}`
}

// 预算提醒的文字（界面通知用，跟随界面语言）
export function budgetAlertMessage(t: Translate, status: BudgetStatus) {
  return t(status.level === 'over' ? 'budget.alertOver' : 'budget.alertWarn', {
    scope: budgetScopeLabel(t, status.rule),
    period: budgetPeriodLabel(t, status.rule.period),
    spent: formatUsd(status.spent),
    limit: formatUsd(status.rule.limit),
    percent: Math.round(status.percent),
  })
}
