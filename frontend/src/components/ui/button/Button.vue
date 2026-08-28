<script setup lang="ts">
import type { PrimitiveProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import type { ButtonVariants } from '.'
import { Primitive } from 'reka-ui'
import { cn } from '@/lib/utils'
import { buttonVariants } from '.'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

interface Props extends PrimitiveProps {
  variant?: ButtonVariants['variant']
  size?: ButtonVariants['size']
  class?: HTMLAttributes['class']
  title?: string
}

const props = withDefaults(defineProps<Props>(), {
  as: 'button',
})

defineOptions({
  inheritAttrs: false,
})
</script>

<template>
  <Tooltip v-if="title">
    <TooltipTrigger as-child>
      <Primitive
        v-bind="$attrs"
        data-slot="button"
        :data-variant="variant"
        :data-size="size"
        :as="as"
        :as-child="asChild"
        :class="cn(buttonVariants({ variant, size }), props.class)"
      >
        <slot />
      </Primitive>
    </TooltipTrigger>
    <TooltipContent>
      {{ title }}
    </TooltipContent>
  </Tooltip>
  <Primitive
    v-else
    v-bind="$attrs"
    data-slot="button"
    :data-variant="variant"
    :data-size="size"
    :as="as"
    :as-child="asChild"
    :class="cn(buttonVariants({ variant, size }), props.class)"
  >
    <slot />
  </Primitive>
</template>
