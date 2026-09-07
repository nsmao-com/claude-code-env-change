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
      d="M8.5 8h4.2l10.8 14.5V8h4v20h-4.1L12.5 13.8V28h-4V8z"
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
