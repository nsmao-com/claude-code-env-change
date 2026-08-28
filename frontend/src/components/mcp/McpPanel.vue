<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" :tool-filter="embedded" :close-on-overlay="false">
    <template #header>
      <div class="flex items-center gap-3">
        <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">MCP</h1>
        <McpStatusBadge />
      </div>
      <p class="mt-2 text-sm text-muted-foreground">刷新会检查 Claude / Codex / Gemini / OpenCode / Grok 配置里是否已有这些服务器</p>
    </template>

    <div class="flex h-full min-h-0 flex-1 flex-col overflow-hidden">
    <div class="mb-4 flex shrink-0 flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <Button size="sm" @click="showAddModal">
          <Plus />
          添加
        </Button>
        <Button size="sm" variant="outline" @click="showJsonImport = true">
          <FileJson />
          JSON 导入
        </Button>
        <Button size="sm" variant="outline" @click="toggleMarket">
          <Store />
          市场
        </Button>
        <ApplyToPlatformMenu
          :disabled="mcpStore.servers.length === 0"
          :applying="isApplying"
          @apply="applyToPlatform"
        />
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

    <div v-if="showMarket" class="mb-4 flex max-h-[36vh] min-h-0 shrink-0 flex-col overflow-hidden rounded-xl border border-dashed border-border bg-secondary/20 p-4">
      <div class="mb-3 flex shrink-0 flex-wrap items-center justify-between gap-3">
        <span class="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">MCP 在线市场</span>
        <Input v-model="marketQuery" class="w-[220px]" placeholder="搜索官方 Registry" />
      </div>
      <p class="mb-3 shrink-0 text-[10px] text-muted-foreground">
        来自 registry.modelcontextprotocol.io，导入会写入当前筛选的平台
        <span v-if="marketWarning"> · {{ marketWarning }}</span>
      </p>
      <Empty v-if="marketItems.length === 0 && !marketLoading" class="min-h-0 border-0 py-3">
        <EmptyHeader>
          <EmptyTitle>{{ marketError || '暂无结果' }}</EmptyTitle>
        </EmptyHeader>
      </Empty>
      <div v-else-if="marketLoading && marketItems.length === 0" class="flex justify-center py-6">
        <Loader2 class="size-5 animate-spin text-muted-foreground" />
      </div>
      <div v-else class="min-h-0 flex-1 overflow-y-auto pr-1">
        <div class="grid grid-cols-2 gap-3">
          <Card v-for="item in marketItems" :key="item.id" size="sm">
            <CardHeader>
              <div class="flex min-w-0 items-start justify-between gap-2">
                <div class="min-w-0 flex-1 overflow-hidden">
                  <AppTooltip :content="item.title || item.name" wrap class="min-w-0">
                    <CardTitle>{{ item.title || item.name }}</CardTitle>
                  </AppTooltip>
                  <CardDescription class="line-clamp-2">{{ item.description || item.hint || item.id }}</CardDescription>
                </div>
                <Button variant="outline" size="sm" class="shrink-0" :disabled="importingId === item.id" @click="importMarketItem(item)">
                  <Loader2 v-if="importingId === item.id" class="animate-spin" />
                  <Download v-else />
                  导入
                </Button>
              </div>
            </CardHeader>
          </Card>
        </div>
        <div v-if="marketNext" class="mt-3 flex justify-center">
          <Button variant="outline" size="sm" :disabled="marketLoading" @click="loadMarket(true)">
            {{ marketLoading ? '加载中...' : '更多' }}
          </Button>
        </div>
      </div>
    </div>

    <Empty
      v-if="filteredServerItems.length === 0 && !mcpStore.isLoading"
      class="min-h-0 flex-1"
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

    <div v-else-if="mcpStore.isLoading" class="flex min-h-0 flex-1 items-center justify-center">
      <Loader2 class="size-8 animate-spin text-muted-foreground" />
    </div>

    <div v-else class="min-h-0 flex-1 overflow-y-auto pr-2">
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
    </div>

    </div>

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
  Download,
  FileJson,
  LayoutGrid,
  List,
  Loader2,
  Plus,
  RefreshCw,
  RotateCw,
  Server,
  Store,
  Zap,
} from '@lucide/vue'
import type { MCPServer, McpMarketItem } from '@/types'
import { useMcpStore } from '@/stores/mcpStore'
import { mcpService } from '@/services/mcpService'
import { useConfigStore } from '@/stores/configStore'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import ApplyToPlatformMenu from '@/components/common/ApplyToPlatformMenu.vue'
import { Button } from '@/components/ui/button'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
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
const isApplying = ref(false)
const showMarket = ref(false)
const marketQuery = ref('')
const marketItems = ref<McpMarketItem[]>([])
const marketNext = ref('')
const marketWarning = ref('')
const marketError = ref('')
const marketLoading = ref(false)
const importingId = ref('')

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

let marketTimer: number | undefined
watch(marketQuery, () => {
  if (!showMarket.value) return
  window.clearTimeout(marketTimer)
  marketTimer = window.setTimeout(() => loadMarket(false), 350)
})

function toggleMarket() {
  showMarket.value = !showMarket.value
  if (showMarket.value && marketItems.value.length === 0) loadMarket(false)
}

function importPlatforms(): string[] {
  if (currentPlatform.value === 'all') return ['claude-code', 'codex', 'gemini', 'opencode', 'grok']
  return [currentPlatform.value]
}

async function loadMarket(more: boolean) {
  marketLoading.value = true
  marketError.value = ''
  try {
    const page = await mcpService.searchMarketplace(marketQuery.value.trim(), more ? marketNext.value : '')
    marketItems.value = more ? [...marketItems.value, ...(page.items || [])] : (page.items || [])
    marketNext.value = page.next || ''
    marketWarning.value = page.warning || ''
  } catch (e: any) {
    if (!more) marketItems.value = []
    marketError.value = e?.message || '加载市场失败'
  } finally {
    marketLoading.value = false
  }
}

async function importMarketItem(item: McpMarketItem) {
  importingId.value = item.id
  try {
    await mcpService.importMarketplace(item.id, importPlatforms())
    await mcpStore.loadServers()
    toast.success(`已导入 ${item.title || item.name}`)
  } catch (e: any) {
    toast.error('导入失败: ' + (e?.message || String(e)))
  } finally {
    importingId.value = ''
  }
}

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
    toast.success('已检查五个平台配置里是否存在这些 MCP')
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

async function applyToPlatform(platform: string) {
  isApplying.value = true
  try {
    const added = await mcpStore.applyToPlatform(platform)
    const label = platformLabel(platform)
    if (added > 0) toast.success(`已把 ${added} 个 MCP 加入 ${label}`)
    else toast.success(`已经都在 ${label} 里了`)
  } catch (e: any) {
    toast.error('加入失败: ' + (e?.message || String(e)))
  } finally {
    isApplying.value = false
  }
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
