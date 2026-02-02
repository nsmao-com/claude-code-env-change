import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { MCPServer, MCPTestResult } from '@/types'
import { mcpService } from '@/services/mcpService'

export const useMcpStore = defineStore('mcp', () => {
  // State
  const servers = ref<MCPServer[]>([])
  const testResults = ref<Map<string, MCPTestResult>>(new Map())
  const isLoading = ref(false)
  const isTestingAll = ref(false)

  // Getters
  const serverCount = computed(() => servers.value.length)

  const availableCount = computed(() => {
    let count = 0
    testResults.value.forEach((result) => {
      if (result.success) count++
    })
    return count
  })

  const failedCount = computed(() => {
    let count = 0
    testResults.value.forEach((result) => {
      if (!result.success) count++
    })
    return count
  })

  const testedCount = computed(() => testResults.value.size)

  const testStatusText = computed(() => {
    if (isTestingAll.value) {
      return '检测中...'
    }
    if (testedCount.value === 0) {
      return ''
    }
    const available = availableCount.value
    const failed = failedCount.value
    if (failed === 0) {
      return `${available} 可用`
    }
    return `${available} 可用 / ${failed} 失败`
  })

  // Actions
  async function loadServers() {
    isLoading.value = true
    try {
      servers.value = await mcpService.listServers()
    } finally {
      isLoading.value = false
    }
  }

  async function saveServers(serverList: MCPServer[]) {
    console.log('[MCP Store] 保存服务器列表，数量:', serverList.length)
    console.log('[MCP Store] 服务器名称:', serverList.map(s => s.name))
    await mcpService.saveServers(serverList)
    console.log('[MCP Store] 保存完成，重新加载...')
    await loadServers()
    console.log('[MCP Store] 重新加载完成，当前数量:', servers.value.length)
  }

  async function testServer(server: MCPServer): Promise<MCPTestResult> {
    const result = await mcpService.testServer(server)
    testResults.value.set(server.name, result)
    return result
  }

  async function testAllServers() {
    if (servers.value.length === 0) return

    isTestingAll.value = true
    testResults.value.clear()

    const promises = servers.value.map(async (server) => {
      try {
        const result = await mcpService.testServer(server)
        testResults.value.set(server.name, result)
      } catch (e) {
        testResults.value.set(server.name, {
          success: false,
          message: '测试失败',
          latency: 0
        })
      }
    })

    await Promise.all(promises)
    isTestingAll.value = false
  }

  async function importFromJSON(jsonStr: string): Promise<MCPServer[]> {
    const imported = await mcpService.importFromJSON(jsonStr)
    return imported
  }

  async function addServers(newServers: MCPServer[]) {
    await mcpService.addServers(newServers)
    await loadServers()
  }

  async function deleteServer(index: number) {
    const serverToDelete = servers.value[index]
    console.log('[MCP Store] 删除服务器:', serverToDelete?.name, 'index:', index)
    console.log('[MCP Store] 删除前服务器数量:', servers.value.length)

    const newList = servers.value.filter((_, i) => i !== index)
    console.log('[MCP Store] 删除后服务器数量:', newList.length)
    console.log('[MCP Store] 新列表:', newList.map(s => s.name))

    await saveServers(newList)
  }

  async function updateServer(index: number, server: MCPServer) {
    const newList = [...servers.value]
    newList[index] = server
    await saveServers(newList)
  }

  async function addServer(server: MCPServer) {
    const newList = [...servers.value, server]
    await saveServers(newList)
  }

  function getTestResult(serverName: string): MCPTestResult | undefined {
    return testResults.value.get(serverName)
  }

  function clearTestResults() {
    testResults.value.clear()
  }

  return {
    // State
    servers,
    testResults,
    isLoading,
    isTestingAll,

    // Getters
    serverCount,
    availableCount,
    failedCount,
    testedCount,
    testStatusText,

    // Actions
    loadServers,
    saveServers,
    testServer,
    testAllServers,
    importFromJSON,
    addServers,
    deleteServer,
    updateServer,
    addServer,
    getTestResult,
    clearTestResults
  }
})
