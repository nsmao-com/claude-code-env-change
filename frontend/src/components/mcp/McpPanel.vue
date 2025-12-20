<template>
  <AppModal v-model="isOpen" size="xl" :close-on-overlay="false">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center">
          <i class="fas fa-server text-primary"></i>
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h3 class="text-lg font-semibold">MCP 服务器</h3>
            <McpStatusBadge />
          </div>
          <p class="text-xs text-muted-foreground">管理 Model Context Protocol 服务器</p>
        </div>
      </div>
    </template>

    <!-- Platform Filter Tabs -->
    <div class="relative mb-4 p-1 bg-secondary/50 border border-border/50 rounded-full inline-flex backdrop-blur-sm">
      <!-- Glider -->
      <div
        class="absolute top-1 bottom-1 bg-white rounded-full transition-all duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] shadow-[0_2px_8px_rgba(0,0,0,0.08)] border border-black/5 dark:border-white/10"
        :style="gliderStyle"
      ></div>

      <button
        v-for="tab in platformTabs"
        :key="tab.value"
        ref="tabRefs"
        :class="['relative z-10 px-4 py-1.5 rounded-full text-xs font-bold uppercase tracking-wide transition-colors duration-200', { 'text-foreground': currentPlatform === tab.value, 'text-muted-foreground hover:text-foreground/80': currentPlatform !== tab.value }]"
        @click="setFilter(tab.value)"
      >
        {{ tab.label }}
        <span v-if="tab.count > 0" class="ml-1 text-[10px] opacity-70">({{ tab.count }})</span>
      </button>
    </div>

    <!-- Toolbar -->
    <div class="flex items-center justify-between mb-4">
      <div class="flex gap-2">
        <button class="btn btn-primary btn-sm" @click="showAddModal">
          <i class="fas fa-plus mr-2"></i>
          添加
        </button>
        <button class="btn btn-outline btn-sm" @click="showJsonImport = true">
          <i class="fas fa-file-import mr-2"></i>
          JSON 导入
        </button>
      </div>
      <button
        class="btn btn-outline btn-sm"
        :disabled="mcpStore.isTestingAll || mcpStore.servers.length === 0"
        @click="testAll"
      >
        <i :class="['fas mr-2', mcpStore.isTestingAll ? 'fa-circle-notch fa-spin' : 'fa-bolt']"></i>
        {{ mcpStore.isTestingAll ? '检测中...' : '全部检测' }}
      </button>
    </div>

    <!-- Empty State -->
    <div
      v-if="filteredServers.length === 0 && !mcpStore.isLoading"
      class="flex flex-col items-center justify-center py-12 text-muted-foreground"
    >
      <i class="fas fa-server text-4xl mb-4"></i>
      <p class="text-sm">{{ mcpStore.servers.length === 0 ? '暂无 MCP 服务器' : '该平台暂无 MCP 服务器' }}</p>
      <p class="text-xs">点击「添加」或「JSON 导入」添加服务器</p>
    </div>

    <!-- Loading -->
    <div v-else-if="mcpStore.isLoading" class="flex items-center justify-center py-12">
      <i class="fas fa-circle-notch fa-spin text-2xl text-muted-foreground"></i>
    </div>

    <!-- Server List -->
    <div v-else class="space-y-3 max-h-[50vh] overflow-y-auto pr-2">
      <McpServerCard
        v-for="server in filteredServers"
        :key="server.name"
        :server="server"
        :test-result="mcpStore.getTestResult(server.name)"
        :is-testing="testingIndex === getOriginalIndex(server.name)"
        @test="testSingle(getOriginalIndex(server.name))"
        @edit="editServer(getOriginalIndex(server.name))"
        @delete="deleteServer(getOriginalIndex(server.name))"
      />
    </div>

    <!-- Edit Modal -->
    <McpEditModal
      v-model="showEditModal"
      :edit-server="editingServer"
      :edit-index="editingIndex"
      @saved="onServerSaved"
    />

    <!-- JSON Import Modal -->
    <McpJsonImport
      v-model="showJsonImport"
      @imported="onServersImported"
    />
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import type { MCPServer } from '@/types'
import { useMcpStore } from '@/stores/mcpStore'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import McpStatusBadge from './McpStatusBadge.vue'
import McpServerCard from './McpServerCard.vue'
import McpEditModal from './McpEditModal.vue'
import McpJsonImport from './McpJsonImport.vue'

type PlatformFilter = 'all' | 'claude-code' | 'codex' | 'gemini'

interface Props {
  modelValue: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const mcpStore = useMcpStore()
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

// Platform filter
const currentPlatform = ref<PlatformFilter>('all')
const tabRefs = ref<HTMLElement[]>([])
const gliderStyle = ref({ left: '4px', width: '0px' })

// Platform tabs with counts
const platformTabs = computed(() => [
  { label: '全部', value: 'all' as PlatformFilter, count: mcpStore.servers.length },
  { label: 'Claude', value: 'claude-code' as PlatformFilter, count: mcpStore.servers.filter(s => s.enable_platform?.includes('claude-code')).length },
  { label: 'Codex', value: 'codex' as PlatformFilter, count: mcpStore.servers.filter(s => s.enable_platform?.includes('codex')).length },
  { label: 'Gemini', value: 'gemini' as PlatformFilter, count: mcpStore.servers.filter(s => s.enable_platform?.includes('gemini')).length }
])

// Filter servers by platform
const filteredServers = computed(() => {
  if (currentPlatform.value === 'all') {
    return mcpStore.servers
  }
  return mcpStore.servers.filter(s => s.enable_platform?.includes(currentPlatform.value))
})

// Get original index from server name
function getOriginalIndex(name: string): number {
  return mcpStore.servers.findIndex(s => s.name === name)
}

// Update glider position
function updateGlider() {
  nextTick(() => {
    const activeIndex = platformTabs.value.findIndex(t => t.value === currentPlatform.value)
    if (activeIndex !== -1 && tabRefs.value[activeIndex]) {
      const el = tabRefs.value[activeIndex]
      gliderStyle.value = {
        left: `${el.offsetLeft}px`,
        width: `${el.offsetWidth}px`
      }
    }
  })
}

function setFilter(platform: PlatformFilter) {
  currentPlatform.value = platform
  updateGlider()
}

// Watch filter change to update glider
watch(currentPlatform, () => {
  updateGlider()
})

// 打开弹窗时加载服务器并自动检测
watch(isOpen, async (open) => {
  if (open) {
    await mcpStore.loadServers()
    // 自动检测所有服务器
    if (mcpStore.servers.length > 0) {
      mcpStore.testAllServers()
    }
    // Update glider after data loaded
    setTimeout(updateGlider, 100)
  } else {
    // 关闭时清除测试结果
    mcpStore.clearTestResults()
    currentPlatform.value = 'all'
  }
})

function showAddModal() {
  editingServer.value = null
  editingIndex.value = undefined
  showEditModal.value = true
}

function editServer(index: number) {
  editingServer.value = mcpStore.servers[index]
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

function onServerSaved() {
  // 保存后重新加载
  mcpStore.loadServers()
}

function onServersImported() {
  // 导入后重新加载并检测
  mcpStore.loadServers().then(() => {
    mcpStore.testAllServers()
  })
}
</script>

<style scoped>
.btn-sm {
  @apply h-8 px-3 text-xs;
}
</style>
