<template>
  <span v-if="disabled || !content" :class="cn('inline-flex min-w-0 max-w-full', wrapperClass)">
    <slot />
  </span>
  <Tooltip v-else>
    <TooltipTrigger as-child>
      <span :class="cn('inline-flex min-w-0 max-w-full', wrapperClass)">
        <slot />
      </span>
    </TooltipTrigger>
    <TooltipContent :side="side" :class="wrap ? 'max-w-xs whitespace-normal break-words text-left leading-relaxed' : undefined">
      {{ content }}
    </TooltipContent>
  </Tooltip>
</template>

<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { computed } from 'vue'
import { cn } from '@/lib/utils'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

const props = withDefaults(defineProps<{
  content?: string
  disabled?: boolean
  side?: 'top' | 'right' | 'bottom' | 'left'
  wrap?: boolean
  class?: HTMLAttributes['class']
}>(), {
  content: '',
  disabled: false,
  side: 'top',
  wrap: false,
})

const wrapperClass = computed(() => props.class)
</script>
