<template>
  <div class="bento-item area-current flex flex-col px-6 py-4 h-full">
    <div class="flex items-center justify-between mb-3 flex-none">
      <h2 class="text-lg font-bold tracking-tight text-foreground uppercase">当前环境</h2>
      <button
        class="btn btn-secondary h-7 text-xs gap-1.5 font-bold uppercase tracking-wide border border-border hover:border-foreground transition-all"
        @click="refresh"
      >
        <i class="fas fa-arrow-rotate-right text-[10px]"></i>
        刷新
      </button>
    </div>

    <div v-if="isLoading" class="flex-1 flex items-center justify-center">
      <div class="flex items-center gap-2">
        <i class="fas fa-circle-notch fa-spin text-lg text-foreground"></i>
        <span class="text-xs text-muted-foreground font-mono">加载中...</span>
      </div>
    </div>

    <template v-else>
      <!-- Provider Tabs -->
      <div class="relative mb-3 p-1 bg-secondary/50 border border-border/50 rounded-full inline-flex flex-none w-full max-w-xs backdrop-blur-sm">
        <!-- Glider -->
        <div
          class="absolute top-1 bottom-1 bg-white rounded-full transition-all duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] shadow-[0_2px_8px_rgba(0,0,0,0.08)] border border-black/5 dark:border-white/10"
          :style="gliderStyle"
        ></div>

        <button
          v-for="tab in providerTabs"
          :key="tab.value"
          ref="tabRefs"
          :class="['relative z-10 flex-1 py-1 rounded-full text-[11px] font-bold uppercase tracking-wide transition-colors duration-200 text-center', { 'text-foreground dark:text-gray-900': activeTab === tab.value, 'text-muted-foreground hover:text-foreground/80': activeTab !== tab.value }]"
          @click="activeTab = tab.value"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- Content Area -->
      <div class="flex-1 overflow-y-auto min-h-0 custom-scrollbar">
        <!-- Claude Settings -->
        <div v-if="activeTab === 'claude'" class="space-y-2">
          <div class="flex items-center gap-2 px-3 py-2 rounded-lg border border-border">
            <div :class="['w-2 h-2 rounded-full border border-foreground', claudeEnvName ? 'bg-foreground' : 'bg-transparent']"></div>
            <span class="font-bold text-xs font-mono">{{ claudeEnvName || '未配置' }}</span>
          </div>
          <div v-if="claudeSettings && Object.keys(claudeSettings).length > 0" class="space-y-0.5">
            <div v-for="(value, key) in claudeSettings" :key="key" class="flex items-center justify-between px-2 py-1 text-[11px] hover:bg-muted/30 rounded">
              <span class="text-muted-foreground font-medium uppercase">{{ key }}</span>
              <span class="font-mono text-foreground truncate ml-2 max-w-[200px]">{{ maskValue(String(key), value) }}</span>
            </div>
          </div>
          <div v-else class="flex items-center justify-center py-4 text-muted-foreground border border-dashed border-border rounded-lg">
            <p class="text-xs">暂无变量</p>
          </div>
        </div>

        <!-- Codex Settings -->
        <div v-if="activeTab === 'codex'" class="space-y-2">
          <div class="flex items-center gap-2 px-3 py-2 rounded-lg border border-border">
            <div :class="['w-2 h-2 rounded-full border border-foreground', codexEnvName ? 'bg-foreground' : 'bg-transparent']"></div>
            <span class="font-bold text-xs font-mono">{{ codexEnvName || '未配置' }}</span>
          </div>
          <div v-if="codexSettings && Object.keys(codexSettings).length > 0" class="space-y-0.5">
            <div v-for="(value, key) in codexSettings" :key="key" class="flex items-center justify-between px-2 py-1 text-[11px] hover:bg-muted/30 rounded">
              <span class="text-muted-foreground font-medium uppercase">{{ key }}</span>
              <span class="font-mono text-foreground truncate ml-2 max-w-[200px]">{{ maskValue(String(key), value) }}</span>
            </div>
          </div>
          <div v-else class="flex items-center justify-center py-4 text-muted-foreground border border-dashed border-border rounded-lg">
            <p class="text-xs">暂无变量</p>
          </div>
        </div>

        <!-- Gemini Settings -->
        <div v-if="activeTab === 'gemini'" class="space-y-2">
          <div class="flex items-center gap-2 px-3 py-2 rounded-lg border border-border">
            <div :class="['w-2 h-2 rounded-full border border-foreground', geminiEnvName ? 'bg-foreground' : 'bg-transparent']"></div>
            <span class="font-bold text-xs font-mono">{{ geminiEnvName || '未配置' }}</span>
          </div>
          <div v-if="geminiSettings && Object.keys(geminiSettings).length > 0" class="space-y-0.5">
            <div v-for="(value, key) in geminiSettings" :key="key" class="flex items-center justify-between px-2 py-1 text-[11px] hover:bg-muted/30 rounded">
              <span class="text-muted-foreground font-medium uppercase">{{ key }}</span>
              <span class="font-mono text-foreground truncate ml-2 max-w-[200px]">{{ maskValue(String(key), value) }}</span>
            </div>
          </div>
          <div v-else class="flex items-center justify-center py-4 text-muted-foreground border border-dashed border-border rounded-lg">
            <p class="text-xs">暂无变量</p>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useConfigStore } from '@/stores/configStore'
