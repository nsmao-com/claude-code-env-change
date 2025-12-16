<template>
  <div class="bento-item area-configs px-8 py-8">
    <div class="flex items-center justify-between mb-8">
      <div>
        <h2 class="text-3xl font-extrabold tracking-tight text-foreground uppercase">配置列表</h2>
        <p class="text-sm text-muted-foreground mt-1 font-mono">{{ filteredConfigs.length }} 个环境</p>
      </div>
      <!-- Search Input -->
      <div class="search-box relative group">
        <i class="fas fa-search search-icon absolute left-3 top-1/2 transform -translate-y-1/2 text-muted-foreground group-focus-within:text-foreground transition-colors"></i>
        <input
          v-model="searchQuery"
          type="text"
          placeholder="搜索..."
          class="search-input w-[240px] h-10 pl-9 pr-4 rounded-lg bg-transparent border border-border focus:border-foreground transition-all outline-none text-sm font-mono placeholder:text-muted-foreground/50"
        />
      </div>
    </div>

    <!-- Filter Tabs -->
    <div class="relative mb-8 p-1 bg-secondary/50 border border-border/50 rounded-full inline-flex backdrop-blur-sm">
      <!-- Glider -->
      <div
        class="absolute top-1 bottom-1 bg-white rounded-full transition-all duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] shadow-[0_2px_8px_rgba(0,0,0,0.08)] border border-black/5 dark:border-white/10"
        :style="gliderStyle"
      ></div>
      
      <button
        v-for="tab in filterTabs"
        :key="tab.value"
        ref="tabRefs"
        :class="['relative z-10 px-6 py-2 rounded-full text-xs font-bold uppercase tracking-wide transition-colors duration-200', { 'text-foreground': currentFilter === tab.value, 'text-muted-foreground hover:text-foreground/80': currentFilter !== tab.value }]"
        @click="setFilter(tab.value)"
      >
        {{ tab.label }}
      </button>
    </div>

    <!-- Empty State -->
    <div
      v-if="configs.length === 0"
      class="flex flex-col items-center justify-center py-20 text-muted-foreground border-2 border-dashed border-border rounded-xl mx-4"
    >
      <div class="w-16 h-16 rounded-full border border-border flex items-center justify-center mb-4 text-2xl">
        <i class="fas fa-inbox opacity-50"></i>
      </div>
      <p class="text-base font-bold text-foreground uppercase tracking-wide">暂无配置</p>
      <p class="text-sm mt-1 font-mono">创建新配置以开始使用</p>
    </div>

    <!-- Config Grid -->
    <div v-else ref="gridRef" class="config-grid-layout pb-8">
      <ConfigCard
        v-for="config in filteredConfigs"
        :key="config.name"
        :config="config"
        :is-active="isEnvActive(config.name, config.provider)"
        :data-index="getOriginalIndex(config.name)"
        @click="$emit('edit', getOriginalIndex(config.name))"
        @apply="$emit('apply', getOriginalIndex(config.name))"
        @duplicate="$emit('duplicate', getOriginalIndex(config.name))"
        @edit="$emit('edit', getOriginalIndex(config.name))"
        @delete="$emit('delete', getOriginalIndex(config.name))"
        @test-latency="$emit('testLatency', getOriginalIndex(config.name))"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import Sortable from 'sortablejs'
import type { EnvConfig, Provider } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import ConfigCard from './ConfigCard.vue'

interface Props {
  configs: EnvConfig[]
}

const props = defineProps<Props>()

