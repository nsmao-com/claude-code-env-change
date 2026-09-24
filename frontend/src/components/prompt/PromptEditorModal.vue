<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" :tool-filter="embedded">
    <template #header>
      <div>
        <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">{{ t('nav.prompts') }}</h1>
        <p class="mt-2 text-sm text-muted-foreground">{{ t('prompt.panelHint') }}</p>
      </div>
    </template>

    <Tabs v-model="activeTab">
      <SegmentedPills
        v-if="configStore.currentFilter === 'all' || configStore.currentFilter === 'claude_desktop'"
        class="mb-4"
        :model-value="activeTab"
        layout-id="prompt-tab-pill"
        :items="tabs"
        @update:model-value="activeTab = $event"
      >
        <template #default="{ item }">
          <BrandIcon :provider="item.value" class="size-3.5" />
          {{ item.label }}
        </template>
      </SegmentedPills>

      <div v-if="isLoading" class="flex items-center justify-center py-16">
        <Loader2 class="size-8 animate-spin text-muted-foreground" />
      </div>

      <template v-else>
        <div v-if="configStore.currentFilter === 'claude_desktop'" class="rounded-xl border border-border/70 bg-muted/30 p-4 text-sm text-muted-foreground">
          {{ t('prompt.desktopNote') }}
        </div>
        <template v-for="tab in tabs" :key="tab.value">
          <TabsContent v-if="!isDesktopFilter" :value="tab.value" class="flex flex-col gap-3">
            <div class="flex items-center justify-between gap-3">
              <div class="flex min-w-0 items-center gap-3">
                <AppTooltip :content="fileOf(tab.value)?.path" wrap :disabled="!fileOf(tab.value)?.path" class="min-w-0 max-w-md">
                  <span class="block truncate font-mono text-xs text-muted-foreground">
                    {{ fileOf(tab.value)?.path || '-' }}
                  </span>
                </AppTooltip>
                <Badge v-if="fileOf(tab.value)?.exists">{{ t('prompt.exists') }}</Badge>
                <Badge v-else variant="outline">{{ t('prompt.notCreated') }}</Badge>
              </div>
              <Button
                v-if="fileOf(tab.value)?.exists"
                variant="destructive"
                size="sm"
                @click="deleteFile(tab.value)"
              >
                <Trash2 />
                {{ t('prompt.delete') }}
              </Button>
            </div>

            <Textarea
              :model-value="fileOf(tab.value)?.content || ''"
              class="min-h-64 resize-y font-mono text-sm"
              :placeholder="getPlaceholder(tab.value)"
              spellcheck="false"
              @update:model-value="(v) => setFileContent(tab.value, v)"
            />
          </TabsContent>
        </template>
      </template>
    </Tabs>

    <template #footer>
      <div class="flex items-center justify-between gap-3">
        <p class="flex items-center text-xs text-muted-foreground">
          <Info class="mr-1.5 size-3.5" />
          {{ t('prompt.restartHint') }}
        </p>
        <div class="flex items-center gap-3">
          <Button v-if="!embedded" variant="secondary" @click="close">{{ t('common.cancel') }}</Button>
          <Button :disabled="isSaving || !editableFile || isDesktopFilter" @click="save">
            <Loader2 v-if="isSaving" class="animate-spin" />
            <Save v-else />
            {{ t('common.save') }}
          </Button>
        </div>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { ref, computed, watch } from 'vue'
import { Info, Loader2, Save, Trash2 } from '@lucide/vue'
import { GetPromptFiles, SavePromptFile, DeletePromptFile } from '../../../wailsjs/go/main/App'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import { useConfigStore } from '@/stores/configStore'
import AppModal from '@/components/common/AppModal.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

import { Tabs, TabsContent } from '@/components/ui/tabs'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import { Textarea } from '@/components/ui/textarea'

const { t } = useI18n()

interface PromptFile {
  provider: string
  path: string
  content: string
  exists: boolean
}

interface Props {
  visible: boolean
  embedded?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  saved: []
}>()

