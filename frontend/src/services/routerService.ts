import type { RouterConfig, GatewayStatus, RouterTestResult, RouterLogQuery, RouterLogPage } from '@/types'

export const routerService = {
  async getConfig(): Promise<RouterConfig> {
    return window.go.main.RouterService.GetRouterConfig()
  },

  async saveConfig(config: RouterConfig): Promise<void> {
    return window.go.main.RouterService.SaveRouterConfig(config)
  },

  async startGateway(): Promise<void> {
    return window.go.main.RouterService.StartGateway()
  },

  async stopGateway(): Promise<void> {
    return window.go.main.RouterService.StopGateway()
  },

  async getStatus(): Promise<GatewayStatus> {
    return window.go.main.RouterService.GetGatewayStatus()
  },

  async testRoute(name: string): Promise<RouterTestResult> {
    return window.go.main.RouterService.TestRoute(name)
  },

  async getLogs(query: RouterLogQuery): Promise<RouterLogPage> {
    return window.go.main.RouterService.GetRouterLogs(query)
  },

  async clearLogs(): Promise<void> {
    return window.go.main.RouterService.ClearRouterLogs()
  }
}