defineEmits<{
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

// Tab Refs and Glider Logic
const tabRefs = ref<HTMLElement[]>([])
const gliderStyle = ref({
  left: '4px',
  width: '0px'
})

// Filter tabs
const filterTabs = [
  { label: '全部', value: 'all' as const },
  { label: 'CLAUDE', value: 'claude' as Provider },
  { label: 'CODEX', value: 'codex' as Provider },
  { label: 'GEMINI', value: 'gemini' as Provider }
]

const currentFilter = computed(() => configStore.currentFilter)

// Update glider position
function updateGlider() {
  nextTick(() => {
    const activeIndex = filterTabs.findIndex(t => t.value === currentFilter.value)
    if (activeIndex !== -1 && tabRefs.value[activeIndex]) {
      const el = tabRefs.value[activeIndex]
      gliderStyle.value = {
        left: `${el.offsetLeft}px`,
        width: `${el.offsetWidth}px`
      }
    }
  })
}

// Watch filter change to update glider
watch(currentFilter, () => {
  updateGlider()
})

// Update on mount
onMounted(() => {
  initSortable()
  // Wait for fonts/layout
  setTimeout(updateGlider, 100)
})

// Check if sorting is enabled (always enabled now)
const canSort = computed(() => {
  return !searchQuery.value.trim()
})

function setFilter(filter: Provider | 'all') {
  configStore.setFilter(filter)
}

// Filter configs by search query
const filteredConfigs = computed(() => {
  if (!searchQuery.value.trim()) {
    return props.configs
  }
  const query = searchQuery.value.toLowerCase()
  return props.configs.filter(config =>
    config.name.toLowerCase().includes(query) ||
    config.description?.toLowerCase().includes(query) ||
    config.provider.toLowerCase().includes(query)
  )
})

// Get original index from config name
function getOriginalIndex(name: string): number {
  return props.configs.findIndex(c => c.name === name)
}

function isEnvActive(name: string, provider: Provider): boolean {
  return configStore.isEnvActive(name, provider)
}

// Initialize or update Sortable instance
function initSortable() {
  if (!gridRef.value) return

  // Destroy existing instance
  if (sortableInstance) {
    sortableInstance.destroy()
    sortableInstance = null
  }

  // Only create sortable when no search active
  if (canSort.value) {
    sortableInstance = Sortable.create(gridRef.value, {
      animation: 150,
      ghostClass: 'sortable-ghost',
      dragClass: 'sortable-drag',
      onEnd: async (evt: { oldIndex?: number; newIndex?: number }) => {
        if (evt.oldIndex !== undefined && evt.newIndex !== undefined && evt.oldIndex !== evt.newIndex) {
          const allEnvs = configStore.environments
          const allNames = allEnvs.map(c => c.name)
          const displayedNames = filteredConfigs.value.map(c => c.name)

          const movedName = displayedNames[evt.oldIndex]
          const targetName = displayedNames[evt.newIndex]

          if (currentFilter.value === 'all') {
            // Simple case: reorder in full list
            const fromIndex = allNames.indexOf(movedName)
            const toIndex = allNames.indexOf(targetName)
            const newOrder = [...allNames]
            newOrder.splice(fromIndex, 1)
            newOrder.splice(toIndex, 0, movedName)
            await configStore.reorderEnvs(newOrder)
          } else {
            // Filtered case: reorder within the same provider
            // Build new order by replacing filtered items in their new order
            const newFilteredOrder = [...displayedNames]
            newFilteredOrder.splice(evt.oldIndex, 1)
            newFilteredOrder.splice(evt.newIndex, 0, movedName)

            // Rebuild full list: keep non-filtered items in place, update filtered items order
            const newOrder: string[] = []
            let filteredIdx = 0

            for (const name of allNames) {
              const env = allEnvs.find(e => e.name === name)
              if (env && env.provider === currentFilter.value) {
                // This is a filtered item, use new order
                newOrder.push(newFilteredOrder[filteredIdx])
                filteredIdx++
              } else {
                // Keep non-filtered item in place
                newOrder.push(name)
              }
            }

            await configStore.reorderEnvs(newOrder)
          }
        }
      }
    })
  }
}

// Setup sortable on mount removed duplicate onMounted
// onMounted(() => {
//   initSortable()
// })

// Re-init sortable when filter/search changes
watch([currentFilter, searchQuery], () => {
  nextTick(() => {
    initSortable()
  })
})

// Re-init sortable when configs change
watch(() => props.configs, () => {
  nextTick(() => {
    initSortable()
  })
}, { deep: true })
</script>

<style scoped>
/* Scoped styles removed as we use Tailwind classes now */
</style>
