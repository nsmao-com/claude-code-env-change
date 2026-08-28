<template>
  <component
    v-if="lucide"
    :is="lucide"
    :class="iconClass"
  />
  <span v-else>{{ display }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { resolveLucideIcon } from '@/lib/configIcons'

const props = withDefaults(defineProps<{
  value?: string
  class?: string
}>(), {
  value: '',
})

const lucide = computed(() => resolveLucideIcon(props.value))
const display = computed(() => props.value?.trim() || '⌘')
const iconClass = computed(() => props.class || 'size-4')
</script>
