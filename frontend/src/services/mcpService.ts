import type { MCPServer, MCPTestResult, McpMarketPage } from '@/types'

export const mcpService = {
  async listServers(): Promise<MCPServer[]> {
    const servers = await window.go.main.MCPService.ListServers()
    return servers || []
  },

  async saveServers(servers: MCPServer[]): Promise<void> {
    return window.go.main.MCPService.SaveServers(servers)
  },

  async testServer(server: MCPServer): Promise<MCPTestResult> {
    return window.go.main.MCPService.TestServer(server)
  },

  async importFromJSON(jsonStr: string): Promise<MCPServer[]> {
    return window.go.main.MCPService.ImportFromJSON(jsonStr)
  },

  async addServers(servers: MCPServer[]): Promise<void> {
    return window.go.main.MCPService.AddServers(servers)
  },

  async syncToPlatforms(): Promise<MCPServer[]> {
    const servers = await window.go.main.MCPService.SyncToPlatforms()
    return servers || []
  },

  async applyToPlatform(platform: string): Promise<number> {
    const count = await window.go.main.MCPService.ApplyToPlatform(platform)
    return typeof count === 'number' ? count : 0
  },

  async searchMarketplace(query: string, cursor = ''): Promise<McpMarketPage> {
    const page = await window.go.main.MCPService.SearchMcpMarketplace(query, cursor)
    return page || { items: [] }
  },

  async importMarketplace(id: string, platforms: string[]): Promise<void> {
    return window.go.main.MCPService.ImportMcpMarketplace(id, platforms)
  }
}
