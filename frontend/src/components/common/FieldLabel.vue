<template>
  <Label>
    <span>{{ label }}</span>
    <AppTooltip v-if="hint" :content="hint" wrap>
      <span class="inline-flex cursor-help text-muted-foreground">
        <CircleHelp class="size-3.5" />
      </span>
    </AppTooltip>
  </Label>
</template>

<script setup lang="ts">
import { CircleHelp } from '@lucide/vue'
import { Label } from '@/components/ui/label'
import AppTooltip from '@/components/common/AppTooltip.vue'

defineProps<{
  label: string
  hint?: string
}>()
</script>
