<template>
  <SegmentedPills
    :model-value="configStore.currentFilter"
    layout-id="workspace-tool-pill"
    dense
    :items="WORKSPACE_TOOLS.map(item => ({ value: item.id, label: item.label }))"
    @update:model-value="onTool"
  />
</template>

<script setup lang="ts">
import type { Provider } from '@/types'
import { WORKSPACE_TOOLS } from '@/lib/workspace'
import { useConfigStore } from '@/stores/configStore'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'

const configStore = useConfigStore()

function onTool(value: string) {
  if (value === 'all' || value === 'claude' || value === 'codex' || value === 'gemini' || value === 'openclaw' || value === 'grok') {
    configStore.setFilter(value as Provider | 'all')
  }
}
</script>
