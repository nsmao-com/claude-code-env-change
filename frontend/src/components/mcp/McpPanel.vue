<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" :tool-filter="embedded" :close-on-overlay="false">
    <template #header>
      <div class="flex items-center gap-3">
        <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">MCP</h1>
        <McpStatusBadge />
      </div>
      <p class="mt-2 text-sm text-muted-foreground">管理 Model Context Protocol 服务器</p>
    </template>

    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <Button size="sm" @click="showAddModal">
          <Plus />
          添加
        </Button>
        <Button size="sm" variant="outline" @click="showJsonImport = true">
          <FileJson />
          JSON 导入
        </Button>
      </div>
      <div class="flex items-center gap-2">
        <SegmentedPills
          :model-value="viewMode"
          layout-id="mcp-view-pill"
          dense
          :items="[{ value: 'list', label: '列表' }, { value: 'cards', label: '卡片' }]"
          @update:model-value="onView"
        >
          <template #default="{ item }">
            <List v-if="item.value === 'list'" class="size-3.5" />
            <LayoutGrid v-else class="size-3.5" />
          </template>
        </SegmentedPills>
        <Button
          size="sm"
          variant="outline"
          :disabled="isRefreshing"
          @click="refreshServers"
        >
          <Loader2 v-if="isRefreshing" class="animate-spin" />
          <RefreshCw v-else />
          {{ isRefreshing ? '刷新中...' : '刷新' }}
        </Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="isSyncing"
          @click="syncToPlatforms"
        >
          <Loader2 v-if="isSyncing" class="animate-spin" />
          <RotateCw v-else />
          {{ isSyncing ? '同步中...' : '同步到平台' }}
        </Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="mcpStore.isTestingAll || mcpStore.servers.length === 0"
          @click="testAll"
        >
          <Loader2 v-if="mcpStore.isTestingAll" class="animate-spin" />
          <Zap v-else />
          {{ mcpStore.isTestingAll ? '检测中...' : '全部检测' }}
        </Button>
      </div>
    </div>

    <Empty
      v-if="filteredServerItems.length === 0 && !mcpStore.isLoading"
      class="py-12"
    >
      <EmptyHeader>
        <Server class="size-10 text-muted-foreground" />
        <EmptyTitle>{{ mcpStore.servers.length === 0 ? '暂无 MCP 服务器' : '该平台暂无 MCP 服务器' }}</EmptyTitle>
        <EmptyDescription>点击「添加」或「JSON 导入」添加服务器</EmptyDescription>
      </EmptyHeader>
      <EmptyContent>
        <Button size="sm" @click="showAddModal">
          <Plus />
          添加
        </Button>
      </EmptyContent>
    </Empty>

    <div v-else-if="mcpStore.isLoading" class="flex items-center justify-center py-12">
      <Loader2 class="size-8 animate-spin text-muted-foreground" />
    </div>

    <ScrollArea v-else class="h-[50vh] pr-2">
      <div :class="viewMode === 'cards' ? 'grid grid-cols-2 gap-3' : 'flex flex-col gap-2.5'">
        <McpServerCard
          v-for="item in filteredServerItems"
          :key="`${item.server.name}-${item.index}`"
          :server="item.server"
          :test-result="mcpStore.getTestResult(item.server.name)"
          :is-testing="testingIndex === item.index"
          :compact="viewMode === 'list'"
          @test="testSingle(item.index)"
          @edit="editServer(item.index)"
          @delete="deleteServer(item.index)"
          @toggle-platform="togglePlatform(item.server, $event)"
        />
      </div>
    </ScrollArea>

    <McpEditModal
      v-model="showEditModal"
      :edit-server="editingServer"
      :edit-index="editingIndex"
      @saved="onServerSaved"
    />

    <McpJsonImport
      v-model="showJsonImport"
      @imported="onServersImported"
    />
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import {
  FileJson,
  LayoutGrid,
  List,
  Loader2,
  Plus,
  RefreshCw,
  RotateCw,
  Server,
  Zap,
} from '@lucide/vue'
import type { MCPServer } from '@/types'
import { useMcpStore } from '@/stores/mcpStore'
import { useConfigStore } from '@/stores/configStore'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import { Button } from '@/components/ui/button'

import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { ScrollArea } from '@/components/ui/scroll-area'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import McpStatusBadge from './McpStatusBadge.vue'
import McpServerCard from './McpServerCard.vue'
import McpEditModal from './McpEditModal.vue'
import McpJsonImport from './McpJsonImport.vue'

type PlatformFilter = 'all' | 'claude-code' | 'codex' | 'gemini' | 'opencode' | 'grok'

