import type { MCPServer } from '@/types'

export interface McpExportTarget {
  id: string
  label: string
  /** 保存对话框默认文件名 */
  fileName: string
  language: 'json' | 'toml'
  build: (servers: MCPServer[]) => string
}

interface JsonEntryOptions {
  /** 是否显式输出 type 字段（Claude Code / VS Code / ZCode 官方示例带 type；Cursor / Trae 文档不带） */
  includeType: boolean
  headerKey: string
}

function isRemote(server: MCPServer) {
  return server.type === 'http' || server.type === 'sse'
}

function jsonEntry(server: MCPServer, opts: JsonEntryOptions): Record<string, unknown> {
  const entry: Record<string, unknown> = {}
  if (opts.includeType) entry.type = server.type
  if (isRemote(server)) {
    entry.url = server.url
    if (server.headers && Object.keys(server.headers).length > 0) {
      entry[opts.headerKey] = server.headers
    }
  } else {
    entry.command = server.command
    if (server.args && server.args.length > 0) entry.args = server.args
    if (server.env && Object.keys(server.env).length > 0) entry.env = server.env
  }
  return entry
}

function toJson(servers: MCPServer[], rootKey: string[], opts: JsonEntryOptions): string {
  const entries: Record<string, unknown> = {}
  for (const server of servers) entries[server.name] = jsonEntry(server, opts)
  let root: unknown = entries
  for (let i = rootKey.length - 1; i >= 0; i--) root = { [rootKey[i]]: root }
  return JSON.stringify(root, null, 2) + '\n'
}

function tomlString(value: string): string {
  return '"' + value.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"'
}

function toToml(servers: MCPServer[]): string {
  const lines: string[] = []
  for (const server of servers) {
    lines.push(`[mcp_servers.${server.name}]`)
    if (isRemote(server)) {
      lines.push(`url = ${tomlString(server.url || '')}`)
      if (server.headers && Object.keys(server.headers).length > 0) {
        const pairs = Object.entries(server.headers)
          .map(([key, value]) => `${key} = ${tomlString(value)}`)
          .join(', ')
        lines.push(`http_headers = { ${pairs} }`)
      }
    } else {
      lines.push(`command = ${tomlString(server.command || '')}`)
      if (server.args && server.args.length > 0) {
        lines.push(`args = [${server.args.map(tomlString).join(', ')}]`)
      }
      if (server.env && Object.keys(server.env).length > 0) {
        const pairs = Object.entries(server.env)
          .map(([key, value]) => `${tomlString(key)} = ${tomlString(value)}`)
          .join(', ')
        lines.push(`env = { ${pairs} }`)
      }
    }
    lines.push('')
  }
  return lines.join('\n')
}

export const MCP_EXPORT_TARGETS: McpExportTarget[] = [
  {
    id: 'claude',
    label: 'Claude Code',
    fileName: 'mcp.json',
    language: 'json',
    build: servers => toJson(servers, ['mcpServers'], { includeType: true, headerKey: 'headers' }),
  },
  {
    id: 'cursor',
    label: 'Cursor',
    fileName: 'mcp.json',
    language: 'json',
    build: servers => toJson(servers, ['mcpServers'], { includeType: false, headerKey: 'headers' }),
  },
  {
    id: 'trae',
    label: 'Trae',
    fileName: 'mcp.json',
    language: 'json',
    build: servers => toJson(servers, ['mcpServers'], { includeType: false, headerKey: 'headers' }),
  },
  {
    id: 'zcode',
    label: 'ZCode',
    fileName: 'mcp.json',
    language: 'json',
    build: servers => toJson(servers, ['mcp', 'servers'], { includeType: true, headerKey: 'headers' }),
  },
  {
    id: 'workbuddy',
    label: 'WorkBuddy',
    fileName: 'mcp.json',
    language: 'json',
    build: servers => toJson(servers, ['mcpServers'], { includeType: false, headerKey: 'headers' }),
  },
  {
    id: 'vscode',
    label: 'VS Code',
    fileName: 'mcp.json',
    language: 'json',
    build: servers => toJson(servers, ['servers'], { includeType: true, headerKey: 'headers' }),
  },
  {
    id: 'codex',
    label: 'Codex',
    fileName: 'config.toml',
    language: 'toml',
    build: toToml,
  },
  {
    id: 'generic',
    label: 'Generic JSON',
    fileName: 'mcp.json',
    language: 'json',
    build: servers => toJson(servers, ['mcpServers'], { includeType: true, headerKey: 'headers' }),
  },
]
