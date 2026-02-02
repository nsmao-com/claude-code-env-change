<template>
  <div class="min-h-screen">
    <!-- Background Pattern -->
    <div class="bg-pattern"></div>

    <!-- Custom Titlebar -->
    <AppTitlebar />

    <!-- Main Content -->
    <BentoGrid>
      <!-- Sidebar -->
      <Sidebar
        @add="openAddConfig"
        @open-mcp="showMcpPanel = true"
        @open-stats="showStatsModal = true"
        @open-prompts="showPromptModal = true"
        @open-skills="showSkillsPanel = true"
        @open-uptime="showUptimePanel = true"
        @export="exportConfig"
        @import="importConfig"
        @clear-claude="clearClaude"
        @clear-codex="clearCodex"
        @clear-gemini="clearGemini"
        @clear-all="clearAll"
      />

      <!-- Current Env Panel -->
      <CurrentEnvPanel />

      <!-- Config Grid -->
      <ConfigGrid
        :configs="configStore.filteredEnvironments"
        @edit="openEditConfig"
        @apply="applyConfig"
        @duplicate="duplicateConfig"
        @delete="deleteConfig"
        @test-latency="testConfigLatency"
      />
    </BentoGrid>

    <!-- Config Modal -->
    <ConfigModal
      v-model="showConfigModal"
      :edit-config="editingConfig"
      @saved="onConfigSaved"
    />

    <!-- MCP Panel -->
    <McpPanel v-model="showMcpPanel" />

    <!-- Stats Modal -->
    <StatsModal v-model="showStatsModal" />

    <!-- Prompt Editor Modal -->
    <PromptEditorModal
      :visible="showPromptModal"
      @close="showPromptModal = false"
      @saved="onPromptSaved"
    />

    <!-- Skills Panel -->
    <SkillsPanel v-model="showSkillsPanel" />

    <!-- Uptime Panel -->
    <UptimePanel v-model="showUptimePanel" />

    <!-- Toast Container -->
    <AppToast />

    <!-- Confirm Dialog -->
    <AppConfirm />
  </div>
</template>

<script setup lang="ts">
 import { ref, onMounted, nextTick } from 'vue'
 import type { EnvConfig } from '@/types'
 import { useConfigStore } from '@/stores/configStore'
 import { useUptimeStore } from '@/stores/uptimeStore'
 import { useConfirm } from '@/composables/useConfirm'
 import { useToast } from '@/composables/useToast'
 import { useTheme } from '@/composables/useTheme'

// Components
import AppTitlebar from '@/components/common/AppTitlebar.vue'
import AppToast from '@/components/common/AppToast.vue'
import AppConfirm from '@/components/common/AppConfirm.vue'
import BentoGrid from '@/components/layout/BentoGrid.vue'
import Sidebar from '@/components/layout/Sidebar.vue'
import CurrentEnvPanel from '@/components/config/CurrentEnvPanel.vue'
import ConfigGrid from '@/components/config/ConfigGrid.vue'
import ConfigModal from '@/components/config/ConfigModal.vue'
import McpPanel from '@/components/mcp/McpPanel.vue'
import StatsModal from '@/components/stats/StatsModal.vue'
import PromptEditorModal from '@/components/prompt/PromptEditorModal.vue'
import SkillsPanel from '@/components/skills/SkillsPanel.vue'
import UptimePanel from '@/components/uptime/UptimePanel.vue'

 // Initialize
 const configStore = useConfigStore()
 const uptimeStore = useUptimeStore()
 const confirm = useConfirm()
 const toast = useToast()
 useTheme() // Initialize theme

// State
const showConfigModal = ref(false)
const showMcpPanel = ref(false)
const showStatsModal = ref(false)
const showPromptModal = ref(false)
const showSkillsPanel = ref(false)
const showUptimePanel = ref(false)
const editingConfig = ref<EnvConfig | null>(null)

// Load config on mount
onMounted(async () => {
  try {
    await configStore.loadConfig()
   } catch (e) {
     console.error('Failed to load config:', e)
     toast.error('加载配置失败')
   }

   try {
     await uptimeStore.loadSnapshot()
     if (uptimeStore.settings.enabled) {
      uptimeStore.runOnce().catch((e: any) => {
        console.error('Uptime runOnce failed:', e)
      })
    }
  } catch (e) {
    console.error('Failed to init uptime:', e)
  }
})

// Config Modal Actions
function openAddConfig() {
  editingConfig.value = null
  showConfigModal.value = true
}

async function openEditConfig(index: number) {
  // Reset first to ensure watch triggers properly
  editingConfig.value = null
  await nextTick()
  // Deep clone to avoid reference issues
  editingConfig.value = JSON.parse(JSON.stringify(configStore.filteredEnvironments[index]))
  showConfigModal.value = true
}