interface Props {
  modelValue: boolean
  embedded?: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const mcpStore = useMcpStore()
const configStore = useConfigStore()
const confirm = useConfirm()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const showEditModal = ref(false)
const showJsonImport = ref(false)
const editingServer = ref<MCPServer | null>(null)
const editingIndex = ref<number | undefined>(undefined)
const testingIndex = ref<number | null>(null)
const isRefreshing = ref(false)
const isSyncing = ref(false)

type ViewMode = 'cards' | 'list'
const viewMode = ref<ViewMode>('list')
const viewModeStorageKey = 'claudia_mcp_view_mode'

function setViewMode(mode: ViewMode) {
  viewMode.value = mode
  try {
    localStorage.setItem(viewModeStorageKey, mode)
  } catch {}
}

const currentPlatform = ref<PlatformFilter>('all')

const filteredServerItems = computed(() => {
  const withIndex = mcpStore.servers.map((server, index) => ({ server, index }))
  if (currentPlatform.value === 'all') {
    return withIndex
  }
  return withIndex.filter(item => item.server.enable_platform?.includes(currentPlatform.value))
})

function syncTool(tool: string) {
  if (tool === 'claude') currentPlatform.value = 'claude-code'
  else if (tool === 'codex' || tool === 'gemini' || tool === 'opencode' || tool === 'grok') currentPlatform.value = tool
  else currentPlatform.value = 'all'
}

function onView(value: string) {
  if (value === 'cards' || value === 'list') {
    setViewMode(value)
  }
}

watch(isOpen, async (open) => {
  if (open) {
    try {
      const saved = localStorage.getItem(viewModeStorageKey)
      if (saved === 'cards' || saved === 'list') viewMode.value = saved
    } catch {}

    await mcpStore.loadServers()
    if (mcpStore.servers.length > 0) {
      mcpStore.testAllServers()
    }
  } else {
    mcpStore.clearTestResults()
  }}, { immediate: true })

watch(() => configStore.currentFilter, (tool) => syncTool(tool), { immediate: true })

function showAddModal() {
  editingServer.value = null
  editingIndex.value = undefined
  showEditModal.value = true
}

function cloneServerForEdit(server: MCPServer): MCPServer {
  return {
    ...server,
    args: [...(server.args || [])],
    env: { ...(server.env || {}) },
    headers: { ...(server.headers || {}) },
    enable_platform: [...(server.enable_platform || [])],
    missing_placeholders: [...(server.missing_placeholders || [])]
  }
}

function editServer(index: number) {
  const server = mcpStore.servers[index]
  if (!server) return
  editingServer.value = cloneServerForEdit(server)
  editingIndex.value = index
  showEditModal.value = true
}

async function deleteServer(index: number) {
  const server = mcpStore.servers[index]

  const confirmed = await confirm.show(
    '删除 MCP 服务器',
    `确定要删除 "${server.name}" 吗？`,
    'danger'
  )
  if (!confirmed) return

  try {
    await mcpStore.deleteServer(index)
    toast.success('MCP 服务器已删除')
  } catch (e: any) {
    toast.error('删除失败: ' + e.message)
  }
}

async function testSingle(index: number) {
  testingIndex.value = index
  try {
    const server = mcpStore.servers[index]
    const result = await mcpStore.testServer(server)
    if (result.success) {
      toast.success(`${server.name}: ${result.message} (${result.latency}ms)`)
    } else {
      toast.error(`${server.name}: ${result.message}`)
    }
  } catch (e: any) {
    toast.error('测试失败: ' + e.message)
  } finally {
    testingIndex.value = null
  }
}

function testAll() {
  mcpStore.testAllServers()
}

async function refreshServers() {
  isRefreshing.value = true
  try {
    await mcpStore.loadServers()
    toast.success('MCP 服务器列表已刷新')
    if (mcpStore.servers.length > 0) {
      mcpStore.testAllServers()
    }
  } catch (e: any) {
    toast.error('刷新失败: ' + e.message)
  } finally {
    isRefreshing.value = false
  }
}

async function syncToPlatforms() {
  isSyncing.value = true
  try {
    await mcpStore.syncToPlatforms()
    toast.success('已重新同步到 Claude / Codex / Gemini / OpenCode / Grok')
  } catch (e: any) {
    toast.error('同步失败: ' + (e?.message || String(e)))
  } finally {
    isSyncing.value = false
  }
}

function onServerSaved() {
  mcpStore.loadServers()
}

function onServersImported() {
  mcpStore.loadServers().then(() => {
    mcpStore.testAllServers()
  })
}

async function togglePlatform(server: MCPServer, platform: string) {
  if (server.missing_placeholders?.length) {
    toast.error('请先补全占位符再启用平台')
    return
  }
  try {
    await mcpStore.togglePlatform(server.name, platform)
    const on = (mcpStore.servers.find(item => item.name === server.name)?.enable_platform || []).includes(platform)
    toast.success(on ? `已加入 ${platformLabel(platform)}` : `已从 ${platformLabel(platform)} 移除`)
  } catch (e: any) {
    toast.error('切换失败: ' + (e?.message || String(e)))
  }
}

function platformLabel(platform: string) {
  if (platform === 'claude-code') return 'Claude'
  if (platform === 'codex') return 'Codex'
  if (platform === 'gemini') return 'Gemini'
  if (platform === 'opencode') return 'OpenCode'
  if (platform === 'grok') return 'Grok'
  return platform
}
</script>
