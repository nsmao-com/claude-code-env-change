<template>
  <section class="pt-0">
    <Card class="gap-0 overflow-hidden py-0">
      <CurrentAppliedBar
        @edit="onBarEdit"
        @apply="onBarApply"
        @duplicate="onBarDuplicate"
        @delete="onBarDelete"
      />
      <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
        <div class="flex items-center gap-3">
          <div class="relative">
            <Search class="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input id="config-search" v-model="searchQuery" class="w-[200px] rounded-full bg-muted/70 pl-8" :placeholder="t('envList.search')" />
          </div>
          <Button variant="outline" size="sm" :disabled="importing" @click="$emit('import-local')">
            <Upload />
            {{ importing ? t('envList.importing') : t('envList.importLocal') }}
          </Button>
          <Button variant="outline" size="sm" @click="$emit('import-json')">
            <FileJson />
            {{ t('envList.importJson') }}
          </Button>
          <Button variant="outline" size="sm" @click="$emit('import-clipboard')" :title="t('envList.clipboardTip')">
            <ClipboardPaste />
            {{ t('envList.fromClipboard') }}
          </Button>
          <Button variant="outline" size="sm" :disabled="addingOfficial" @click="$emit('add-official')">
            <KeyRound />
            {{ addingOfficial ? t('envList.adding') : t('envList.officialLogin') }}
          </Button>
        </div>
        <div class="flex items-center gap-2">
          <SegmentedPills
            :model-value="viewMode"
            layout-id="env-view-pill"
            dense
            :items="[{ value: 'list', label: t('envList.viewList') }, { value: 'cards', label: t('envList.viewCards') }]"
            @update:model-value="onView"
          >
            <template #default="{ item }">
              <List v-if="item.value === 'list'" class="size-3.5" />
              <LayoutGrid v-else class="size-3.5" />
            </template>
          </SegmentedPills>
          <Button size="sm" @click="$emit('add')">
            <Plus />
            {{ t('envList.new') }}
          </Button>
        </div>
      </div>

      <motion.div
        v-if="totalCount === 0"
        class="px-6 py-16"
        :initial="fadeEnter.initial"
        :animate="fadeEnter.animate"
        :transition="fadeEnter.transition"
      >
        <Empty class="min-h-0 items-start border-0 p-0 text-left">
          <EmptyHeader class="items-start text-left">
            <EmptyTitle>{{ t('envList.emptyTitle') }}</EmptyTitle>
            <EmptyDescription>{{ t('envList.emptyDesc') }}</EmptyDescription>
          </EmptyHeader>
          <EmptyContent class="flex-row items-start">
            <Button @click="$emit('add')">{{ t('envList.newConfig') }}</Button>
            <Button variant="outline" @click="$emit('import-json')">{{ t('envList.importJson') }}</Button>
          </EmptyContent>
        </Empty>
      </motion.div>

      <motion.div
        v-else-if="filteredConfigs.length === 0"
        class="px-6 py-16"
        :initial="fadeEnter.initial"
        :animate="fadeEnter.animate"
        :transition="fadeEnter.transition"
      >
        <Empty class="min-h-0 items-start border-0 p-0 text-left">
          <EmptyHeader class="items-start text-left">
            <EmptyTitle>{{ searchQuery.trim() ? t('envList.noMatch') : t('envList.noneFor', { name: filterLabel }) }}</EmptyTitle>
            <EmptyDescription>
              {{ searchQuery.trim() ? t('envList.noMatchDesc', { count: totalCount }) : t('envList.noneForDesc') }}
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent class="items-start">
            <Button @click="$emit('add')">{{ t('envList.newConfig') }}</Button>
          </EmptyContent>
        </Empty>
      </motion.div>

      <div
        v-else
        ref="gridRef"
        :class="displayMode === 'cards'
          ? 'grid grid-cols-[repeat(auto-fill,minmax(min(100%,260px),1fr))] gap-3 px-4 pb-4 pt-1'
          : 'divide-y'"
      >
        <template v-if="displayMode === 'cards'">
          <ConfigCard
            v-for="(config, index) in filteredConfigs"
            :key="`${config.name}-${config.provider}`"
            :config="config"
            :index="index"
            :is-active="isEnvActive(config.name, config.provider)"
            @dblclick="$emit('apply', getOriginalIndex(config.name, config.provider))"
            :title="t('envList.itemTitle', { name: config.name })"
            @click="$emit('edit', getOriginalIndex(config.name, config.provider))"
            @apply="$emit('apply', getOriginalIndex(config.name, config.provider))"
            @duplicate="$emit('duplicate', getOriginalIndex(config.name, config.provider))"
            @edit="$emit('edit', getOriginalIndex(config.name, config.provider))"
            @delete="$emit('delete', getOriginalIndex(config.name, config.provider))"
          />
        </template>
        <template v-else>
          <ConfigListItem
            v-for="(config, index) in filteredConfigs"
            :key="`${config.name}-${config.provider}`"
            :config="config"
            :index="index"
            nested
            :is-active="isEnvActive(config.name, config.provider)"
            @dblclick="$emit('apply', getOriginalIndex(config.name, config.provider))"
            @click="$emit('edit', getOriginalIndex(config.name, config.provider))"
            @apply="$emit('apply', getOriginalIndex(config.name, config.provider))"
            @duplicate="$emit('duplicate', getOriginalIndex(config.name, config.provider))"
            @edit="$emit('edit', getOriginalIndex(config.name, config.provider))"
            @delete="$emit('delete', getOriginalIndex(config.name, config.provider))"
          />
        </template>
      </div>
    </Card>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { motion } from 'motion-v'
