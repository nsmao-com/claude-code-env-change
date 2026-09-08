<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 36 36"
    role="img"
    aria-label="AI ENV"
    :class="className"
  >
    <defs><linearGradient id="n-face" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#303033" /><stop offset="0.5" stop-color="#121214" /><stop offset="1" stop-color="#050506" /></linearGradient></defs>
    <rect width="36" height="36" rx="10" fill="url(#n-face)" />
    <rect x="0.6" y="0.6" width="34.8" height="34.8" rx="9.4" fill="none" stroke="rgba(255,255,255,0.18)" stroke-width="1.2" />
    <circle cx="18" cy="18" r="8.3" fill="none" stroke="rgba(255,255,255,0.24)" stroke-width="0.8" />
    <path fill="#fff" d="M18 8.8c.9 5.4 3.8 8.3 9.2 9.2-5.4.9-8.3 3.8-9.2 9.2-.9-5.4-3.8-8.3-9.2-9.2 5.4-.9 8.3-3.8 9.2-9.2Z" />
    <circle cx="18" cy="18" r="1.8" fill="#111113" />
  </svg>
</template>

<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()

const className = computed(() => cn('size-9 shrink-0', props.class))
</script>
