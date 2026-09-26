<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" :tool-filter="embedded">
    <template #header>
      <div>
        <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">{{ t('nav.prompts') }}</h1>
        <p class="mt-2 text-sm text-muted-foreground">{{ t('prompt.panelHint') }}</p>
      </div>
    </template>

    <Empty v-if="isDesktopFilter" class="min-h-64">
      <EmptyHeader>
        <BrandIcon provider="claude_desktop" class="size-8 text-muted-foreground" />
        <EmptyTitle>{{ t('prompt.desktopTitle') }}</EmptyTitle>
        <EmptyDescription>{{ t('prompt.desktopNote') }}</EmptyDescription>
      </EmptyHeader>
      <Button variant="outline" @click="configStore.setFilter('all')">{{ t('prompt.showSupported') }}</Button>
    </Empty>

    <Tabs v-else v-model="activeTab">
      <SegmentedPills
        v-if="configStore.currentFilter === 'all'"
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
        <template v-for="tab in tabs" :key="tab.value">
          <TabsContent :value="tab.value" class="flex flex-col gap-3">
            <div class="flex items-center justify-between gap-3">
              <div class="flex min-w-0 items-center gap-3">
                <AppTooltip :content="fileOf(tab.value)?.path" wrap :disabled="!fileOf(tab.value)?.path" class="min-w-0 max-w-md">
                  <span class="block truncate font-mono text-xs text-muted-foreground">
                    {{ fileOf(tab.value)?.path || '-' }}
                  </span>
                </AppTooltip>
                <Badge v-if="fileOf(tab.value)?.exists">{{ t('prompt.exists') }}</Badge>
                <Badge v-else variant="outline">{{ t('prompt.notCreated') }}</Badge>
                <Badge v-if="dirtyProviders.includes(tab.value)" variant="outline">{{ t('prompt.unsaved') }}</Badge>
              </div>
              <Button
                v-if="fileOf(tab.value)?.exists"
                variant="destructive"
                size="sm"
                :disabled="isSaving || isDeleting"
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
              :disabled="isDeleting"
              @update:model-value="(v) => setFileContent(tab.value, v)"
            />
          </TabsContent>
        </template>
      </template>
    </Tabs>

    <template v-if="!isDesktopFilter" #footer>
      <div class="flex items-center justify-between gap-3">
        <p class="flex items-center text-xs text-muted-foreground">
          <Info class="mr-1.5 size-3.5" />
          {{ t('prompt.restartHint') }}
        </p>
        <div class="flex items-center gap-3">
          <Button v-if="!embedded" variant="secondary" @click="close">{{ t('common.cancel') }}</Button>
          <Button :disabled="isLoading || isSaving || isDeleting || !editableFile || isDesktopFilter" @click="save">
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
import { ref, computed, watch, onScopeDispose } from 'vue'
import { Info, Loader2, Save, Trash2 } from '@lucide/vue'
import { GetPromptFiles, GetPromptFile, SavePromptFile, DeletePromptFile } from '../../../wailsjs/go/main/App'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import { useConfigStore } from '@/stores/configStore'
import AppModal from '@/components/common/AppModal.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'

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
const isDeleting = ref(false)
let generation = 0
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
  const current = ++generation
  isLoading.value = true
  try {
    const loaded = await GetPromptFiles()
    if (current !== generation) return
    files.value = loaded
    originals.value = Object.fromEntries(files.value.map(item => [item.provider, item.content]))
  } catch (e: unknown) {
    if (current === generation) toast.error(t('prompt.loadFailed', { error: e instanceof Error ? e.message : String(e) }))
  } finally {
    if (current === generation) isLoading.value = false
  }
}

async function save() {
  if (isLoading.value || isSaving.value || isDeleting.value || isDesktopFilter.value) return
  // 逐个标签编辑时用户往往改了多个标签才点一次保存：
  // 只保存当前标签会让其余改动静默丢失，这里保存全部与磁盘不一致的标签
  const dirty = files.value.filter(item => dirtyProviders.value.includes(item.provider))
  const targets = (dirty.length ? dirty : files.value.filter(item => item.provider === activeTab.value))
    .map(item => ({ ...item }))
  if (!targets.length) {
    toast.error(t('prompt.notLoaded'))
    return
  }
  isSaving.value = true
  const current = generation
  try {
    for (const file of targets) {
      await SavePromptFile(file.provider, file.content)
      const saved = await GetPromptFile(file.provider)
      if (!saved.exists || saved.content !== file.content) throw new Error(t('prompt.verifyFailed'))
      if (current === generation) {
        // 只提交本次快照；保存期间输入的新草稿仍留在编辑器中。
        originals.value[file.provider] = saved.content
        const draft = fileOf(file.provider)
        if (draft) {
          draft.exists = saved.exists
          draft.path = saved.path
        }
      }
    }
    if (current === generation) emit('saved')
  } catch (e: unknown) {
    if (current === generation) toast.error(t('prompt.saveFailed', { error: e instanceof Error ? e.message : String(e) }))
  } finally {
    isSaving.value = false
  }
}

async function deleteFile(provider = activeTab.value) {
  if (isLoading.value || isSaving.value || isDeleting.value) return
  const current = generation
  isDeleting.value = true
  try {
    const ok = await confirm.show(
      t('prompt.deleteTitle'),
      t('prompt.deleteMsg', { name: provider.toUpperCase() }),
      'danger'
    )
    if (!ok || current !== generation) return
    await DeletePromptFile(provider)
    if (current !== generation) return

    const file = fileOf(provider)
    if (file) {
      file.content = ''
      file.exists = false
    }
    originals.value[provider] = ''

    emit('saved')
  } catch (e: unknown) {
    if (current === generation) toast.error(t('prompt.deleteFailed', { error: e instanceof Error ? e.message : String(e) }))
  } finally {
    isDeleting.value = false
  }
}

function close() {
  emit('close')
}

watch(() => props.visible, (newVal) => {
  if (newVal) {
    loadFiles()
  } else {
    generation++
  }
}, { immediate: true })

onScopeDispose(() => { generation++ })

watch(() => configStore.currentFilter, (tool) => {
  if (tool === 'claude' || tool === 'codex' || tool === 'antigravity' || tool === 'opencode' || tool === 'grok') {
    activeTab.value = tool
  }
}, { immediate: true })
</script>
