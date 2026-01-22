<template>
  <div class="bento-item area-sidebar flex flex-col h-full border-r border-border">
    <!-- Logo & Title -->
    <div class="flex-none">
      <div class="flex items-center gap-4 mb-8 px-2 pt-2">
        <div class="icon-badge border border-border">
          <i class="fas fa-terminal text-xl"></i>
        </div>
        <div>
          <h1 class="text-xl font-bold tracking-tight text-foreground uppercase">Claude Code</h1>
          <p class="text-xs text-muted-foreground font-medium tracking-wide">环境管理器</p>
        </div>
      </div>

      <!-- Action Buttons -->
      <div class="space-y-3 px-1">
        <button class="btn btn-primary w-full gap-3 h-10 text-sm shadow-none hover:shadow-none" @click="$emit('add')">
          <i class="fas fa-plus"></i>
          新建配置
        </button>
        
        <div class="grid grid-cols-2 gap-3">
          <button class="btn btn-secondary w-full gap-2 h-10 text-xs font-medium" @click="$emit('openMcp')">
            <i class="fas fa-server"></i>
            MCP
          </button>
          <button class="btn btn-secondary w-full gap-2 h-10 text-xs font-medium" @click="$emit('openStats')">
            <i class="fas fa-chart-line"></i>
            统计
          </button>
        </div>

        <button class="btn btn-secondary w-full gap-2 h-10 text-xs font-medium" @click="$emit('openPrompts')">
          <i class="fas fa-file-alt"></i>
          提示词规则
        </button>

        <button class="btn btn-secondary w-full gap-2 h-10 text-xs font-medium" @click="$emit('openSkills')">
          <i class="fas fa-layer-group"></i>
          Skills
        </button>

        <button class="btn btn-secondary w-full gap-2 h-10 text-xs font-medium" @click="$emit('openUptime')">
          <i class="fas fa-heartbeat"></i>
          监控&轮换
        </button>

        <div class="flex gap-3 pt-1">
          <button class="btn btn-outline flex-1 gap-2 h-9 text-xs" @click="$emit('export')">
            <i class="fas fa-download opacity-70"></i>
            导出
          </button>
          <button class="btn btn-outline flex-1 gap-2 h-9 text-xs" @click="$emit('import')">
            <i class="fas fa-upload opacity-70"></i>
            导入
          </button>
        </div>
      </div>

      <!-- System Status -->
      <div class="mt-8 mx-1 p-4 rounded-lg border border-dashed border-border">
        <div class="flex items-center justify-between mb-3">
          <span class="text-[10px] font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-1.5">
            延迟状态
          </span>
          <button
            class="w-6 h-6 rounded hover:bg-muted flex items-center justify-center text-muted-foreground hover:text-foreground transition-all"
            @click="testAllLatency"
            title="刷新"
          >
            <i :class="['fas fa-sync-alt text-[10px]', { 'fa-spin': isTesting }]"></i>
          </button>
        </div>
        <div v-if="!hasAnyConfig && !isTesting" class="text-xs text-muted-foreground text-center py-4">
          点击刷新测试
        </div>
        <div v-else class="space-y-2.5">
          <div v-for="item in latencyItems" :key="item.provider" class="flex items-center justify-between group">
            <span class="text-xs font-bold text-foreground group-hover:underline decoration-1 underline-offset-2">{{ item.name }}</span>
            <span :class="['text-[11px] font-mono px-2 py-0.5 rounded border border-border', item.colorClass]">
              <i v-if="item.loading" class="fas fa-circle-notch fa-spin"></i>
              <template v-else>{{ item.display }}</template>
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Bottom Section -->
    <div class="mt-auto space-y-4 px-1 pb-2">
      <div class="h-px bg-border w-full"></div>
      
      <!-- Clear Dropdown -->
      <div class="relative" ref="dropdownRef">
        <button
          class="btn btn-ghost w-full justify-between h-10 text-xs font-medium hover:bg-destructive hover:text-destructive-foreground transition-colors group border border-transparent hover:border-destructive"
          @click="showClearMenu = !showClearMenu"
        >
          <span class="flex items-center gap-2.5">
            <i class="fas fa-eraser"></i>
            清除配置
          </span>
          <i :class="['fas fa-chevron-down text-[10px] transition-transform duration-200', { 'rotate-180': showClearMenu }]"></i>
        </button>
        <transition
          enter-active-class="transition duration-100 ease-out"
          enter-from-class="transform scale-95 opacity-0 translate-y-2"
          enter-to-class="transform scale-100 opacity-100 translate-y-0"
          leave-active-class="transition duration-75 ease-in"
          leave-from-class="transform scale-100 opacity-100 translate-y-0"
          leave-to-class="transform scale-95 opacity-0 translate-y-2"
        >
          <div v-if="showClearMenu" class="absolute bottom-12 left-0 w-full mb-2 z-[9999] bg-popover border border-border shadow-2xl rounded-xl overflow-hidden ring-1 ring-black/5">
            <button class="w-full text-left px-4 py-2.5 text-xs font-mono hover:bg-muted transition-colors border-b border-border flex items-center justify-between" @click="handleClear('claude')">
              <span>CLAUDE</span>
              <i class="fas fa-arrow-right text-[10px] opacity-0 group-hover:opacity-100"></i>
            </button>
            <button class="w-full text-left px-4 py-2.5 text-xs font-mono hover:bg-muted transition-colors border-b border-border" @click="handleClear('codex')">
              <span>CODEX</span>
            </button>
            <button class="w-full text-left px-4 py-2.5 text-xs font-mono hover:bg-muted transition-colors border-b border-border" @click="handleClear('gemini')">
              <span>GEMINI</span>
            </button>
            <button class="w-full text-left px-4 py-2.5 text-xs font-mono text-destructive hover:bg-destructive hover:text-destructive-foreground transition-colors font-bold" @click="handleClear('all')">
              <span>清除全部</span>
            </button>
          </div>
        </transition>
      </div>

      <!-- Theme Toggle -->
      <div class="flex items-center justify-between px-3 py-2 rounded-lg border border-border">
        <div class="flex items-center gap-2">
           <i :class="['fas text-xs text-foreground', isDark ? 'fa-moon' : 'fa-sun']"></i>
           <span class="text-xs font-bold text-foreground uppercase">深色模式</span>
        </div>
        <button
          class="theme-toggle"
          :class="{ active: isDark }"
          @click="toggleTheme"
          :aria-checked="isDark"
          role="switch"
        >
          <!-- Thumb is handled by CSS ::after now to match style.css -->
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useTheme } from '@/composables/useTheme'
import { useConfigStore } from '@/stores/configStore'
import { configService } from '@/services/configService'

