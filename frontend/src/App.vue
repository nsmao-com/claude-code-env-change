<template>
  <TooltipProvider>
    <MotionConfig :reduced-motion="'user'" :transition="{ ease: [0.23, 1, 0.32, 1], duration: 0.18 }">
    <div class="flex h-full min-h-0 flex-col bg-background">
      <AppTitlebar
        :page="page"
        :update-available="updateAvailable"
        @navigate="page = $event"
        @check-update="showUpdateDialog = true"
        @export="exportConfig"
        @import="importConfig"
        @clear-claude="clearClaude"
        @clear-codex="clearCodex"
        @clear-gemini="clearGemini"
        @clear-opencode="clearOpencode"
        @clear-grok="clearGrok"
        @clear-all="clearAll"
      />
      <div class="relative min-h-0 flex-1 overflow-hidden">
        <motion.div
          :key="page"
          class="h-full min-h-0"
          :initial="pageEnter.initial"
          :animate="pageEnter.animate"
          :transition="pageEnter.transition"
        >
            <ScrollArea v-if="page === 'env'" class="h-full">
              <div class="space-y-4 px-6 pb-8 pt-4">
                <CurrentEnvPanel @add="openAddConfig" @navigate="page = $event" />
                <ConfigGrid
                  :configs="configStore.filteredEnvironments"
                  @add="openAddConfig"
                  @edit="openEditConfig"
                  @apply="applyConfig"
                  @duplicate="duplicateConfig"
                  @delete="deleteConfig"
                  @test-latency="testConfigLatency"
                />
              </div>
            </ScrollArea>
            <McpPanel v-else-if="page === 'mcp'" class="h-full min-h-0" embedded :model-value="true" />
            <SkillsPanel v-else-if="page === 'skills'" class="h-full min-h-0" embedded :model-value="true" />
            <RouterPanel v-else-if="page === 'router'" class="h-full min-h-0" embedded :model-value="true" />
            <UptimePanel v-else-if="page === 'uptime'" class="h-full min-h-0" embedded :model-value="true" />
            <CloudSyncPanel v-else-if="page === 'cloud'" class="h-full min-h-0" embedded :model-value="true" @pulled="onCloudPulled" />
            <PromptEditorModal v-else-if="page === 'prompts'" class="h-full min-h-0" embedded :visible="true" @saved="onPromptSaved" />
            <StatsModal v-else-if="page === 'stats'" class="h-full min-h-0" embedded :model-value="true" />
        </motion.div>
      </div>
    </div>

    <ConfigModal v-model="showConfigModal" :edit-config="editingConfig" @saved="onConfigSaved" />
    <UpdateDialog v-model="showUpdateDialog" @available="updateAvailable = true" />
    <AppToast />
    <AppConfirm />
    </MotionConfig>
  </TooltipProvider>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick } from 'vue'
import type { AppPage, EnvConfig } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import { useUptimeStore } from '@/stores/uptimeStore'
import { useRouterStore } from '@/stores/routerStore'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import { useTheme } from '@/composables/useTheme'
import { MotionConfig, motion } from 'motion-v'
import { pageEnter } from '@/lib/motion'
import { TooltipProvider } from '@/components/ui/tooltip'
import { ScrollArea } from '@/components/ui/scroll-area'
import AppTitlebar from '@/components/common/AppTitlebar.vue'
import AppToast from '@/components/common/AppToast.vue'
import AppConfirm from '@/components/common/AppConfirm.vue'
import CurrentEnvPanel from '@/components/config/CurrentEnvPanel.vue'
import ConfigGrid from '@/components/config/ConfigGrid.vue'
import ConfigModal from '@/components/config/ConfigModal.vue'
import McpPanel from '@/components/mcp/McpPanel.vue'
import StatsModal from '@/components/stats/StatsModal.vue'
import PromptEditorModal from '@/components/prompt/PromptEditorModal.vue'
import SkillsPanel from '@/components/skills/SkillsPanel.vue'
import UptimePanel from '@/components/uptime/UptimePanel.vue'
import RouterPanel from '@/components/router/RouterPanel.vue'
import CloudSyncPanel from '@/components/cloud/CloudSyncPanel.vue'
import UpdateDialog from '@/components/common/UpdateDialog.vue'

const configStore = useConfigStore()
const uptimeStore = useUptimeStore()
const routerStore = useRouterStore()
const confirm = useConfirm()
const toast = useToast()
useTheme()

const page = ref<AppPage>('env')
const showConfigModal = ref(false)
const showUpdateDialog = ref(false)
const updateAvailable = ref(false)
const editingConfig = ref<EnvConfig | null>(null)

onMounted(async () => {
  try {
    await configStore.loadConfig()
  } catch {
    toast.error('加载配置失败')
  }
  try {
    await uptimeStore.loadSnapshot()
    if (uptimeStore.settings.enabled) {
      uptimeStore.runOnce().catch(() => {})
    }
  } catch {
    /* ignore */
  }
})

function openAddConfig() {
  editingConfig.value = null
  showConfigModal.value = true
}

