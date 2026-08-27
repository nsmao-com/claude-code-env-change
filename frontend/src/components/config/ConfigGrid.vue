<template>
  <section class="px-8 pt-3">
    <Card class="gap-0 overflow-hidden py-0">
      <div class="flex items-center justify-between gap-3 px-4 py-3">
        <div class="relative">
          <Search class="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input id="config-search" v-model="searchQuery" class="w-[240px] rounded-full bg-muted/70 pl-8" placeholder="搜索名称、描述" />
        </div>
        <ToggleGroup type="single" variant="pill" :spacing="1" :model-value="viewMode" size="sm" class="rounded-full bg-muted p-1" @update:model-value="onView">
          <ToggleGroupItem value="list" aria-label="列表">
            <List class="size-3.5" />
          </ToggleGroupItem>
          <ToggleGroupItem value="cards" aria-label="卡片">
            <LayoutGrid class="size-3.5" />
          </ToggleGroupItem>
        </ToggleGroup>
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
            <EmptyTitle>还没有环境</EmptyTitle>
            <EmptyDescription>为 Claude、Codex、Gemini 或 OpenClaw 建一条配置，点应用后会写入对应 CLI。</EmptyDescription>
          </EmptyHeader>
          <EmptyContent class="items-start">
            <Button @click="$emit('add')">新建配置</Button>
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
            <EmptyTitle>{{ searchQuery.trim() ? '没有匹配的配置' : `没有 ${filterLabel} 配置` }}</EmptyTitle>
            <EmptyDescription>
              {{ searchQuery.trim() ? `换个关键词，或清空搜索看全部 ${totalCount} 条。` : '当前筛选下是空的。可以新建一条，或切回全部。' }}
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent class="items-start">
            <Button @click="$emit('add')">新建配置</Button>
          </EmptyContent>
        </Empty>
      </motion.div>

      <div
        v-else
        ref="gridRef"
        :class="displayMode === 'cards'
          ? 'grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3 p-4 pt-0'
          : 'divide-y'"
      >
        <template v-if="displayMode === 'cards'">
          <ConfigCard
            v-for="(config, index) in filteredConfigs"
            :key="config.name"
            :config="config"
            :index="index"
            :is-active="isEnvActive(config.name, config.provider)"
            @click="$emit('edit', getOriginalIndex(config.name))"
            @apply="$emit('apply', getOriginalIndex(config.name))"
            @duplicate="$emit('duplicate', getOriginalIndex(config.name))"
            @edit="$emit('edit', getOriginalIndex(config.name))"
            @delete="$emit('delete', getOriginalIndex(config.name))"
            @test-latency="$emit('testLatency', getOriginalIndex(config.name))"
          />
        </template>
        <template v-else>
          <ConfigListItem
            v-for="(config, index) in filteredConfigs"
            :key="config.name"
            :config="config"
            :index="index"
            nested
            :is-active="isEnvActive(config.name, config.provider)"
            @click="$emit('edit', getOriginalIndex(config.name))"
            @apply="$emit('apply', getOriginalIndex(config.name))"
            @duplicate="$emit('duplicate', getOriginalIndex(config.name))"
            @edit="$emit('edit', getOriginalIndex(config.name))"
            @delete="$emit('delete', getOriginalIndex(config.name))"
            @test-latency="$emit('testLatency', getOriginalIndex(config.name))"
          />
        </template>
      </div>
    </Card>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { motion } from 'motion-v'
import Sortable from 'sortablejs'
import { fadeEnter } from '@/lib/motion'
import { LayoutGrid, List, Search } from '@lucide/vue'
import type { EnvConfig } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import ConfigCard from './ConfigCard.vue'
import ConfigListItem from './ConfigListItem.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'

interface Props {
  configs: EnvConfig[]
}

const props = defineProps<Props>()
defineEmits<{
  add: []
  edit: [index: number]
  apply: [index: number]
  duplicate: [index: number]
  delete: [index: number]
  reorder: [names: string[]]
  testLatency: [index: number]
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
  if (currentFilter.value === 'all') return '全部'
  if (currentFilter.value === 'claude') return 'Claude'
  if (currentFilter.value === 'codex') return 'Codex'
  if (currentFilter.value === 'gemini') return 'Gemini'
  return 'OpenClaw'
})

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

function onView(value: unknown) {
  if (value !== 'list' && value !== 'cards') return
  userPickedView.value = true
  viewMode.value = value
  try {
    localStorage.setItem(viewModeStorageKey, value)
  } catch { /* ignore */ }
  nextTick(() => initSortable())
}

function getOriginalIndex(name: string): number {
  return props.configs.findIndex(c => c.name === name)
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
    onEnd: async (evt: { oldIndex?: number; newIndex?: number }) => {
      if (evt.oldIndex === undefined || evt.newIndex === undefined || evt.oldIndex === evt.newIndex) return
      const allEnvs = configStore.environments
      const allNames = allEnvs.map(c => c.name)
      const displayedNames = filteredConfigs.value.map(c => c.name)
      const movedName = displayedNames[evt.oldIndex]
      const targetName = displayedNames[evt.newIndex]
      if (currentFilter.value === 'all') {
        const fromIndex = allNames.indexOf(movedName)
        const toIndex = allNames.indexOf(targetName)
        const newOrder = [...allNames]
        newOrder.splice(fromIndex, 1)
        newOrder.splice(toIndex, 0, movedName)
        await configStore.reorderEnvs(newOrder)
        return
      }
      const newFilteredOrder = [...displayedNames]
      newFilteredOrder.splice(evt.oldIndex, 1)
      newFilteredOrder.splice(evt.newIndex, 0, movedName)
      const newOrder: string[] = []
      let filteredIdx = 0
      for (const name of allNames) {
        const env = allEnvs.find(e => e.name === name)
        if (env && env.provider === currentFilter.value) {
          newOrder.push(newFilteredOrder[filteredIdx])
          filteredIdx++
        } else {
          newOrder.push(name)
        }
      }
      await configStore.reorderEnvs(newOrder)
    },
  })
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
