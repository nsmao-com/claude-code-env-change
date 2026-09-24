<script setup lang="ts">
import type { SwitchRootProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { computed } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import {
  SwitchRoot,
  SwitchThumb,
  useForwardProps,
} from 'reka-ui'
import { cn } from '@/lib/utils'

// reka-ui 2 的 SwitchRoot 只认 modelValue / update:modelValue；项目里沿用的是
// radix-vue 时代的 :checked / @update:checked，直接透传时开关永远显示关闭、
// 点击也不会通知父组件。这里两套写法都接住，并同时发出两种事件。
const props = withDefaults(defineProps<SwitchRootProps & {
  class?: HTMLAttributes['class']
  size?: 'sm' | 'default'
  checked?: boolean
}>(), {
  size: 'default',
  checked: undefined,
  modelValue: undefined,
})

const emits = defineEmits<{
  'update:modelValue': [value: boolean]
  'update:checked': [value: boolean]
}>()

const delegatedProps = reactiveOmit(props, 'class', 'size', 'checked', 'modelValue')

const forwarded = useForwardProps(delegatedProps)

const value = computed(() => props.checked ?? props.modelValue ?? undefined)

function onUpdate(next: boolean) {
  emits('update:modelValue', next)
  emits('update:checked', next)
}
</script>

<template>
  <SwitchRoot
    v-slot="slotProps"
    data-slot="switch"
    :data-size="size"
    v-bind="forwarded"
    :model-value="value"
    :class="cn(
      'data-checked:bg-primary data-unchecked:bg-input dark:data-unchecked:bg-input/80 shrink-0 rounded-full border border-transparent aria-invalid:ring-destructive/20 aria-invalid:border-destructive data-[size=default]:h-[18.4px] data-[size=default]:w-8 data-[size=sm]:h-3.5 data-[size=sm]:w-6 peer group/switch relative inline-flex items-center transition-all outline-none after:absolute after:-inset-x-3 after:-inset-y-2 data-disabled:cursor-not-allowed data-disabled:opacity-50',
      props.class,
    )"
    @update:model-value="onUpdate"
  >
    <SwitchThumb
      data-slot="switch-thumb"
      class="bg-background dark:data-unchecked:bg-foreground dark:data-checked:bg-primary-foreground rounded-full group-data-[size=default]/switch:size-4 group-data-[size=sm]/switch:size-3 group-data-[size=default]/switch:data-checked:translate-x-[calc(100%-2px)] group-data-[size=sm]/switch:data-checked:translate-x-[calc(100%-2px)] group-data-[size=default]/switch:data-unchecked:translate-x-0 group-data-[size=sm]/switch:data-unchecked:translate-x-0 pointer-events-none block ring-0 transition-transform"
    >
      <slot name="thumb" v-bind="slotProps" />
    </SwitchThumb>
  </SwitchRoot>
</template>
