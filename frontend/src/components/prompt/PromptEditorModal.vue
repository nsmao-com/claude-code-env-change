<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" :tool-filter="embedded">
    <template #header>
      <div>
        <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">提示词</h1>
        <p class="mt-2 text-sm text-muted-foreground">编辑五个平台的自定义提示词，保存会直接覆盖对应本机文件；Claude Desktop 使用独立配置文件</p>
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
          Claude Desktop 没有独立的全局提示词文件。请在环境配置中编辑它的 configLibrary JSON；这里显示的是 Claude Code、Codex、Antigravity、OpenCode 和 Grok 的提示词文件。
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
                <Badge v-if="fileOf(tab.value)?.exists">已存在</Badge>
                <Badge v-else variant="outline">未创建</Badge>
              </div>
              <Button
                v-if="fileOf(tab.value)?.exists"
                variant="destructive"
                size="sm"
                @click="deleteFile(tab.value)"
              >
                <Trash2 />
                删除
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
          修改后需要重启 CLI 工具生效
        </p>
        <div class="flex items-center gap-3">
          <Button v-if="!embedded" variant="secondary" @click="close">取消</Button>
          <Button :disabled="isSaving || !editableFile || isDesktopFilter" @click="save">
            <Loader2 v-if="isSaving" class="animate-spin" />
            <Save v-else />
            保存
          </Button>
        </div>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
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
  const placeholders: Record<string, string> = {
    claude: `# CLAUDE.md 示例

## 项目规则
- 使用 TypeScript 编写代码
- 遵循 ESLint 规则
- 不要创建测试文件

## 代码风格
- 使用函数式编程风格
- 注释使用中文`,
    codex: `# AGENTS.md 示例

## Agent 指令
- 优先使用函数式编程模式
- 注释使用中文
- 代码风格遵循项目规范`,
    antigravity: `# GEMINI.md 示例

## Gemini 指令
- 回复使用中文
- 代码风格遵循 Google Style Guide
- 简洁明了地回答问题`,
    grok: `# GROK.md 示例

## Grok 指令
- 回复使用中文
- 改代码前先看现有结构
- 不要引入无关依赖`,
    opencode: `# AGENTS.md 示例

## OpenCode 指令
- 回复使用中文
- 改代码前先看现有结构
- 遵循项目现有的代码风格`
  }
  return placeholders[key] || ''
}

async function loadFiles() {
  isLoading.value = true
  try {
    files.value = await GetPromptFiles()
  } catch (e: any) {
    toast.error('加载失败: ' + (e?.message || String(e)))
  } finally {
    isLoading.value = false
  }
}

async function save() {
  const provider = activeTab.value
  const file = fileOf(provider)
  if (!file) {
    toast.error('提示词文件尚未加载完成，请稍后再试')
    return
  }
  const content = file.content
  isSaving.value = true
  try {
    await SavePromptFile(provider, content)
    const refreshed = await GetPromptFiles()
    const saved = refreshed.find(item => item.provider === provider)
    if (!saved || saved.content !== content) throw new Error('保存后读取到的内容与编辑内容不一致')
    files.value = files.value.map(item => item.provider === provider ? saved : item)
    emit('saved')
  } catch (e: any) {
    toast.error('保存失败: ' + (e?.message || String(e)))
  } finally {
    isSaving.value = false
  }
}

async function deleteFile(provider = activeTab.value) {
  const ok = await confirm.show(
    '删除提示词',
    `确定要删除 ${provider.toUpperCase()} 的提示词文件吗？`,
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
    toast.error('删除失败: ' + (e?.message || String(e)))
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
