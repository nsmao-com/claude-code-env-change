<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" :tool-filter="embedded" width="wide" :close-on-overlay="false">
    <template #header>
      <div class="flex items-center gap-3">
        <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">MCP</h1>
        <McpStatusBadge />
      </div>
      <p class="mt-2 text-sm text-muted-foreground">{{ t('mcp.panelHint') }}</p>
    </template>

    <div class="flex h-full min-h-0 flex-1 flex-col overflow-hidden">
    <div class="mb-4 flex shrink-0 flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <Button size="sm" @click="showAddModal">
          <Plus />
          {{ t('mcp.add') }}
        </Button>
        <Button size="sm" variant="outline" @click="showJsonImport = true">
          <FileJson />
          {{ t('mcp.jsonImport') }}
        </Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="mcpStore.servers.length === 0"
          @click="showExportModal = true"
        >
          <FileDown />
          {{ t('mcp.export') }}
        </Button>
        <Button size="sm" variant="outline" @click="toggleMarket">
          <Store />
          {{ t('mcp.market') }}
        </Button>
        <ApplyToPlatformMenu
          :items="MCP_PLATFORM_ITEMS"
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
          :items="[{ value: 'list', label: t('mcp.viewList') }, { value: 'cards', label: t('mcp.viewCards') }]"
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
          {{ isRefreshing ? t('mcp.refreshing') : t('mcp.refresh') }}
        </Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="isSyncing"
          @click="syncToPlatforms"
        >
          <Loader2 v-if="isSyncing" class="animate-spin" />
          <RotateCw v-else />
          {{ isSyncing ? t('mcp.syncing') : t('mcp.syncToPlatforms') }}
        </Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="mcpStore.isTestingAll || mcpStore.servers.length === 0"
          @click="testAll"
        >
          <Loader2 v-if="mcpStore.isTestingAll" class="animate-spin" />
          <Zap v-else />
          {{ mcpStore.isTestingAll ? t('mcp.testing') : t('mcp.testAll') }}
        </Button>
      </div>
    </div>

    <div v-if="showMarket" class="mb-4 flex max-h-[36vh] min-h-0 shrink-0 flex-col overflow-hidden rounded-xl border border-dashed border-border bg-secondary/20 p-4">
      <div class="mb-3 flex shrink-0 flex-wrap items-center justify-between gap-3">
        <span class="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">{{ t('mcp.marketTitle') }}</span>
        <Input v-model="marketQuery" class="w-[220px]" :placeholder="t('mcp.marketSearch')" />
      </div>
      <p class="mb-3 shrink-0 text-[10px] text-muted-foreground">
        {{ t('mcp.marketSource') }}
        <span v-if="marketWarning"> · {{ marketWarning }}</span>
      </p>
      <Empty v-if="marketItems.length === 0 && !marketLoading" class="min-h-0 border-0 py-3">
        <EmptyHeader>
          <EmptyTitle>{{ marketError || t('mcp.noResults') }}</EmptyTitle>
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
                  {{ t('mcp.import') }}
                </Button>
              </div>
            </CardHeader>
          </Card>
        </div>
        <div v-if="marketNext" class="mt-3 flex justify-center">
          <Button variant="outline" size="sm" :disabled="marketLoading" @click="loadMarket(true)">
            {{ marketLoading ? t('mcp.loading') : t('mcp.more') }}
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
        <EmptyTitle>{{ mcpStore.servers.length === 0 ? t('mcp.emptyAll') : t('mcp.emptyPlatform') }}</EmptyTitle>
        <EmptyDescription>{{ t('mcp.emptyDesc') }}</EmptyDescription>
      </EmptyHeader>
      <EmptyContent>
        <Button size="sm" @click="showAddModal">
          <Plus />
          {{ t('mcp.add') }}
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

    <McpExportModal
      v-model="showExportModal"
      :servers="mcpStore.servers"
    />
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import {
  Download,
  FileDown,
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
import { MCP_PLATFORM_ITEMS } from '@/lib/platforms'
import { Button } from '@/components/ui/button'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import McpStatusBadge from './McpStatusBadge.vue'
import McpServerCard from './McpServerCard.vue'
import McpEditModal from './McpEditModal.vue'
import McpJsonImport from './McpJsonImport.vue'
import McpExportModal from './McpExportModal.vue'

const { t } = useI18n()

type PlatformFilter = 'all' | 'claude-code' | 'claude-desktop' | 'codex' | 'antigravity' | 'opencode' | 'grok'

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
const showExportModal = ref(false)
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
  else if (tool === 'claude_desktop') currentPlatform.value = 'claude-desktop'
  else if (tool === 'codex' || tool === 'antigravity' || tool === 'opencode' || tool === 'grok') currentPlatform.value = tool
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
// 面板随页面切换销毁，防抖回调可能在卸载后触发 loadMarket
onBeforeUnmount(() => window.clearTimeout(marketTimer))

function toggleMarket() {
  showMarket.value = !showMarket.value
  if (showMarket.value && marketItems.value.length === 0) loadMarket(false)
}

function importPlatforms(): string[] {
  if (currentPlatform.value === 'all') return ['claude-code', 'codex', 'antigravity', 'opencode', 'grok']
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
    marketError.value = e?.message || t('mcp.marketLoadFailed')
  } finally {
    marketLoading.value = false
  }
}

