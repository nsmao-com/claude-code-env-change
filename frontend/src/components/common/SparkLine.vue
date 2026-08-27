<template>
  <svg :width="width" :height="height" :viewBox="`0 0 ${width} ${height}`" class="overflow-visible">
    <polyline
      :points="points"
      fill="none"
      :stroke="color"
      stroke-width="1.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  values: number[]
  width?: number
  height?: number
  color?: string
}>(), {
  width: 48,
  height: 18,
  color: '#4ADE80',
})

const points = computed(() => {
  const max = Math.max(...props.values, 1)
  const min = Math.min(...props.values, 0)
  const range = Math.max(max - min, 1)
  const n = props.values.length
  const stepX = n > 1 ? props.width / (n - 1) : props.width
  return props.values
    .map((v, i) => {
      const x = i * stepX
      const y = props.height - 2 - ((v - min) / range) * (props.height - 4)
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
})
</script>
