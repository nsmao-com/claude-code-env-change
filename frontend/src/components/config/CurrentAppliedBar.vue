<template>
  <div class="border-b border-border">
    <button
      type="button"
      class="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left hover:bg-muted/50"
      @click="expanded = !expanded"
    >
      <div class="flex min-w-0 items-center gap-2">
        <span class="text-xs font-medium text-muted-foreground">目前在用</span>
        <div v-if="!expanded" class="flex min-w-0 items-center gap-2">
          <template v-if="appliedConfigs.length">
            <span
              v-for="config in appliedConfigs"
              :key="config.name"
              class="inline-flex min-w-0 items-center gap-1 text-xs"
            >
              <BrandIcon :provider="config.provider" class="size-3" />
              <span class="max-w-[120px] truncate font-medium">{{ config.name }}</span>
            </span>
          </template>
          <span v-else class="text-xs text-muted-foreground">暂无</span>
        </div>
      </div>
      <ChevronDown :class="['size-3.5 shrink-0 text-muted-foreground transition-transform', expanded && 'rotate-180']" />
    </button>
    <div v-if="expanded" class="divide-y border-t">
      <ConfigListItem
        v-for="config in appliedConfigs"
        :key="config.name"
        :config="config"
        nested
        is-active
        @click="$emit('edit', config.name)"
        @apply="$emit('apply', config.name)"
        @duplicate="$emit('duplicate', config.name)"
        @edit="$emit('edit', config.name)"
        @delete="$emit('delete', config.name)"
      />
      <p v-if="appliedConfigs.length === 0" class="px-4 py-3 text-xs text-muted-foreground">
        当前筛选下还没有已应用的配置
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronDown } from '@lucide/vue'
import type { Provider } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import BrandIcon from '@/components/common/BrandIcon.vue'
import ConfigListItem from './ConfigListItem.vue'

defineEmits<{
  edit: [name: string]
  apply: [name: string]
  duplicate: [name: string]
  delete: [name: string]
}>()

const expanded = ref(false)
const configStore = useConfigStore()

const providers: Provider[] = ['claude', 'codex', 'gemini', 'opencode', 'grok']

const appliedConfigs = computed(() => {
  const filter = configStore.currentFilter
  const list = filter === 'all' ? providers : [filter]
  return list
    .map((provider) => {
      const name = configStore.activeEnvs[provider]
      return name ? configStore.getEnvByName(name) : undefined
    })
    .filter((config): config is NonNullable<typeof config> => !!config)
})
</script>