const { toggle: toggleTheme, isDark } = useTheme()
const configStore = useConfigStore()

const emit = defineEmits<{
  add: []
  openMcp: []
  openStats: []
  openPrompts: []
  openSkills: []
  openUptime: []
  export: []
  import: []
  clearClaude: []
  clearCodex: []
  clearGemini: []
  clearAll: []
}>()

// Clear dropdown
const showClearMenu = ref(false)
const dropdownRef = ref<HTMLElement>()

function handleClear(type: 'claude' | 'codex' | 'gemini' | 'all') {
  showClearMenu.value = false
  if (type === 'claude') emit('clearClaude')
  else if (type === 'codex') emit('clearCodex')
  else if (type === 'gemini') emit('clearGemini')
  else emit('clearAll')
}

// Close dropdown on click outside
function handleClickOutside(e: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(e.target as Node)) {
    showClearMenu.value = false
  }
}

onMounted(() => document.addEventListener('click', handleClickOutside))
onUnmounted(() => document.removeEventListener('click', handleClickOutside))

// Latency testing
interface LatencyResult {
  provider: string
  name: string
  url: string
  ms: number
  loading: boolean
  display: string
  colorClass: string
}

const isTesting = ref(false)
const latencyResults = ref<Map<string, LatencyResult>>(new Map())

