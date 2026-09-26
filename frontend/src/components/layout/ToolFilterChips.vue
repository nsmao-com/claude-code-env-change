<template>
  <div class="inline-flex flex-wrap items-center gap-2">
    <button
      v-for="item in WORKSPACE_TOOLS"
      :key="item.id"
      type="button"
      :aria-pressed="configStore.currentFilter === item.id"
      class="inline-flex h-8.5 shrink-0 items-center gap-2 rounded-[10px] border border-border bg-card px-3 text-[13px] font-medium text-muted-foreground shadow-[0_1px_2px_rgba(16,24,40,0.04)] transition-all duration-150 hover:border-foreground/20 hover:text-foreground active:scale-[0.97]"
      :class="configStore.currentFilter === item.id ? 'text-foreground shadow-[0_1px_3px_rgba(16,24,40,0.1)] ring-1 ring-black/[0.06]' : ''"
      @click="onTool(item.id)"
    >
      <Check v-if="configStore.currentFilter === item.id" class="size-3.5 text-brand" />
      <span v-else-if="item.id === 'all'" class="size-1.5 rounded-full bg-brand" />
      <BrandIcon v-else :provider="item.id" class="size-3.5" :class="iconColor(item.id)" />
      {{ item.id === 'all' ? t('ui.all') : item.label }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { Check } from '@lucide/vue'
import { WORKSPACE_TOOLS, type WorkspaceTool } from '@/lib/workspace'
import { useConfigStore } from '@/stores/configStore'
import BrandIcon from '@/components/common/BrandIcon.vue'

const { t } = useI18n()

const configStore = useConfigStore()

const ICON_COLORS: Record<string, string> = {
  claude: 'text-[#D97757]',
  claude_desktop: 'text-[#D97757]',
  codex: 'text-[#1A1D21] dark:text-white/80',
  antigravity: 'text-[#4F6BED]',
  opencode: 'text-[#131010] dark:text-white/80',
  grok: 'text-[#6B7280]',
}

function iconColor(id: string) {
  return ICON_COLORS[id] || 'text-muted-foreground'
}

function onTool(value: WorkspaceTool) {
  configStore.setFilter(value)
}
</script>
