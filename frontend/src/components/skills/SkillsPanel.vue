<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" :tool-filter="embedded" width="wide" :close-on-overlay="false">
    <template #header>
      <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">Skills</h1>
      <p class="mt-2 text-sm text-muted-foreground">{{ t('skills.panelHint') }}</p>
    </template>

    <div class="flex h-full min-h-0 flex-1 flex-col overflow-hidden">
    <div class="mb-4 flex shrink-0 flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <Button size="sm" @click="openCreate">
          <Plus />
          {{ t('skills.new') }}
        </Button>
        <Button variant="outline" size="sm" @click="toggleMarket">
          <Store />
          {{ t('skills.library') }}
        </Button>
        <Button variant="outline" size="sm" :disabled="isRefreshing" @click="refreshSkills">
          <Loader2 v-if="isRefreshing" class="animate-spin" />
          <RefreshCw v-else />
          {{ isRefreshing ? t('skills.refreshing') : t('skills.refresh') }}
        </Button>
        <ApplyToPlatformMenu
          :disabled="skillStore.skillCount === 0"
          :applying="isApplying"
          @apply="applyToPlatform"
        />
      </div>
      <div class="flex items-center gap-2">
        <SegmentedPills
          :model-value="viewMode"
          layout-id="skills-view-pill"
          dense
          :items="[{ value: 'list', label: t('skills.viewList') }, { value: 'cards', label: t('skills.viewCards') }]"
          @update:model-value="onView"
        >
          <template #default="{ item }">
            <List v-if="item.value === 'list'" class="size-3.5" />
            <LayoutGrid v-else class="size-3.5" />
          </template>
        </SegmentedPills>
        <span class="text-xs text-muted-foreground">
          {{ t('skills.total', { count: skillStore.skillCount }) }}
        </span>
      </div>
    </div>

    <div v-if="showPresets" class="mb-4 flex max-h-[36vh] min-h-0 shrink-0 flex-col overflow-hidden rounded-xl border border-dashed border-border bg-secondary/20 p-4">
      <div class="mb-3 flex shrink-0 flex-wrap items-center justify-between gap-3">
        <SegmentedPills
          :model-value="marketSource"
          layout-id="skills-market-source"
          dense
          :items="marketSources"
          @update:model-value="onMarketSource"
        />
        <Input v-model="marketQuery" class="w-[200px]" :placeholder="t('skills.search')" />
      </div>
      <p class="mb-3 shrink-0 text-[10px] text-muted-foreground">{{ t('skills.marketSourceHint') }}</p>
      <Empty v-if="marketItems.length === 0 && !isLoadingPresets" class="min-h-0 border-0 py-3">
        <EmptyHeader>
          <EmptyTitle>{{ marketError || t('skills.noSkills') }}</EmptyTitle>
        </EmptyHeader>
      </Empty>
      <div v-else-if="isLoadingPresets" class="flex justify-center py-6">
        <Loader2 class="size-5 animate-spin text-muted-foreground" />
      </div>
      <div v-else class="min-h-0 flex-1 overflow-y-auto pr-1">
        <div class="grid grid-cols-2 gap-3">
          <Card v-for="item in marketItems" :key="item.id" size="sm">
            <CardHeader>
              <div class="flex min-w-0 items-start justify-between gap-2">
                <div class="min-w-0 flex-1 overflow-hidden">
                  <div class="flex min-w-0 items-center gap-1.5">
                    <AppTooltip :content="item.name" wrap class="min-w-0 flex-1">
                      <CardTitle>{{ item.name }}</CardTitle>
                    </AppTooltip>
                    <Badge v-if="installedNames.has(item.name)" variant="secondary" class="shrink-0">{{ t('skills.installed') }}</Badge>
                  </div>
                  <CardDescription class="line-clamp-2">{{ item.description }}</CardDescription>
                </div>
                <Button variant="outline" size="sm" class="shrink-0" :disabled="importingId === item.id" @click="importMarketItem(item)">
                  <Loader2 v-if="importingId === item.id" class="animate-spin" />
                  <Download v-else />
                  {{ t('skills.import') }}
                </Button>
              </div>
            </CardHeader>
          </Card>
        </div>
      </div>
    </div>

    <Empty
      v-if="filteredSkills.length === 0 && !skillStore.isLoading"
      class="min-h-0 flex-1"
    >
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <Layers />
        </EmptyMedia>
        <EmptyTitle>{{ skillStore.skillCount === 0 ? t('skills.emptyAll') : t('skills.emptyPlatform') }}</EmptyTitle>
        <EmptyDescription>{{ t('skills.emptyDesc') }}</EmptyDescription>
      </EmptyHeader>
    </Empty>

    <div v-else-if="skillStore.isLoading" class="flex min-h-0 flex-1 items-center justify-center">
      <Loader2 class="size-8 animate-spin text-muted-foreground" />
    </div>

    <div v-else class="min-h-0 flex-1 overflow-y-auto pr-2">
      <div :class="viewMode === 'cards' ? 'grid grid-cols-2 gap-3 pb-2' : 'flex flex-col gap-2.5 pb-2'">
        <SkillCard
          v-for="skill in filteredSkills"
          :key="skill.name"
          :skill="skill"
          :compact="viewMode === 'list'"
          @edit="openEdit(skill)"
          @delete="remove(skill)"
          @toggle-platform="togglePlatform(skill, $event)"
        />
      </div>
    </div>
    </div>

    <SkillEditModal v-model="showEditModal" :edit-skill="editingSkill" @saved="onSaved" />
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { Download, Layers, LayoutGrid, List, Loader2, Plus, RefreshCw, Store } from '@lucide/vue'
import type { Skill, SkillMarketItem } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import ApplyToPlatformMenu from '@/components/common/ApplyToPlatformMenu.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import SkillCard from './SkillCard.vue'
import SkillEditModal from './SkillEditModal.vue'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import { useSkillStore } from '@/stores/skillStore'
import { skillService } from '@/services/skillService'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { useConfigStore } from '@/stores/configStore'
import { toolToPlatform } from '@/lib/workspace'

