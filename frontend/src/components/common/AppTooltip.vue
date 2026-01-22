<template>
  <span
    class="relative inline-flex"
    @mouseenter="open = true"
    @mouseleave="open = false"
    @focusin="open = true"
    @focusout="open = false"
  >
    <slot />

    <span
      v-if="!disabled && content"
      class="absolute bottom-full left-1/2 -translate-x-1/2 mb-3 z-[9999] pointer-events-none"
    >
      <transition name="app-tooltip">
        <span
          v-if="open"
          class="relative block"
        >
          <span class="block max-w-[360px] rounded-xl bg-popover/95 backdrop-blur-md text-popover-foreground border border-border/80 shadow-2xl px-4 py-3.5 text-[12px] leading-6">
            <div class="flex items-center gap-2">
              <span :class="['w-1.5 h-1.5 rounded-full', toneDotClass]"></span>
              <span :class="['font-bold tracking-wide', toneTextClass]">{{ header }}</span>
            </div>

            <div v-if="bodyLines.length" class="mt-2.5 space-y-2">
              <div v-for="(row, idx) in bodyRows" :key="`${idx}-${row.raw}`" class="text-[12px]">
                <div v-if="row.label" class="flex items-center justify-between gap-6 min-w-0">
                  <span class="text-muted-foreground shrink-0">{{ row.label }}</span>
                  <span class="font-mono text-foreground/90 min-w-0 text-right break-all">{{ row.value }}</span>
                </div>
                <div v-else class="text-muted-foreground break-words">
                  {{ row.value }}
                </div>
              </div>
            </div>
          </span>
          <span class="absolute left-1/2 -translate-x-1/2 -bottom-1 w-3 h-3 rotate-45 bg-popover/95 border border-border/80"></span>
        </span>
      </transition>
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed, ref, toRefs } from 'vue'

interface Props {
  content?: string
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  content: '',
  disabled: false
})

const open = ref(false)
const { content, disabled } = toRefs(props)

const lines = computed(() => {
  const raw = String(content.value || '')
  return raw
    .split(/\r?\n/g)
    .map((s) => s.trim())
    .filter(Boolean)
})

const header = computed(() => lines.value[0] || '')
const bodyLines = computed(() => lines.value.slice(1))

function splitRow(raw: string): { raw: string; label: string; value: string } {
  const idx = raw.indexOf(' ')
  if (idx > 0) {
    return { raw, label: raw.slice(0, idx).trim(), value: raw.slice(idx + 1).trim() }
  }
  return { raw, label: '', value: raw }
}

const bodyRows = computed(() => bodyLines.value.map(splitRow))

const tone = computed<'success' | 'danger' | 'neutral'>(() => {
  const h = header.value
  if (h.includes('成功') || h.toUpperCase().includes('OK')) return 'success'
  if (h.includes('失败') || h.toUpperCase().includes('FAIL')) return 'danger'
  return 'neutral'
})

const toneDotClass = computed(() => {
  if (tone.value === 'success') return 'bg-green-500'
  if (tone.value === 'danger') return 'bg-red-500'
  return 'bg-muted-foreground'
})

const toneTextClass = computed(() => {
  if (tone.value === 'success') return 'text-green-600'
  if (tone.value === 'danger') return 'text-red-600'
  return 'text-foreground'
})
</script>

<style scoped>
.app-tooltip-enter-active,
.app-tooltip-leave-active {
  transition: opacity 0.12s ease, transform 0.12s ease;
}

.app-tooltip-enter-from,
.app-tooltip-leave-to {
  opacity: 0;
  transform: translateY(4px);
}
</style>
