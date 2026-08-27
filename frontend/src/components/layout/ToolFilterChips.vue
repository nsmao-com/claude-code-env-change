<template>
  <Select :model-value="configStore.currentFilter" @update:model-value="onTool">
    <SelectTrigger class="h-9 gap-2 rounded-full border-0 bg-card px-3 shadow-none ring-1 ring-black/[0.06] dark:ring-white/10">
      <Calendar class="size-3.5 text-muted-foreground" />
      <SelectValue placeholder="平台" />
    </SelectTrigger>
    <SelectContent align="end">
      <SelectItem v-for="item in WORKSPACE_TOOLS" :key="item.id" :value="item.id">
        {{ item.label }}{{ item.id === 'all' ? '平台' : '' }}
      </SelectItem>
    </SelectContent>
  </Select>
</template>

<script setup lang="ts">
import { Calendar } from '@lucide/vue'
import type { Provider } from '@/types'
import { WORKSPACE_TOOLS } from '@/lib/workspace'
import { useConfigStore } from '@/stores/configStore'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const configStore = useConfigStore()

function onTool(value: unknown) {
  if (value === 'all' || value === 'claude' || value === 'codex' || value === 'gemini' || value === 'openclaw') {
    configStore.setFilter(value as Provider | 'all')
  }
}
</script>
