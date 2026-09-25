<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 36 36"
    role="img"
    aria-label="AI ENV"
    :class="className"
  >
    <defs>
      <linearGradient id="ai-logo-face" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#FF9A5A" /><stop offset="0.5" stop-color="#EE6A22" /><stop offset="1" stop-color="#C9480C" /></linearGradient>
      <radialGradient id="ai-logo-gloss" cx="0.28" cy="0.12" r="0.75"><stop offset="0" stop-color="#fff" stop-opacity="0.34" /><stop offset="1" stop-color="#fff" stop-opacity="0" /></radialGradient>
      <linearGradient id="ai-logo-rim" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fff" stop-opacity="0.55" /><stop offset="0.45" stop-color="#fff" stop-opacity="0.08" /><stop offset="1" stop-color="#fff" stop-opacity="0" /></linearGradient>
      <filter id="ai-logo-lift" x="-30%" y="-30%" width="160%" height="160%"><feDropShadow dx="0" dy="0.5" stdDeviation="0.7" flood-color="#7A2A00" flood-opacity="0.38" /></filter>
    </defs>
    <!-- 与 build/appicon.svg 同一设计：橙色圆角底 + 开关，滑块里是 AI 星芒 -->
    <rect width="36" height="36" rx="10" fill="url(#ai-logo-face)" />
    <rect width="36" height="36" rx="10" fill="url(#ai-logo-gloss)" />
    <rect x="0.3" y="0.3" width="35.4" height="35.4" rx="9.7" fill="none" stroke="url(#ai-logo-rim)" stroke-width="0.6" />
    <rect x="5.5" y="11.25" width="25" height="13.5" rx="6.75" fill="#fff" fill-opacity="0.24" />
    <rect x="5.75" y="11.5" width="24.5" height="13" rx="6.5" fill="none" stroke="#fff" stroke-opacity="0.35" stroke-width="0.5" />
    <circle cx="12.25" cy="18" r="1.1" fill="#fff" fill-opacity="0.7" />
    <circle cx="23.75" cy="18" r="5.85" fill="#fff" filter="url(#ai-logo-lift)" />
    <path fill="#E9621A" d="M23.75 14.37C24.11 16 25.75 17.64 27.38 18C25.75 18.36 24.11 20 23.75 21.63C23.39 20 21.75 18.36 20.12 18C21.75 17.64 23.39 16 23.75 14.37Z" />
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
