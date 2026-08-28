<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 36 36"
    role="img"
    aria-label="AI ENV"
    :class="className"
  >
    <defs>
      <linearGradient id="n-face" x1="18" y1="4" x2="18" y2="32" gradientUnits="userSpaceOnUse">
        <stop offset="0" stop-color="#2A2A2A" />
        <stop offset="1" stop-color="#0A0A0A" />
      </linearGradient>
    </defs>
    <rect width="36" height="36" rx="10" fill="url(#n-face)" />
    <rect x="0.6" y="0.6" width="34.8" height="34.8" rx="9.4" fill="none" stroke="rgba(255,255,255,0.12)" stroke-width="1.2" />
    <path
      fill="#FAFAFA"
      d="M9.1 7.4h4.05L22.7 22.2V7.4h4.2v21.2h-4.05L13.3 13.6v15H9.1V7.4z"
    />
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
