<template>
  <div
    :class="[
      'relative inline-flex items-center rounded-full bg-muted p-1',
      full ? 'w-full' : '',
      className || '',
    ]"
  >
    <button
      v-for="item in items"
      :key="item.value"
      type="button"
      :class="[
        'relative isolate inline-flex h-8 shrink-0 items-center justify-center gap-1.5 rounded-full px-3 text-sm font-medium transition-colors',
        full ? 'flex-1' : '',
        dense ? 'h-7 px-2.5 text-xs' : '',
        modelValue === item.value ? 'text-foreground' : 'text-muted-foreground hover:text-foreground',
      ]"
      @click="$emit('update:modelValue', item.value)"
    >
      <motion.span
        v-if="modelValue === item.value"
        :layout-id="layoutId"
        class="absolute inset-0 rounded-full bg-background shadow-sm"
        :transition="{ type: 'spring', stiffness: 520, damping: 40 }"
      />
      <span class="relative z-10 inline-flex items-center gap-1.5">
        <slot :item="item">{{ item.label }}</slot>
      </span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { motion } from 'motion-v'

export interface SegmentedItem {
  value: string
  label: string
}

const props = withDefaults(defineProps<{
  modelValue: string
  items: SegmentedItem[]
  layoutId: string
  full?: boolean
  dense?: boolean
  class?: string
}>(), {
  full: false,
  dense: false,
})

defineEmits<{
  'update:modelValue': [value: string]
}>()

const className = props.class
</script>