const { t } = useI18n()

type PlatformFilter = 'all' | 'claude-code' | 'codex' | 'antigravity' | 'opencode' | 'grok'

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

type ViewMode = 'cards' | 'list'
const viewMode = ref<ViewMode>('list')
const viewModeStorageKey = 'claudia_skills_view_mode'

function setViewMode(mode: ViewMode) {
  viewMode.value = mode
  try {
    localStorage.setItem(viewModeStorageKey, mode)
  } catch { /* ignore */ }
}

function onView(value: string) {
  if (value === 'cards' || value === 'list') setViewMode(value)
}

const currentPlatform = ref<PlatformFilter>('all')

const filteredSkills = computed(() => {
  if (currentPlatform.value === 'all') return skillStore.skills
  return skillStore.skills.filter(s => s.enable_platform?.includes(currentPlatform.value))
})

watch(isOpen, async (open) => {
  if (open) {
    try {
      const saved = localStorage.getItem(viewModeStorageKey)
      if (saved === 'cards' || saved === 'list') viewMode.value = saved
    } catch { /* ignore */ }
    await skillStore.loadSkills()
  } else {
    showPresets.value = false
  }
}, { immediate: true })

watch(() => configStore.currentFilter, (tool) => {
  currentPlatform.value = toolToPlatform(tool) as PlatformFilter
}, { immediate: true })

const isRefreshing = ref(false)
const isApplying = ref(false)
const showPresets = ref(false)
const marketItems = ref<SkillMarketItem[]>([])
const isLoadingPresets = ref(false)
const marketSource = ref('builtin')
const marketQuery = ref('')
const marketError = ref('')
const importingId = ref('')
const marketSources = computed(() => [
  { value: 'builtin', label: t('skills.source.builtin') },
  { value: 'online', label: t('skills.source.online') },
  { value: 'baoyu', label: t('skills.source.baoyu') },
  { value: 'engineering', label: t('skills.source.engineering') },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'vercel', label: 'Vercel' },
  { value: 'skillsmp', label: t('skills.source.skillsmp') },
])

