<template>
  <TooltipProvider>
    <MotionConfig :reduced-motion="settings.reducedMotion ? 'always' : 'user'" :transition="{ ease: [0.23, 1, 0.32, 1], duration: 0.18 }">
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
        @search="showPalette = true"
      />
      <div class="relative min-h-0 flex-1 overflow-hidden">
        <motion.div
          :key="page"
          class="h-full min-h-0"
          :initial="pageEnter.initial"
          :animate="pageEnter.animate"
          :transition="pageEnter.transition"
        >
            <ScrollArea v-if="page === 'home'" class="h-full">
              <div class="space-y-4 px-6 pb-8 pt-4">
                <CurrentEnvPanel @add="openAddConfig" @navigate="page = $event" />
              </div>
            </ScrollArea>
            <ScrollArea v-else-if="page === 'env'" class="h-full">
              <div class="space-y-4 px-6 pb-8 pt-4">
                <ConfigGrid
                  :configs="configStore.filteredEnvironments"
                  :importing="importingLocal"
                  @add="openAddConfig"
                  @edit="openEditConfig"
                  @apply="applyConfig"
                  @duplicate="duplicateConfig"
                  @delete="deleteConfig"
                  @import-local="importLocalConfig"
                  @import-json="openImportModal"
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
            <SettingsPanel
              v-else-if="page === 'settings'"
              class="h-full min-h-0"
              embedded
              :model-value="true"
              @check-update="showUpdateDialog = true"
              @export="exportConfig"
              @import="importConfig"
            />
        </motion.div>
      </div>
    </div>

    <CommandPalette
      v-model="showPalette"
      @navigate="page = $event"
      @add="openAddConfig"
      @edit="openEditByName"
      @apply="applyByName"
      @import-local="importLocalConfig"
      @import-json="openImportModal"
    />
    <ConfigImportModal v-model="showImportModal" :seed="importSeed" />
    <ConfigModal v-model="showConfigModal" :edit-config="editingConfig" @saved="onConfigSaved" />
    <UpdateDialog v-model="showUpdateDialog" @available="updateAvailable = true" />
    <AppToast />
    <AppConfirm />
    <div
      v-if="windowDragging"
      class="pointer-events-none fixed inset-0 z-[80] flex items-center justify-center bg-background/70 px-6"
    >
      <div class="flex w-full max-w-md flex-col items-center gap-3 rounded-2xl bg-card px-8 py-10 text-center ring-2 ring-brand">
        <Upload class="size-8 text-brand" />
        <p class="text-base font-medium">{{ t('importModal.dropWindow') }}</p>
        <p class="text-sm text-muted-foreground">{{ t('importModal.dropWindowHint') }}</p>
      </div>
    </div>
    </MotionConfig>
  </TooltipProvider>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick, watch, onBeforeUnmount } from 'vue'
import type { AppPage, EnvConfig } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import { useUptimeStore } from '@/stores/uptimeStore'
import { useRouterStore } from '@/stores/routerStore'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import { useTheme } from '@/composables/useTheme'
import { useSettings } from '@/composables/useSettings'
import { useI18n } from '@/composables/useI18n'
import { updateService } from '@/services/updateService'
import { APP_PAGES } from '@/lib/nav'
import { MotionConfig, motion } from 'motion-v'
import { pageEnter } from '@/lib/motion'
import { TooltipProvider } from '@/components/ui/tooltip'
import { ScrollArea } from '@/components/ui/scroll-area'
import AppTitlebar from '@/components/common/AppTitlebar.vue'
import AppToast from '@/components/common/AppToast.vue'
import AppConfirm from '@/components/common/AppConfirm.vue'
import CurrentEnvPanel from '@/components/config/CurrentEnvPanel.vue'
import ConfigGrid from '@/components/config/ConfigGrid.vue'
import ConfigImportModal from '@/components/config/ConfigImportModal.vue'
import ConfigModal from '@/components/config/ConfigModal.vue'
import McpPanel from '@/components/mcp/McpPanel.vue'
import StatsModal from '@/components/stats/StatsModal.vue'
import PromptEditorModal from '@/components/prompt/PromptEditorModal.vue'
import SkillsPanel from '@/components/skills/SkillsPanel.vue'
import UptimePanel from '@/components/uptime/UptimePanel.vue'
import RouterPanel from '@/components/router/RouterPanel.vue'
import CloudSyncPanel from '@/components/cloud/CloudSyncPanel.vue'
import UpdateDialog from '@/components/common/UpdateDialog.vue'
import SettingsPanel from '@/components/settings/SettingsPanel.vue'
import CommandPalette from '@/components/common/CommandPalette.vue'
import { OnFileDrop, OnFileDropOff } from '../wailsjs/runtime/runtime'
import { Upload } from '@lucide/vue'
import { classifyImportPayload } from '@/lib/configImport'

