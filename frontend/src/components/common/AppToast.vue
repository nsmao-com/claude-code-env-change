<template>
  <Teleport to="body">
    <div
      class="pointer-events-none fixed inset-x-0 top-3 z-[10000] flex justify-center px-3"
      style="--wails-draggable: nodrag"
      aria-live="polite"
      aria-relevant="additions"
    >
      <TransitionGroup
        tag="div"
        name="app-msg"
        class="relative flex w-full flex-col items-center gap-2"
      >
        <button
          v-for="item in toasts"
          :key="item.id"
          type="button"
          class="app-msg-card pointer-events-auto inline-flex w-fit max-w-[min(28rem,calc(100vw-1.5rem))] shrink-0 items-center gap-2.5 rounded-2xl bg-white px-3.5 py-2.5 text-left"
          @click="dismiss(item.id)"
        >
          <span
            class="flex size-5 shrink-0 items-center justify-center rounded-full text-white"
            :class="iconWrap[item.type]"
          >
            <Check v-if="item.type === 'success'" class="size-3.5" :stroke-width="2.8" />
            <X v-else-if="item.type === 'error'" class="size-3.5" :stroke-width="2.8" />
            <Info v-else class="size-3.5" :stroke-width="2.8" />
          </span>
          <span class="min-w-0 break-words text-[13px] leading-snug font-medium text-neutral-800">{{ item.message }}</span>
        </button>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { Check, Info, X } from '@lucide/vue'
import { useToast } from '@/composables/useToast'
import type { ToastType } from '@/types'

const { toasts, dismiss } = useToast()

const iconWrap: Record<ToastType, string> = {
  success: 'bg-emerald-500',
  error: 'bg-red-500',
  info: 'bg-sky-500',
}
</script>

<style>
.app-msg-card {
  width: max-content;
  max-width: min(28rem, calc(100vw - 1.5rem));
  box-shadow:
    0 10px 28px rgba(15, 23, 42, 0.08),
    0 1px 3px rgba(15, 23, 42, 0.06);
  outline: 1px solid rgba(15, 23, 42, 0.06);
}

.app-msg-enter-active {
  transition:
    transform 0.38s cubic-bezier(0.22, 1, 0.36, 1),
    opacity 0.28s ease;
}
.app-msg-leave-active {
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  transition:
    transform 0.2s ease-in,
    opacity 0.16s ease-in;
}
.app-msg-enter-from {
  opacity: 0;
  transform: translateY(-18px);
}
.app-msg-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(-8px) scale(0.98);
}
.app-msg-move {
  transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1);
}

html[data-reduce-motion='true'] .app-msg-enter-active,
html[data-reduce-motion='true'] .app-msg-leave-active,
html[data-reduce-motion='true'] .app-msg-move {
  transition: none;
}

@media (prefers-reduced-motion: reduce) {
  .app-msg-enter-active,
  .app-msg-leave-active,
  .app-msg-move {
    transition: none;
  }
}
</style>