async function refreshSkills() {
  isRefreshing.value = true
  try {
    await skillStore.loadSkills()
    toast.success(t('skills.refreshed'))
  } catch (e: any) {
    toast.error(t('skills.refreshFailed', { error: e?.message || String(e) }))
  } finally {
    isRefreshing.value = false
  }
}

const installedNames = computed(() => new Set(skillStore.skills.map((s) => s.name)))

function toggleMarket() {
  showPresets.value = !showPresets.value
  if (showPresets.value) loadMarket()
}

function onMarketSource(value: string) {
  if (!value) return
  marketSource.value = value
  loadMarket()
}

let marketTimer: number | undefined
watch(marketQuery, () => {
  if (!showPresets.value) return
  window.clearTimeout(marketTimer)
  marketTimer = window.setTimeout(() => loadMarket(), 350)
})
onBeforeUnmount(() => window.clearTimeout(marketTimer))

async function loadMarket() {
  isLoadingPresets.value = true
  marketError.value = ''
  try {
    marketItems.value = await skillService.searchMarketplace(marketSource.value, marketQuery.value.trim())
  } catch (e: any) {
    marketItems.value = []
    marketError.value = e?.message || t('skills.marketLoadFailed')
  } finally {
    isLoadingPresets.value = false
  }
}

async function importMarketItem(item: SkillMarketItem) {
  importingId.value = item.id
  try {
    const skill = await skillService.importMarketplace(item.id)
    editingSkill.value = {
      ...skill,
      enable_platform: importPlatforms(),
    }
    showEditModal.value = true
  } catch (e: any) {
    toast.error(t('skills.importFailed', { error: e?.message || String(e) }))
  } finally {
    importingId.value = ''
  }
}

function importPlatforms(): string[] {
  if (currentPlatform.value === 'all') return ['claude-code', 'codex', 'antigravity', 'opencode', 'grok']
  return [currentPlatform.value]
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
    t('skills.deleteTitle'),
    t('skills.deleteMsg', { name: skill.name }),
    'danger'
  )
  if (!ok) return
  try {
    await skillStore.deleteSkill(skill.name)
    toast.success(t('skills.deleted'))
  } catch (e: any) {
    toast.error(t('skills.deleteFailed', { error: e?.message || String(e) }))
  }
}

function onSaved() {
  showEditModal.value = false
}

async function applyToPlatform(platform: string) {
  isApplying.value = true
  try {
    const added = await skillStore.applyToPlatform(platform)
    const label = platformLabel(platform)
    if (added > 0) toast.success(t('skills.appliedCount', { count: added, platform: label }))
    else toast.success(t('skills.alreadyIn', { platform: label }))
  } catch (e: any) {
    toast.error(t('skills.applyFailed', { error: e?.message || String(e) }))
  } finally {
    isApplying.value = false
  }
}

async function togglePlatform(skill: Skill, platform: string) {
  try {
    const wasOn = skill.enable_platform?.includes(platform)
    await skillStore.togglePlatform(skill, platform)
    toast.success(wasOn ? t('skills.removedFrom', { platform: platformLabel(platform) }) : t('skills.addedTo', { platform: platformLabel(platform) }))
  } catch (e: any) {
    toast.error(t('skills.toggleFailed', { error: e?.message || String(e) }))
  }
}

function platformLabel(platform: string) {
  if (platform === 'claude-code') return 'Claude Code'
  if (platform === 'codex') return 'Codex'
  if (platform === 'antigravity') return 'Antigravity'
  if (platform === 'opencode') return 'OpenCode'
  if (platform === 'grok') return 'Grok'
  return platform
}
</script>
