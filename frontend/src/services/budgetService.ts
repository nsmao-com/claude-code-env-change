import type { BudgetSettings, BudgetStatus } from '@/types'

const api = () => window.go.main.BudgetService

export const budgetService = {
  async getSettings(): Promise<BudgetSettings> {
    const settings = await api().GetBudgetSettings()
    return { rules: settings?.rules || [], warn_percent: settings?.warn_percent || 80 }
  },
  save(settings: BudgetSettings): Promise<void> {
    return api().SaveBudgetSettings(settings)
  },
  async status(): Promise<BudgetStatus[]> {
    return (await api().GetBudgetStatus()) || []
  },
}
