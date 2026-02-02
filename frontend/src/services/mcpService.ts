import type { MCPServer, MCPTestResult } from '@/types'

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
  }
}