const hasAnyConfig = computed(() => latencyResults.value.size > 0)
const latencyItems = computed(() => Array.from(latencyResults.value.values()))

function getLatencyDisplay(ms: number): { display: string; colorClass: string } {
  if (ms === -1) return { display: 'N/A', colorClass: 'text-muted-foreground' }
  if (ms > 1000) return { display: `${(ms / 1000).toFixed(1)}s`, colorClass: 'text-red-500' }
  if (ms > 500) return { display: `${ms}ms`, colorClass: 'text-orange-500' }
  if (ms > 300) return { display: `${ms}ms`, colorClass: 'text-yellow-500' }
  return { display: `${ms}ms`, colorClass: 'text-green-500' }
}

async function testAllLatency() {
  isTesting.value = true
  latencyResults.value.clear()

  const configs: { provider: string; name: string; url: string }[] = []

  try {
    const claudeSettings = await configService.getClaudeSettings()
    if (claudeSettings?.['ANTHROPIC_BASE_URL']) {
      configs.push({ provider: 'claude', name: 'Claude', url: claudeSettings['ANTHROPIC_BASE_URL'] })
    }
  } catch { /* ignore */ }

  try {
    const codexSettings = await configService.getCodexSettings()
    if (codexSettings?.['base_url']) {
      configs.push({ provider: 'codex', name: 'Codex', url: codexSettings['base_url'] })
    }
  } catch { /* ignore */ }

  try {
    const geminiSettings = await configService.getGeminiSettings()
    if (geminiSettings?.['GOOGLE_GEMINI_BASE_URL']) {
      configs.push({ provider: 'gemini', name: 'Gemini', url: geminiSettings['GOOGLE_GEMINI_BASE_URL'] })
    }
  } catch { /* ignore */ }

  if (configs.length === 0) {
    isTesting.value = false
    return
  }

  // Initialize with loading state
  for (const config of configs) {
    latencyResults.value.set(config.provider, {
      ...config, ms: -1, loading: true, display: '', colorClass: 'text-muted-foreground'
    })
  }

  // Test each config
  for (const config of configs) {
    try {
      const ms = await configStore.testLatency(config.url)
      const displayInfo = getLatencyDisplay(ms)
      latencyResults.value.set(config.provider, { ...config, ms, loading: false, ...displayInfo })
    } catch {
      latencyResults.value.set(config.provider, {
        ...config, ms: -1, loading: false, display: 'Error', colorClass: 'text-red-500'
      })
    }
  }

  isTesting.value = false
}
</script>

<style scoped>
.clear-menu {
  position: absolute;
  bottom: 100%;
  left: 0;
  right: 0;
  margin-bottom: 4px;
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 4px;
  box-shadow: 0 -4px 12px rgba(0, 0, 0, 0.1);
  z-index: 10;
}

.clear-menu-item {
  display: flex;
  align-items: center;
  width: 100%;
  padding: 8px 12px;
  font-size: 12px;
  border-radius: 6px;
  transition: background-color 0.15s;
}

.clear-menu-item:hover {
  background: var(--muted);
}

/* Theme Toggle Switch */
.theme-toggle {
  position: relative;
  width: 36px;
  height: 20px;
  min-width: 36px;
  border-radius: 10px;
  background: var(--muted);
  border: none;
  cursor: pointer;
  transition: background-color 0.2s ease;
  box-sizing: border-box;
  padding: 0;
}

.theme-toggle.active {
  background: var(--primary);
}

.theme-toggle::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: white;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  transition: left 0.2s ease;
  pointer-events: none;
}

.theme-toggle.active::after {
  left: 4px;
}
</style>
