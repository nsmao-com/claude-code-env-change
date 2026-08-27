<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" :tool-filter="embedded" :close-on-overlay="false">
    <template #header>
      <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">Skills</h1>
      <p class="mt-2 text-sm text-muted-foreground">管理 Claude/Codex/Gemini/OpenCode 的自定义 SKILL.md</p>
    </template>

    <div class="mb-4 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <Button size="sm" @click="openCreate">
          <Plus />
          新建
        </Button>
        <Button variant="outline" size="sm" @click="showPresets = !showPresets">
          <Store />
          技能库
        </Button>
        <Button variant="outline" size="sm" @click="skillStore.loadSkills">
          <RefreshCw />
          刷新
        </Button>
      </div>
      <span class="text-xs text-muted-foreground">
        共 {{ skillStore.skillCount }} 个
      </span>
    </div>

    <div v-if="showPresets" class="mb-4 rounded-xl border border-dashed border-border bg-secondary/20 p-4">
      <div class="mb-3 flex items-center justify-between">
        <span class="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">内置技能库</span>
        <span class="text-[10px] text-muted-foreground">点击「导入」后可选择启用平台</span>
      </div>
      <Empty v-if="presets.length === 0" class="min-h-0 border-0 py-3">
        <EmptyHeader>
          <Loader2 v-if="isLoadingPresets" class="size-5 animate-spin text-muted-foreground" />
          <EmptyTitle>{{ isLoadingPresets ? '加载中...' : '暂无预设' }}</EmptyTitle>
        </EmptyHeader>
      </Empty>
      <div v-else class="grid grid-cols-2 gap-2">
        <Card v-for="preset in presets" :key="preset.name" size="sm">
          <CardHeader>
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <div class="flex items-center gap-1.5">
                  <CardTitle class="truncate">{{ preset.name }}</CardTitle>
                  <Badge v-if="installedNames.has(preset.name)" variant="secondary">已导入</Badge>
                </div>
                <CardDescription class="line-clamp-2">{{ preset.description }}</CardDescription>
              </div>
              <Button variant="outline" size="sm" class="shrink-0" @click="importPreset(preset)">
                <Download />
                导入
              </Button>
            </div>
          </CardHeader>
        </Card>
      </div>
    </div>

    <Empty
      v-if="filteredSkills.length === 0 && !skillStore.isLoading"
      class="min-h-[240px]"
    >
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <Layers />
        </EmptyMedia>
        <EmptyTitle>{{ skillStore.skillCount === 0 ? '暂无 Skills' : '该平台暂无 Skills' }}</EmptyTitle>
        <EmptyDescription>点击“新建”添加一个自定义 Skill</EmptyDescription>
      </EmptyHeader>
    </Empty>

    <div v-else-if="skillStore.isLoading" class="flex items-center justify-center py-12">
      <Loader2 class="size-8 animate-spin text-muted-foreground" />
    </div>

    <ScrollArea v-else class="h-[50vh] pr-2">
      <div class="space-y-3">
        <Card v-for="skill in filteredSkills" :key="skill.name" size="sm">
          <CardHeader>
            <div class="flex items-start justify-between gap-4">
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <CardTitle class="truncate">{{ skill.name }}</CardTitle>
                  <Badge
                    v-if="!skill.has_frontmatter || !skill.has_name || !skill.has_description"
                    variant="destructive"
                    title="SKILL.md frontmatter 可能不完整"
                  >
                    格式问题
                  </Badge>
                </div>
                <CardDescription class="mt-1 whitespace-pre-line">
                  {{ skill.description || skill.frontmatter_error || '（未提供 description）' }}
                </CardDescription>
                <div class="mt-3 flex flex-wrap gap-2">
                  <Badge variant="outline" class="gap-1">
                    <BrandIcon provider="claude" class="size-3" />
                    Claude:
                    <span :class="skill.enabled_in_claude ? 'text-green-600' : 'text-muted-foreground'">
                      {{ skill.enabled_in_claude ? '已安装' : '未安装' }}
                    </span>
                  </Badge>
                  <Badge variant="outline" class="gap-1">
                    <BrandIcon provider="codex" class="size-3" />
                    Codex:
                    <span :class="skill.enabled_in_codex ? 'text-green-600' : 'text-muted-foreground'">
                      {{ skill.enabled_in_codex ? '已安装' : '未安装' }}
                    </span>
                  </Badge>
                  <Badge variant="outline" class="gap-1">
                    <BrandIcon provider="gemini" class="size-3" />
                    Gemini:
                    <span :class="skill.enabled_in_gemini ? 'text-green-600' : 'text-muted-foreground'">
                      {{ skill.enabled_in_gemini ? '已安装' : '未安装' }}
                    </span>
                  </Badge>
                  <Badge variant="outline" class="gap-1">
                    <BrandIcon provider="opencode" class="size-3" />
                    OpenCode:
                    <span :class="skill.enabled_in_opencode ? 'text-green-600' : 'text-muted-foreground'">
                      {{ skill.enabled_in_opencode ? '已安装' : '未安装' }}
                    </span>
                  </Badge>
                  <Badge variant="outline" class="gap-1">
                    <BrandIcon provider="grok" class="size-3" />
                    Grok:
                    <span :class="skill.enabled_in_grok ? 'text-green-600' : 'text-muted-foreground'">
                      {{ skill.enabled_in_grok ? '已安装' : '未安装' }}
                    </span>
                  </Badge>
                </div>
              </div>
              <div class="flex shrink-0 items-center gap-2">
                <Button variant="outline" size="sm" @click="openEdit(skill)">
                  <Pencil />
                  编辑
                </Button>
                <Button variant="destructive" size="sm" @click="remove(skill)">
                  <Trash2 />
                  删除
                </Button>
              </div>
            </div>
          </CardHeader>
        </Card>
      </div>
    </ScrollArea>

    <SkillEditModal v-model="showEditModal" :edit-skill="editingSkill" @saved="onSaved" />
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Download, Layers, Loader2, Pencil, Plus, RefreshCw, Store, Trash2 } from '@lucide/vue'
import type { Skill, SkillPreset } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import SkillEditModal from './SkillEditModal.vue'
import { useSkillStore } from '@/stores/skillStore'
import { skillService } from '@/services/skillService'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useConfigStore } from '@/stores/configStore'
import { toolToPlatform } from '@/lib/workspace'

