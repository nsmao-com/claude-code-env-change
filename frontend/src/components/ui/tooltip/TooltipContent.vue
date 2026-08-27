<script setup lang="ts">
import type { TooltipContentEmits, TooltipContentProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { TooltipArrow, TooltipContent, TooltipPortal, useForwardPropsEmits } from 'reka-ui'
import { cn } from '@/lib/utils'

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(defineProps<TooltipContentProps & { class?: HTMLAttributes['class'] }>(), {
  sideOffset: 6,
  side: 'top',
  positionStrategy: 'fixed',
  collisionPadding: 8,
})

const emits = defineEmits<TooltipContentEmits>()

const delegatedProps = reactiveOmit(props, 'class')
const forwarded = useForwardPropsEmits(delegatedProps, emits)
</script>

<template>
  <TooltipPortal to="body">
    <TooltipContent
      data-slot="tooltip-content"
      v-bind="{ ...forwarded, ...$attrs }"
      :class="cn('data-open:animate-in data-open:fade-in-0 data-[state=delayed-open]:animate-in data-[state=delayed-open]:fade-in-0 data-closed:animate-out data-closed:fade-out-0 data-open:zoom-in-95 data-closed:zoom-out-95 pointer-events-none z-[9999] flex w-max min-w-max max-w-xs items-center gap-1.5 overflow-visible rounded-xl bg-foreground px-3 py-1.5 text-xs text-background whitespace-nowrap shadow-sm duration-150 ease-out motion-reduce:animate-none', props.class)"
    >
      <slot />

      <TooltipArrow class="z-[9999] size-2.5 translate-y-[calc(-50%_-_2px)] rotate-45 rounded-xs bg-foreground fill-foreground" />
    </TooltipContent>
  </TooltipPortal>
</template>
