<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <div
    data-slot="card-title"
    :class="cn('min-w-0 max-w-full truncate text-base leading-snug font-medium group-data-[size=sm]/card:text-sm cn-font-heading', props.class)"
  >
    <slot />
  </div>
</template>