import type { Provider } from '@/types'

const configStore = useConfigStore()

const isLoading = computed(() => configStore.isLoading)
const claudeEnvName = computed(() => configStore.currentEnvClaude)
const codexEnvName = computed(() => configStore.currentEnvCodex)
const geminiEnvName = computed(() => configStore.currentEnvGemini)

const activeTab = computed({
  get: () => configStore.currentEnvTab,
  set: (val: Provider) => configStore.setEnvTab(val)
})
const claudeSettings = ref<Record<string, string> | null>(null)
const codexSettings = ref<Record<string, string> | null>(null)
const geminiSettings = ref<Record<string, string> | null>(null)

// Tab Glider Logic
const tabRefs = ref<HTMLElement[]>([])
const gliderStyle = ref({
  left: '4px',
  width: '0px'
})

const providerTabs = [
  { value: 'claude' as Provider, label: 'CLAUDE' },
  { value: 'codex' as Provider, label: 'CODEX' },
  { value: 'gemini' as Provider, label: 'GEMINI' }
]

function updateGlider() {
  nextTick(() => {
    const activeIndex = providerTabs.findIndex(t => t.value === activeTab.value)
    if (activeIndex !== -1 && tabRefs.value[activeIndex]) {
      const el = tabRefs.value[activeIndex]
      gliderStyle.value = {
        left: `${el.offsetLeft}px`,
        width: `${el.offsetWidth}px`
      }
    }
  })
}

watch(activeTab, () => updateGlider())

// Mask sensitive values like API keys
function maskValue(key: string, value: string): string {
  const sensitiveKeys = ['API_KEY', 'AUTH_TOKEN', 'SECRET', 'PASSWORD', 'TOKEN']
  const isSensitive = sensitiveKeys.some(k => key.toUpperCase().includes(k))

  if (isSensitive && value && value.length > 8) {
    return value.substring(0, 4) + '••••' + value.substring(value.length - 4)
  }
  return value || '-'
}

async function loadSettings() {
  try {
    claudeSettings.value = await configStore.getCurrentSettings('claude')
    codexSettings.value = await configStore.getCurrentSettings('codex')
    geminiSettings.value = await configStore.getCurrentSettings('gemini')
  } catch {
    // ignore
  }
}

async function refresh() {
  await configStore.loadConfig()
  await loadSettings()
  updateGlider()
}

onMounted(() => {
  loadSettings()
  setTimeout(updateGlider, 100)
})

watch([claudeEnvName, codexEnvName, geminiEnvName], () => {
  loadSettings()
})

watch(isLoading, (newVal, oldVal) => {
  if (oldVal === true && newVal === false) {
    loadSettings()
  }
})
</script>