type PlatformFilter = 'all' | 'claude-code' | 'codex' | 'gemini' | 'opencode' | 'grok'

interface Props {
  modelValue: boolean
  embedded?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const toast = useToast()
const confirm = useConfirm()
const skillStore = useSkillStore()
const configStore = useConfigStore()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const currentPlatform = ref<PlatformFilter>('all')

const filteredSkills = computed(() => {
  if (currentPlatform.value === 'all') return skillStore.skills
  return skillStore.skills.filter(s => s.enable_platform?.includes(currentPlatform.value))
})

watch(isOpen, async (open) => {
  if (open) {
    await skillStore.loadSkills()
    loadPresets()
  } else {
    showPresets.value = false
  }}, { immediate: true })

watch(() => configStore.currentFilter, (tool) => {
  currentPlatform.value = toolToPlatform(tool) as PlatformFilter
}, { immediate: true })

const showPresets = ref(false)
const presets = ref<SkillPreset[]>([])
const isLoadingPresets = ref(false)

const installedNames = computed(() => new Set(skillStore.skills.map((s) => s.name)))

async function loadPresets() {
  isLoadingPresets.value = true
  try {
    presets.value = await skillService.getPresets()
  } catch {
    presets.value = []
  } finally {
    isLoadingPresets.value = false
  }
}

function importPreset(preset: SkillPreset) {
  editingSkill.value = {
    name: preset.name,
    content: preset.content,
    enable_platform: ['claude-code', 'codex', 'gemini'],
    enabled_in_claude: false,
    enabled_in_codex: false,
    enabled_in_gemini: false,
    enabled_in_opencode: false,
    enabled_in_grok: false,
    frontmatter_name: preset.name,
    description: preset.description,
    has_frontmatter: true,
    has_name: true,
    has_description: true,
    frontmatter_error: ''
  }
  showEditModal.value = true
}

const showEditModal = ref(false)
const editingSkill = ref<Skill | null>(null)

function openCreate() {
  editingSkill.value = null
  showEditModal.value = true
}

function openEdit(skill: Skill) {
  editingSkill.value = skill
  showEditModal.value = true
}

async function remove(skill: Skill) {
  const ok = await confirm.show(
    '删除 Skill',
    `确定要删除 “${skill.name}” 吗？将从已安装的平台移除 SKILL.md（不会强制删除目录内的其他文件）。`,
    'danger'
  )
  if (!ok) return
  try {
    await skillStore.deleteSkill(skill.name)
    toast.success('Skill 已删除')
  } catch (e: any) {
    toast.error('删除失败: ' + (e?.message || String(e)))
  }
}

function onSaved() {
  showEditModal.value = false
}
</script>