async function applyConfig(index: number) {
  const config = configStore.filteredEnvironments[index]
  try {
    await configStore.applyEnv(config.name)
    toast.success(`已应用配置: ${config.name}`)
  } catch (e: any) {
    toast.error('应用失败: ' + e.message)
  }
}

async function duplicateConfig(index: number) {
  const config = configStore.filteredEnvironments[index]

  // Generate new name
  let newName = config.name + ' - 副本'
  let suffix = 1
  while (configStore.environments.some(c => c.name === newName)) {
    newName = config.name + ' - 副本 ' + suffix
    suffix++
  }

  const newConfig: EnvConfig = {
    ...config,
    name: newName
  }

  try {
    await configStore.addEnv(newConfig)
    toast.success('配置已复制')
  } catch (e: any) {
    toast.error('复制失败: ' + e.message)
  }
}

async function deleteConfig(index: number) {
  const config = configStore.filteredEnvironments[index]
  const confirmed = await confirm.show(
    '删除配置',
    '确定要删除此配置吗？此操作不可撤销。',
    'danger'
  )
  if (!confirmed) return

  try {
    await configStore.deleteEnv(config.name)
    toast.success('配置已删除')
  } catch (e: any) {
    toast.error('删除失败: ' + e.message)
  }
}

async function testConfigLatency(index: number) {
  const config = configStore.filteredEnvironments[index]
  const provider = config.provider || 'claude'

  // Get base URL based on provider
  let url = ''
  if (provider === 'claude') {
    url = config.variables?.ANTHROPIC_BASE_URL || ''
  } else if (provider === 'codex') {
    url = config.variables?.base_url || ''
  } else if (provider === 'gemini') {
    url = config.variables?.GOOGLE_GEMINI_BASE_URL || ''
  }

  if (!url) {
    toast.error('Base URL 为空')
    return
  }

  try {
    const ms = await configStore.testLatency(url)
    let colorText = ''
    if (ms > 1000) {
      colorText = `延迟: ${(ms / 1000).toFixed(1)}s (慢)`
    } else if (ms > 500) {
      colorText = `延迟: ${ms}ms (较慢)`
    } else if (ms > 300) {
      colorText = `延迟: ${ms}ms (一般)`
    } else {
      colorText = `延迟: ${ms}ms (快)`
    }
    toast.success(colorText)
  } catch (e: any) {
    toast.error('测速失败: ' + e.message)
  }
}

function onConfigSaved() {
  // Refresh handled by store
}

function onPromptSaved() {
  toast.success('提示词已保存')
}

// Export/Import Config
async function exportConfig() {
  try {
    const defaultName = `claudia-config-${Date.now()}.json`
    const savedPath = await configStore.exportConfig(defaultName)
    if (savedPath) {
      toast.success('配置已导出')
    }
    // If savedPath is empty, user cancelled the dialog - no message needed
  } catch (e: any) {
    toast.error('导出失败: ' + e.message)
  }
}

async function importConfig() {
  try {
    const count = await configStore.importConfig()
    if (count > 0) {
      toast.success(`已导入 ${count} 个配置`)
    }
    // If count is 0, either user cancelled or file was empty - no error message needed
  } catch (e: any) {
    toast.error('导入失败: ' + e.message)
  }
}

// Clear Actions
async function clearClaude() {
  const confirmed = await confirm.show(
    '清除 Claude 配置',
    '确定要清除 Claude 配置文件吗？',
    'warning'
  )
  if (!confirmed) return

  try {
    await configStore.clearClaudeSettings()
    toast.success('Claude 配置已清除')
  } catch (e: any) {
    toast.error('操作失败: ' + e.message)
  }
}

async function clearCodex() {
  const confirmed = await confirm.show(
    '清除 Codex 配置',
    '确定要清除 Codex 配置文件吗？',
    'warning'
  )
  if (!confirmed) return

  try {
    await configStore.clearCodexSettings()
    toast.success('Codex 配置已清除')
  } catch (e: any) {
    toast.error('操作失败: ' + e.message)
  }
}

async function clearGemini() {
  const confirmed = await confirm.show(
    '清除 Gemini 配置',
    '确定要清除 Gemini 配置文件吗？',
    'warning'
  )
  if (!confirmed) return

  try {
    await configStore.clearGeminiSettings()
    toast.success('Gemini 配置已清除')
  } catch (e: any) {
    toast.error('操作失败: ' + e.message)
  }
}

async function clearAll() {
  const confirmed = await confirm.show(
    '清除所有配置',
    '这将清除 Claude、Codex、Gemini 的配置文件。确定要继续吗？',
    'danger'
  )
  if (!confirmed) return

  try {
    await configStore.clearAllEnv()
    toast.success('所有配置已清除')
  } catch (e: any) {
    toast.error('操作失败: ' + e.message)
  }
}
</script>