const configStore = useConfigStore()
const uptimeStore = useUptimeStore()
const routerStore = useRouterStore()
const confirm = useConfirm()
const toast = useToast()
useTheme()
const { settings, saveLastPage, readLastPage } = useSettings()
const { t } = useI18n()

const page = ref<AppPage>('home')
const showConfigModal = ref(false)
const showUpdateDialog = ref(false)
const showPalette = ref(false)
const showImportModal = ref(false)
const importSeed = ref<{ name: string, text: string } | null>(null)
const windowDragging = ref(false)
const updateAvailable = ref(false)
const editingConfig = ref<EnvConfig | null>(null)
const importingLocal = ref(false)

watch(page, (id) => saveLastPage(id))

function onSettingsShortcut(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === ',') {
    e.preventDefault()
    page.value = 'settings'
  }
  if ((e.ctrlKey || e.metaKey) && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault()
    showPalette.value = !showPalette.value
  }
}

onMounted(async () => {
  const last = readLastPage()
  if (last && APP_PAGES.some(item => item.id === last)) page.value = last as AppPage
  window.addEventListener('keydown', onSettingsShortcut)
  window.addEventListener('dragenter', onWinDragEnter)
  window.addEventListener('dragover', onWinDragOver)
  window.addEventListener('dragleave', onWinDragLeave)
  window.addEventListener('drop', onWinDrop)
  try {
    OnFileDrop((_x, _y, paths) => {
      void handleDroppedPaths(paths)
    }, false)
  } catch {
    /* runtime 未就绪时仍可用 HTML5 拖放 */
  }

  try {
    await configStore.loadConfig()
  } catch {
    toast.error(t('toast.loadFailed'))
  }
  try {
    await uptimeStore.loadSnapshot()
    if (uptimeStore.settings.enabled) {
      uptimeStore.runOnce().catch(() => {})
    }
  } catch {
    /* ignore */
  }
  if (settings.checkUpdateOnLaunch) {
    updateService.check().then((info) => {
      if (info?.available) updateAvailable.value = true
    }).catch(() => {})
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onSettingsShortcut)
  window.removeEventListener('dragenter', onWinDragEnter)
  window.removeEventListener('dragover', onWinDragOver)
  window.removeEventListener('dragleave', onWinDragLeave)
  window.removeEventListener('drop', onWinDrop)
  try { OnFileDropOff() } catch { /* ignore */ }
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
  if (config) await applyByName(config.name)
}

async function applyByName(name: string) {
  try {
    const message = await configStore.applyEnv(name)
    if (message === 'unapplied') toast.success(t('toast.unapplied', { name }))
    else if (message && message.includes('⚠')) toast.error(message)
    else toast.success(t('toast.applied', { name }))
  } catch (e: any) {
    toast.error(t('toast.applyFailed', { error: e.message }))
  }
}

async function openEditByName(name: string) {
  const config = configStore.getEnvByName(name)
  if (!config) return
  editingConfig.value = null
  await nextTick()
  editingConfig.value = JSON.parse(JSON.stringify(config))
  showConfigModal.value = true
}

async function importLocalConfig() {
  if (importingLocal.value) return
  page.value = 'env'
  importingLocal.value = true
  try {
    const filter = configStore.currentFilter
    const added = await configStore.importLocalEnv(filter === 'all' ? 'all' : filter)
    toast.success(t('toast.importedLocal', { count: added.length }))
  } catch (e: any) {
    toast.error(t('toast.importLocalFailed', { error: e?.message || String(e) }))
  } finally {
    importingLocal.value = false
  }
}

async function duplicateConfig(index: number) {
  const config = configStore.filteredEnvironments[index]
  if (!config) {
    toast.error(t('toast.copyFailed', { error: '找不到这条配置' }))
    return
  }
  let newName = config.name + ' - 副本'
  let suffix = 1
  while (configStore.environments.some(c => c.name === newName)) {
    newName = config.name + ' - 副本 ' + suffix
    suffix++
  }
  try {
    await configStore.addEnv({ ...config, name: newName })
    toast.success(t('toast.copied'))
  } catch (e: any) {
    toast.error(t('toast.copyFailed', { error: e.message }))
  }
}

async function deleteConfig(index: number) {
  const config = configStore.filteredEnvironments[index]
  if (!config) {
    toast.error(t('toast.deleteFailed', { error: '找不到这条配置' }))
    return
  }
  if (!(await confirm.show(t('confirm.deleteConfig'), t('confirm.deleteConfigMsg'), 'danger'))) return
  try {
    await configStore.deleteEnv(config.name)
    toast.success(t('toast.deleted'))
  } catch (e: any) {
    toast.error(t('toast.deleteFailed', { error: e.message }))
  }
}

function onConfigSaved() {}
function onPromptSaved() {
  toast.success(t('toast.promptSaved'))
}

async function onCloudPulled() {
  try {
    await configStore.loadConfig()
    await uptimeStore.loadSnapshot()
    await routerStore.loadConfig()
    await routerStore.refreshStatus()
  } catch (e: any) {
    toast.error(t('toast.cloudRefreshFailed', { error: e?.message || String(e) }))
  }
}

async function exportConfig() {
  try {
    const savedPath = await configStore.exportConfig(`claudia-config-${Date.now()}.json`)
    if (savedPath) toast.success(t('toast.exported'))
  } catch (e: any) {
    toast.error(t('toast.exportFailed', { error: e.message }))
  }
}

function openImportModal() {
  importSeed.value = null
  showImportModal.value = true
}

function openImportWithFile(file: { name: string, text: string }) {
  importSeed.value = file
  showImportModal.value = true
}

let dropGuard = 0

function isFileDrag(event: DragEvent) {
  return Array.from(event.dataTransfer?.types || []).includes('Files')
}

function onWinDragEnter(event: DragEvent) {
  if (!isFileDrag(event)) return
  event.preventDefault()
  windowDragging.value = true
}

function onWinDragOver(event: DragEvent) {
  if (!isFileDrag(event)) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  windowDragging.value = true
}

function onWinDragLeave(event: DragEvent) {
  if (!isFileDrag(event)) return
  if (event.relatedTarget) return
  windowDragging.value = false
}

async function onWinDrop(event: DragEvent) {
  windowDragging.value = false
  if (!isFileDrag(event)) return
  event.preventDefault()
  const file = event.dataTransfer?.files?.[0]
  if (!file) return
  const text = await file.text()
  openImportGuarded({ name: file.name, text })
}

async function handleDroppedPaths(paths: string[]) {
  windowDragging.value = false
  const path = (paths || []).find(item => /\.(json|txt|jsonc)$/i.test(item)) || paths?.[0]
  if (!path) return
  try {
    const text = await configStore.readDroppedFile(path)
    const name = path.replace(/^.*[\\/]/, '') || 'import.json'
    openImportGuarded({ name, text })
  } catch (e: unknown) {
    toast.error(t('toast.importFailed', { error: e instanceof Error ? e.message : String(e) }))
  }
}

function openImportGuarded(file: { name: string, text: string }) {
  const now = Date.now()
  if (now - dropGuard < 500) return
  dropGuard = now
  const kind = classifyImportPayload(file.text)
  if (kind === 'mcp') {
    if (page.value !== 'mcp') toast.error(t('importModal.mcpHint'))
    return
  }
  openImportWithFile(file)
}

function importConfig() {
  openImportModal()
}

async function clearClaude() {
  if (!(await confirm.show(t('confirm.clearClaude'), t('confirm.clearClaudeMsg'), 'warning'))) return
  try {
    await configStore.clearClaudeSettings()
    toast.success(t('toast.clearedClaude'))
  } catch (e: any) {
    toast.error(t('toast.opFailed', { error: e.message }))
  }
}

async function clearCodex() {
  if (!(await confirm.show(t('confirm.clearCodex'), t('confirm.clearCodexMsg'), 'warning'))) return
  try {
    await configStore.clearCodexSettings()
    toast.success(t('toast.clearedCodex'))
  } catch (e: any) {
    toast.error(t('toast.opFailed', { error: e.message }))
  }
}

async function clearGemini() {
  if (!(await confirm.show(t('confirm.clearGemini'), t('confirm.clearGeminiMsg'), 'warning'))) return
  try {
    await configStore.clearGeminiSettings()
    toast.success(t('toast.clearedGemini'))
  } catch (e: any) {
    toast.error(t('toast.opFailed', { error: e.message }))
  }
}

async function clearOpencode() {
  if (!(await confirm.show(t('confirm.clearOpencode'), t('confirm.clearOpencodeMsg'), 'warning'))) return
  try {
    await configStore.clearOpencodeSettings()
    toast.success(t('toast.clearedOpencode'))
  } catch (e: any) {
    toast.error(t('toast.opFailed', { error: e.message }))
  }
}

async function clearGrok() {
  if (!(await confirm.show(t('confirm.clearGrok'), t('confirm.clearGrokMsg'), 'warning'))) return
  try {
    await configStore.clearGrokSettings()
    toast.success(t('toast.clearedGrok'))
  } catch (e: any) {
    toast.error(t('toast.opFailed', { error: e.message }))
  }
}

async function clearAll() {
  if (!(await confirm.show(t('confirm.clearAll'), t('confirm.clearAllMsg'), 'danger'))) return
  try {
    await configStore.clearAllEnv()
    toast.success(t('toast.clearedAll'))
  } catch (e: any) {
    toast.error(t('toast.opFailed', { error: e.message }))
  }
}
</script>
