<template>
  <UiSidebar collapsible="none" class="h-full border-r border-sidebar-border">
    <SidebarContent class="pt-3">
      <SidebarGroup>
        <SidebarGroupContent>
          <SidebarMenu>
            <SidebarMenuItem v-for="item in pages" :key="item.id">
              <SidebarMenuButton :is-active="page === item.id" @click="$emit('navigate', item.id)">
                <component :is="item.icon" />
                <span>{{ item.label }}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>
    </SidebarContent>

    <SidebarFooter class="gap-2 px-3 pb-4">
      <div class="flex items-center justify-between gap-2 px-1">
        <span class="text-xs text-muted-foreground">延迟</span>
        <Button variant="ghost" size="icon-xs" @click="testAllLatency">
          <RefreshCw :class="['size-3.5', isTesting && 'animate-spin']" />
        </Button>
      </div>
      <div v-if="!hasAnyConfig && !isTesting" class="px-1 text-xs text-muted-foreground">未检测</div>
      <div v-else class="space-y-1 px-1">
        <div v-for="item in latencyItems" :key="item.provider" class="flex items-center justify-between gap-2 text-xs">
          <span class="flex items-center gap-1.5">
            <BrandIcon :provider="item.provider" class="size-3 text-muted-foreground" />
            {{ item.name }}
          </span>
          <span class="font-mono text-muted-foreground">{{ item.loading ? '…' : item.display }}</span>
        </div>
      </div>
      <div class="px-1 text-[11px] text-muted-foreground">v{{ appVersion }}</div>
    </SidebarFooter>
  </UiSidebar>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, type Component } from 'vue'
import {
  Book,
  ChartLine,
  Cloud,
  FileText,
  HeartPulse,
  Layers,
  RefreshCw,
  Route,
  Server,
} from '@lucide/vue'
import type { AppPage } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import { configService } from '@/services/configService'
import { updateService } from '@/services/updateService'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Button } from '@/components/ui/button'
import {
  Sidebar as UiSidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar'

withDefaults(defineProps<{
  page?: AppPage
}>(), {
  page: 'env',
})

defineEmits<{
  navigate: [page: AppPage]
}>()

const configStore = useConfigStore()

const pages: { id: AppPage; label: string; icon: Component }[] = [
  { id: 'env', label: '环境', icon: Layers },
  { id: 'mcp', label: 'MCP', icon: Server },
  { id: 'skills', label: 'Skills', icon: Book },
  { id: 'router', label: 'API 路由', icon: Route },
  { id: 'uptime', label: '监控轮换', icon: HeartPulse },
  { id: 'cloud', label: '云同步', icon: Cloud },
  { id: 'prompts', label: '提示词规则', icon: FileText },
  { id: 'stats', label: '使用统计', icon: ChartLine },
]

const appVersion = ref('2.0.0')
onMounted(async () => {
  try {
    appVersion.value = await updateService.version()
  } catch {
    /* ignore */
  }
})

interface LatencyResult {
  provider: string
  name: string
  url: string
  ms: number
  loading: boolean
  display: string
}

const isTesting = ref(false)
const latencyResults = ref<Map<string, LatencyResult>>(new Map())
const hasAnyConfig = computed(() => latencyResults.value.size > 0)
const latencyItems = computed(() => {
  const all = Array.from(latencyResults.value.values())
  const tool = configStore.currentFilter
  if (tool === 'all') return all
  return all.filter(item => item.provider === tool)
})

function formatMs(ms: number) {
  if (ms === -1) return 'N/A'
  if (ms > 1000) return `${(ms / 1000).toFixed(1)}s`
  return `${ms}ms`
}

async function testAllLatency() {
  isTesting.value = true
  latencyResults.value.clear()
  const configs: { provider: string; name: string; url: string }[] = []
  try {
    const s = await configService.getClaudeSettings()
    if (s?.['ANTHROPIC_BASE_URL']) configs.push({ provider: 'claude', name: 'Claude', url: s['ANTHROPIC_BASE_URL'] })
  } catch { /* ignore */ }
  try {
    const s = await configService.getCodexSettings()
    if (s?.['base_url']) configs.push({ provider: 'codex', name: 'Codex', url: s['base_url'] })
  } catch { /* ignore */ }
  try {
    const s = await configService.getAntigravitySettings()
    if (s?.['GOOGLE_GEMINI_BASE_URL']) configs.push({ provider: 'antigravity', name: 'Antigravity', url: s['GOOGLE_GEMINI_BASE_URL'] })
  } catch { /* ignore */ }
  try {
    const s = await configService.getOpencodeSettings()
    if (s?.['OPENCODE_BASE_URL']) configs.push({ provider: 'opencode', name: 'OpenCode', url: s['OPENCODE_BASE_URL'] })
  } catch { /* ignore */ }

  if (configs.length === 0) {
    isTesting.value = false
    return
  }
  for (const config of configs) {
    latencyResults.value.set(config.provider, { ...config, ms: -1, loading: true, display: '' })
  }
  await Promise.all(configs.map(async (config) => {
    try {
      const ms = await configStore.testLatency(config.url)
      latencyResults.value.set(config.provider, { ...config, ms, loading: false, display: formatMs(ms) })
    } catch {
      latencyResults.value.set(config.provider, { ...config, ms: -1, loading: false, display: 'Error' })
    }
  }))
  isTesting.value = false
}
</script>