import Sortable from 'sortablejs'
import { fadeEnter } from '@/lib/motion'
import { ClipboardPaste, FileJson, KeyRound, LayoutGrid, List, Plus, Search, Upload } from '@lucide/vue' 
import type { EnvConfig, Provider } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'
import ConfigCard from './ConfigCard.vue'
import ConfigListItem from './ConfigListItem.vue'
import CurrentAppliedBar from './CurrentAppliedBar.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'

const { t } = useI18n()

interface Props {
  configs: EnvConfig[]
  importing?: boolean
  addingOfficial?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  add: []
  edit: [index: number]
  apply: [index: number]
  duplicate: [index: number]
  delete: [index: number]
  reorder: [names: string[]]
  'import-local': []
  'add-official': []
  'import-json': []
  'import-clipboard': []
}>()

const configStore = useConfigStore()
const gridRef = ref<HTMLElement>()
const searchQuery = ref('')
let sortableInstance: InstanceType<typeof Sortable> | null = null

type ViewMode = 'cards' | 'list'
const viewMode = ref<ViewMode>('list')
const viewModeStorageKey = 'claudia_config_view_mode'
const userPickedView = ref(false)

const currentFilter = computed(() => configStore.currentFilter)
const totalCount = computed(() => configStore.environments.length)
const filterLabel = computed(() => {
  if (currentFilter.value === 'all') return t('envList.all')
  if (currentFilter.value === 'claude') return 'Claude Code'
  if (currentFilter.value === 'claude_desktop') return 'Claude Desktop'
  if (currentFilter.value === 'codex') return 'Codex'
  if (currentFilter.value === 'antigravity') return 'Antigravity'
  if (currentFilter.value === 'opencode') return 'OpenCode'
  if (currentFilter.value === 'grok') return 'Grok'
  return t('envList.all')
})

// 后端按 provider::name 定位配置（名称只在服务商内唯一）
function envKey(env: EnvConfig) {
  return `${env.provider || 'claude'}::${env.name}`
}

const filteredConfigs = computed(() => {
  if (!searchQuery.value.trim()) return props.configs
  const query = searchQuery.value.toLowerCase()
  return props.configs.filter(config =>
    config.name.toLowerCase().includes(query)
    || config.description?.toLowerCase().includes(query)
    || config.provider.toLowerCase().includes(query),
  )
})

const displayMode = computed<ViewMode>(() => {
  if (userPickedView.value) return viewMode.value
  return filteredConfigs.value.length >= 8 ? 'list' : viewMode.value
})

function onView(value: string) {
  if (value !== 'list' && value !== 'cards') return
  userPickedView.value = true
  viewMode.value = value
  try {
    localStorage.setItem(viewModeStorageKey, value)
  } catch { /* ignore */ }
  nextTick(() => initSortable())
}