async function openEditConfig(index: number) {
  editingConfig.value = null
  await nextTick()
  editingConfig.value = JSON.parse(JSON.stringify(configStore.filteredEnvironments[index]))
  showConfigModal.value = true
}

async function applyConfig(index: number) {
  const config = configStore.filteredEnvironments[index]
  try {
    const message = await configStore.applyEnv(config.name)
    if (message && message.includes('⚠')) toast.error(message)
    else toast.success(`已应用: ${config.name}`)
  } catch (e: any) {
    toast.error('应用失败: ' + e.message)
  }
}

async function duplicateConfig(index: number) {
  const config = configStore.filteredEnvironments[index]
  let newName = config.name + ' - 副本'
  let suffix = 1
  while (configStore.environments.some(c => c.name === newName)) {
    newName = config.name + ' - 副本 ' + suffix
    suffix++
  }
  try {
    await configStore.addEnv({ ...config, name: newName })
    toast.success('配置已复制')
  } catch (e: any) {
    toast.error('复制失败: ' + e.message)
  }
}

async function deleteConfig(index: number) {
  const config = configStore.filteredEnvironments[index]
  if (!(await confirm.show('删除配置', '确定删除此配置？此操作不可撤销。', 'danger'))) return
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
  let url = ''
  if (provider === 'claude') url = config.variables?.ANTHROPIC_BASE_URL || ''
  else if (provider === 'codex') url = config.variables?.base_url || ''
  else if (provider === 'gemini') url = config.variables?.GOOGLE_GEMINI_BASE_URL || ''
  else if (provider === 'opencode') url = config.variables?.OPENCODE_BASE_URL || ''
  else if (provider === 'grok') url = config.variables?.XAI_BASE_URL || 'https://api.x.ai/v1'
  if (!url) {
    toast.error('Base URL 为空')
    return
  }
  try {
    const ms = await configStore.testLatency(url)
    toast.success(ms > 1000 ? `延迟 ${(ms / 1000).toFixed(1)}s` : `延迟 ${ms}ms`)
  } catch (e: any) {
    toast.error('测速失败: ' + e.message)
  }
}

function onConfigSaved() {}
function onPromptSaved() {
  toast.success('提示词已保存')
}

async function onCloudPulled() {
  try {
    await configStore.loadConfig()
    await uptimeStore.loadSnapshot()
    await routerStore.loadConfig()
    await routerStore.refreshStatus()
  } catch (e: any) {
    toast.error('云端配置已写入，刷新界面失败: ' + (e?.message || String(e)))
  }
}

async function exportConfig() {
  try {
    const savedPath = await configStore.exportConfig(`claudia-config-${Date.now()}.json`)
    if (savedPath) toast.success('配置已导出')
  } catch (e: any) {
    toast.error('导出失败: ' + e.message)
  }
}

async function importConfig() {
  try {
    const count = await configStore.importConfig()
    if (count > 0) toast.success(`已导入 ${count} 个配置`)
  } catch (e: any) {
    toast.error('导入失败: ' + e.message)
  }
}

async function clearClaude() {
  if (!(await confirm.show('清除 Claude 配置', '确定清除 Claude 配置文件？', 'warning'))) return
  try {
    await configStore.clearClaudeSettings()
    toast.success('Claude 配置已清除')
  } catch (e: any) {
    toast.error('操作失败: ' + e.message)
  }
}

async function clearCodex() {
  if (!(await confirm.show('清除 Codex 配置', '确定清除 Codex 配置文件？', 'warning'))) return
  try {
    await configStore.clearCodexSettings()
    toast.success('Codex 配置已清除')
  } catch (e: any) {
    toast.error('操作失败: ' + e.message)
  }
}

async function clearGemini() {
  if (!(await confirm.show('清除 Gemini 配置', '确定清除 Gemini 配置文件？', 'warning'))) return
  try {
    await configStore.clearGeminiSettings()
    toast.success('Gemini 配置已清除')
  } catch (e: any) {
    toast.error('操作失败: ' + e.message)
  }
}

async function clearOpencode() {
  if (!(await confirm.show('清除 OpenCode 配置', '确定清除 OpenCode 写入的模型配置？其余配置会保留。', 'warning'))) return
  try {
    await configStore.clearOpencodeSettings()
    toast.success('OpenCode 模型配置已清除')
  } catch (e: any) {
    toast.error('操作失败: ' + e.message)
  }
}

async function clearGrok() {
  if (!(await confirm.show('清除 Grok 配置', '确定清除 Grok 写入的 API Key？MCP 配置会保留。', 'warning'))) return
  try {
    await configStore.clearGrokSettings()
    toast.success('Grok API Key 已从 config.toml 清除')
  } catch (e: any) {
    toast.error('操作失败: ' + e.message)
  }
}

async function clearAll() {
  if (!(await confirm.show('清除全部', '确定清除所有平台配置？此操作不可撤销。', 'danger'))) return
  try {
    await configStore.clearAllEnv()
    toast.success('已清除全部配置')
  } catch (e: any) {
    toast.error('操作失败: ' + e.message)
  }
}
</script>
