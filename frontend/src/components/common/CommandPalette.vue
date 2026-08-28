<template>
  <Dialog :open="open" @update:open="onOpen">
    <DialogContent
      class="top-[16%] w-full max-w-[560px] translate-y-0 gap-0 overflow-hidden p-0 sm:max-w-[560px]"
      :show-close-button="false"
    >
      <DialogTitle class="sr-only">{{ t('palette.title') }}</DialogTitle>
      <div class="flex items-center gap-2 border-b px-3">
        <Search class="size-4 shrink-0 text-muted-foreground" />
        <input
          ref="inputRef"
          v-model="query"
          class="h-11 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          :placeholder="t('palette.placeholder')"
          @keydown="onKeydown"
        >
        <span class="shrink-0 rounded-md bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">ESC</span>
      </div>
      <ScrollArea class="max-h-[420px]">
        <div v-if="groups.length === 0" class="px-4 py-8 text-center text-sm text-muted-foreground">
          {{ t('palette.empty') }}
        </div>
        <div v-for="group in groups" :key="group.label" class="px-2 py-2">
          <div class="px-2 pb-1 text-[10px] font-medium tracking-wide text-muted-foreground uppercase">
            {{ group.label }}
          </div>
          <div class="flex flex-col gap-0.5">
            <button
              v-for="item in group.items"
              :key="item.id"
              type="button"
              :class="[
                'flex w-full items-center gap-2 rounded-[4px] px-2 py-1.5 text-left text-sm',
                item.id === activeId ? 'bg-muted' : 'hover:bg-muted/60',
              ]"
              @mouseenter="activeId = item.id"
              @click="run(item)"
            >
              <component :is="item.icon" v-if="item.icon" class="size-4 shrink-0 text-muted-foreground" />
              <BrandIcon v-else-if="item.brand" :provider="item.brand" class="size-4 shrink-0" />
              <span class="min-w-0 flex-1 truncate">{{ item.label }}</span>
              <span v-if="item.hint" class="shrink-0 text-xs text-muted-foreground">{{ item.hint }}</span>
            </button>
          </div>
        </div>
      </ScrollArea>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { FileJson, Plus, Search, Upload } from '@lucide/vue'
import type { AppPage, Provider } from '@/types'
import { APP_PAGES } from '@/lib/nav'
import { useConfigStore } from '@/stores/configStore'
import { useI18n } from '@/composables/useI18n'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'

interface PaletteItem {
  id: string
  group: string
  label: string
  hint?: string
  keywords: string
  icon?: unknown
  brand?: Provider
  run: () => void
}

const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  navigate: [page: AppPage]
  add: []
  edit: [name: string]
  apply: [name: string]
  importLocal: []
  importJson: []
}>()

const { t } = useI18n()
const configStore = useConfigStore()
const query = ref('')
const activeId = ref('')
const inputRef = ref<HTMLInputElement | null>(null)

const open = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
})

const allItems = computed<PaletteItem[]>(() => {
  const pages: PaletteItem[] = APP_PAGES.map(page => ({
    id: `page-${page.id}`,
    group: t('palette.pages'),
    label: t(`nav.${page.id}`),
    hint: t('palette.open'),
    keywords: `${page.id} ${page.label} ${page.title}`,
    icon: page.icon,
    run: () => emit('navigate', page.id),
  }))

  const configs: PaletteItem[] = configStore.environments.flatMap((env) => {
    const keywords = `${env.name} ${env.description || ''} ${env.provider}`
    return [
      {
        id: `apply-${env.name}`,
        group: t('palette.configs'),
        label: env.name,
        hint: t('palette.apply'),
        keywords,
        brand: env.provider,
        run: () => emit('apply', env.name),
      },
      {
        id: `edit-${env.name}`,
        group: t('palette.configs'),
        label: env.name,
        hint: t('palette.edit'),
        keywords,
        brand: env.provider,
        run: () => emit('edit', env.name),
      },
    ]
  })

  const actions: PaletteItem[] = [
    {
      id: 'action-add',
      group: t('palette.actions'),
      label: t('palette.newConfig'),
      keywords: 'new add 新建 配置',
      icon: Plus,
      run: () => emit('add'),
    },
    {
      id: 'action-import-local',
      group: t('palette.actions'),
      label: t('palette.importLocal'),
      keywords: 'import local 本机 导入',
      icon: Upload,
      run: () => emit('importLocal'),
    },
    {
      id: 'action-import-json',
      group: t('palette.actions'),
      label: t('palette.importJson'),
      keywords: 'import json 导入 配置 拖拽 drop',
      icon: FileJson,
      run: () => emit('importJson'),
    },
  ]

  return [...pages, ...configs, ...actions]
})

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return allItems.value
  return allItems.value.filter(item =>
    item.label.toLowerCase().includes(q)
    || item.hint?.toLowerCase().includes(q)
    || item.keywords.toLowerCase().includes(q),
  )
})

const groups = computed(() => {
  const map = new Map<string, PaletteItem[]>()
  for (const item of filtered.value) {
    const list = map.get(item.group) || []
    list.push(item)
    map.set(item.group, list)
  }
  return [...map.entries()].map(([label, items]) => ({ label, items }))
})

const flat = computed(() => filtered.value)

watch(open, async (value) => {
  if (!value) return
  query.value = ''
  activeId.value = flat.value[0]?.id || ''
  await nextTick()
  inputRef.value?.focus()
})

watch(filtered, (items) => {
  if (!items.some(item => item.id === activeId.value)) {
    activeId.value = items[0]?.id || ''
  }
})

function onOpen(value: boolean) {
  open.value = value
}

function run(item: PaletteItem) {
  open.value = false
  item.run()
}

function onKeydown(event: KeyboardEvent) {
  const items = flat.value
  const index = items.findIndex(item => item.id === activeId.value)
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    const next = items[Math.min(items.length - 1, Math.max(0, index) + 1)]
    if (next) activeId.value = next.id
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    const prev = items[Math.max(0, index - 1)]
    if (prev) activeId.value = prev.id
  } else if (event.key === 'Enter') {
    event.preventDefault()
    const current = items.find(item => item.id === activeId.value) || items[0]
    if (current) run(current)
  }
}
</script>