function getOriginalIndex(name: string, provider?: string): number {
  return props.configs.findIndex(c => c.name === name && (provider === undefined || c.provider === provider))
}

// CurrentAppliedBar 事件带 (name, provider) 两个参数，模板内联写法拿不到第二个参数，这里转发
function onBarEdit(name: string, provider?: string) { emitByName('edit', name, provider) }
function onBarApply(name: string, provider?: string) { emitByName('apply', name, provider) }
function onBarDuplicate(name: string, provider?: string) { emitByName('duplicate', name, provider) }
function onBarDelete(name: string, provider?: string) { emitByName('delete', name, provider) }

function emitByName(type: 'edit' | 'apply' | 'duplicate' | 'delete', name: string, provider?: string) {
  const index = getOriginalIndex(name, provider)
  if (index < 0) return
  if (type === 'edit') emit('edit', index)
  else if (type === 'apply') emit('apply', index)
  else if (type === 'duplicate') emit('duplicate', index)
  else emit('delete', index)
}

function isEnvActive(name: string, provider: Provider): boolean {
  return configStore.isEnvActive(name, provider)
}

function initSortable() {
  if (!gridRef.value) return
  if (sortableInstance) {
    sortableInstance.destroy()
    sortableInstance = null
  }
  if (searchQuery.value.trim()) return
  sortableInstance = Sortable.create(gridRef.value, {
    animation: 150,
    ghostClass: 'opacity-50',
    filter: 'button, input, textarea, [data-slot="button"]',
    preventOnFilter: true,
    onEnd: async (evt: { oldIndex?: number; newIndex?: number }) => {
      if (isReordering.value) return
      if (evt.oldIndex === undefined || evt.newIndex === undefined || evt.oldIndex === evt.newIndex) return
      isReordering.value = true
      try {
        await applyReorder(evt)
      } catch (err) {
        // 失败必须回滚重渲染并提示，否则 DOM 顺序与 store 永久不一致
        toast.error(t('envList.reorderFailed', { error: err instanceof Error ? err.message : String(err) }))
        await configStore.loadConfig()
      } finally {
        isReordering.value = false
      }
    },
  })
}

const isReordering = ref(false)
const toast = useToast()

async function applyReorder(evt: { oldIndex?: number; newIndex?: number }) {
      if (evt.oldIndex === undefined || evt.newIndex === undefined || evt.oldIndex === evt.newIndex) return
      {
      const allEnvs = configStore.environments
      // 配置名只在同一服务商内唯一，排序必须按 provider::name 定位，
      // 否则跨服务商的同名配置会被排到对方的位置上。
      const allKeys = allEnvs.map(envKey)
      const displayedKeys = filteredConfigs.value.map(envKey)
      const movedKey = displayedKeys[evt.oldIndex]
      const targetKey = displayedKeys[evt.newIndex]
      if (!movedKey || !targetKey) return
      if (currentFilter.value === 'all') {
        const fromIndex = allKeys.indexOf(movedKey)
        const toIndex = allKeys.indexOf(targetKey)
        if (fromIndex < 0 || toIndex < 0) return
        const newOrder = [...allKeys]
        newOrder.splice(fromIndex, 1)
        newOrder.splice(toIndex, 0, movedKey)
        await configStore.reorderEnvs(newOrder)
        return
      }
      const newFilteredOrder = [...displayedKeys]
      newFilteredOrder.splice(evt.oldIndex, 1)
      newFilteredOrder.splice(evt.newIndex, 0, movedKey)
      const newOrder: string[] = []
      let filteredIdx = 0
      for (const env of allEnvs) {
        if (env.provider === currentFilter.value) {
          newOrder.push(newFilteredOrder[filteredIdx])
          filteredIdx++
        } else {
          newOrder.push(envKey(env))
        }
      }
      await configStore.reorderEnvs(newOrder)
  }
}


onMounted(() => {
  try {
    const saved = localStorage.getItem(viewModeStorageKey)
    if (saved === 'cards' || saved === 'list') {
      viewMode.value = saved
      userPickedView.value = true
    }
  } catch { /* ignore */ }
  initSortable()
})

watch([currentFilter, searchQuery], () => nextTick(() => initSortable()))
watch(() => props.configs, () => nextTick(() => initSortable()), { deep: true })
</script>
