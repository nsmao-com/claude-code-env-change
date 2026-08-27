<template>
  <div class="flex items-end" :style="{ gap: `${gap}px` }">
    <div
      v-for="(v, i) in normalized"
      :key="i"
      class="flex flex-col-reverse"
      :style="{ gap: `${gap}px` }"
    >
      <span
        v-for="d in rows"
        :key="d"
        class="rounded-full transition-opacity duration-500"
        :class="d <= v ? 'opacity-100' : 'opacity-15'"
        :style="{ width: `${size}px`, height: `${size}px`, backgroundColor: d <= v ? color : fallbackColor }"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  values: number[]
  rows?: number
  size?: number
  gap?: number
  color?: string
  fallbackColor?: string
}>(), {
  rows: 6,
  size: 5,
  gap: 4,
  color: '#F26B1D',
  fallbackColor: 'currentColor',
})

const normalized = computed(() => {
  const max = Math.max(...props.values, 1)
  return props.values.map(v => Math.round((v / max) * props.rows))
})
</script>