async function importMarketItem(item: McpMarketItem) {
  importingId.value = item.id
  try {
    await mcpService.importMarketplace(item.id, importPlatforms())
    await mcpStore.loadServers()
    toast.success(t('mcp.imported', { name: item.title || item.name }))
  } catch (e: any) {
    toast.error(t('mcp.importFailed', { error: e?.message || String(e) }))
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
  if (!server) return
  const key = server.name

  const confirmed = await confirm.show(
    t('mcp.deleteTitle'),
    t('mcp.deleteMsg', { name: server.name }),
    'danger'
  )
  if (!confirmed) return

  try {
    await mcpStore.deleteServerByKey(key)
    toast.success(t('mcp.deleted'))
  } catch (e: any) {
    toast.error(t('mcp.deleteFailed', { error: e?.message || String(e) }))
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
    toast.error(t('mcp.testFailed', { error: e?.message || String(e) }))
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
    toast.success(t('mcp.refreshed'))
  } catch (e: any) {
    toast.error(t('mcp.refreshFailed', { error: e?.message || String(e) }))
  } finally {
    isRefreshing.value = false
  }
}

async function syncToPlatforms() {
  isSyncing.value = true
  try {
    await mcpStore.syncToPlatforms()
    toast.success(t('mcp.synced'))
  } catch (e: any) {
    toast.error(t('mcp.syncFailed', { error: e?.message || String(e) }))
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
    if (added > 0) toast.success(t('mcp.appliedCount', { count: added, platform: label }))
    else toast.success(t('mcp.alreadyIn', { platform: label }))
  } catch (e: any) {
    toast.error(t('mcp.applyFailed', { error: e?.message || String(e) }))
  } finally {
    isApplying.value = false
  }
}

async function togglePlatform(server: MCPServer, platform: string) {
  if (server.missing_placeholders?.length) {
    toast.error(t('mcp.fillPlaceholders'))
    return
  }
  try {
    await mcpStore.togglePlatform(server.name, platform)
    const on = (mcpStore.servers.find(item => item.name === server.name)?.enable_platform || []).includes(platform)
    toast.success(on ? t('mcp.addedTo', { platform: platformLabel(platform) }) : t('mcp.removedFrom', { platform: platformLabel(platform) }))
  } catch (e: any) {
    toast.error(t('mcp.toggleFailed', { error: e?.message || String(e) }))
  }
}

function platformLabel(platform: string) {
  if (platform === 'claude-code') return 'Claude Code'
  if (platform === 'claude-desktop') return 'Claude Desktop'
  if (platform === 'codex') return 'Codex'
  if (platform === 'antigravity') return 'Antigravity'
  if (platform === 'opencode') return 'OpenCode'
  if (platform === 'grok') return 'Grok'
  return platform
}
</script>
