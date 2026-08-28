<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <div
    data-slot="card-description"
    :class="cn('min-w-0 max-w-full break-words text-muted-foreground text-sm', props.class)"
  >
    <slot />
  </div>
</template>