const toast = useToast()
const confirm = useConfirm()
const configStore = useConfigStore()

const isOpen = computed({
  get: () => props.visible,
  set: (value) => {
    if (!value) emit('close')
  }
})

const tabs = [
  { value: 'claude', label: 'CLAUDE CODE' },
  { value: 'codex', label: 'CODEX' },
  { value: 'antigravity', label: 'GEMINI' },
  { value: 'opencode', label: 'OPENCODE' },
  { value: 'grok', label: 'GROK' },
]

const activeTab = ref('claude')
const isLoading = ref(false)
const isSaving = ref(false)
const files = ref<PromptFile[]>([])
// 磁盘快照：用于判断哪些标签被改过（保存全部脏标签）
const originals = ref<Record<string, string>>({})
const dirtyProviders = computed(() =>
  files.value.filter(item => originals.value[item.provider] !== undefined && item.content !== originals.value[item.provider]).map(item => item.provider)
)
const editableFile = computed(() => fileOf(activeTab.value))
const isDesktopFilter = computed(() => configStore.currentFilter === 'claude_desktop')

function fileOf(provider: string) {
  return files.value.find(f => f.provider === provider)
}

function setFileContent(provider: string, value: string | number) {
  const file = fileOf(provider)
  if (file) file.content = String(value)
}

function getPlaceholder(provider?: string): string {
  const key = provider || activeTab.value
  const known = ['claude', 'codex', 'antigravity', 'grok', 'opencode']
  return known.includes(key) ? t(`prompt.placeholder.${key}`) : ''
}

async function loadFiles() {
  isLoading.value = true
  try {
    files.value = await GetPromptFiles()
    originals.value = Object.fromEntries(files.value.map(item => [item.provider, item.content]))
  } catch (e: any) {
    toast.error(t('prompt.loadFailed', { error: e?.message || String(e) }))
  } finally {
    isLoading.value = false
  }
}

async function save() {
  // 逐个标签编辑时用户往往改了多个标签才点一次保存：
  // 只保存当前标签会让其余改动静默丢失，这里保存全部与磁盘不一致的标签
  const dirty = files.value.filter(item => dirtyProviders.value.includes(item.provider))
  const targets = dirty.length ? dirty : files.value.filter(item => item.provider === activeTab.value)
  if (!targets.length) {
    toast.error(t('prompt.notLoaded'))
    return
  }
  isSaving.value = true
  try {
    for (const file of targets) {
      await SavePromptFile(file.provider, file.content)
    }
    const refreshed = await GetPromptFiles()
    files.value = files.value.map(item => refreshed.find(r => r.provider === item.provider) || item)
    originals.value = Object.fromEntries(files.value.map(item => [item.provider, item.content]))
    for (const file of targets) {
      const saved = refreshed.find(item => item.provider === file.provider)
      if (!saved || saved.content !== file.content) throw new Error(t('prompt.verifyFailed'))
    }
    emit('saved')
  } catch (e: any) {
    toast.error(t('prompt.saveFailed', { error: e?.message || String(e) }))
  } finally {
    isSaving.value = false
  }
}

async function deleteFile(provider = activeTab.value) {
  const ok = await confirm.show(
    t('prompt.deleteTitle'),
    t('prompt.deleteMsg', { name: provider.toUpperCase() }),
    'danger'
  )
  if (!ok) return

  try {
    await DeletePromptFile(provider)

    const file = fileOf(provider)
    if (file) {
      file.content = ''
      file.exists = false
    }

    emit('saved')
  } catch (e: any) {
    toast.error(t('prompt.deleteFailed', { error: e?.message || String(e) }))
  }
}

function close() {
  emit('close')
}

watch(() => props.visible, (newVal) => {
  if (newVal) {
    loadFiles()
  }
}, { immediate: true })

watch(() => configStore.currentFilter, (tool) => {
  if (tool === 'claude' || tool === 'codex' || tool === 'antigravity' || tool === 'opencode' || tool === 'grok') {
    activeTab.value = tool
  }
}, { immediate: true })
</script>
