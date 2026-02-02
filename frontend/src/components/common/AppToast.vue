<template>
  <Teleport to="body">
    <div class="fixed bottom-4 right-4 z-[99999] flex flex-col gap-2">
      <TransitionGroup name="toast">
        <div
          v-for="toast in toasts"
          :key="toast.id"
          class="toast p-4 rounded-lg shadow-lg flex items-center gap-3"
        >
          <i :class="['fas', iconClass(toast.type), colorClass(toast.type)]"></i>
          <span class="text-sm font-medium">{{ toast.message }}</span>
          <button
            class="ml-2 text-muted-foreground hover:text-foreground"
            @click="remove(toast.id)"
          >
            <i class="fas fa-times text-xs"></i>
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useToast } from '@/composables/useToast'
import type { ToastType } from '@/types'

const { toasts, remove } = useToast()

function iconClass(type: ToastType): string {
  const icons: Record<ToastType, string> = {
    success: 'fa-check-circle',
    error: 'fa-exclamation-circle',
    info: 'fa-info-circle'
  }
  return icons[type]
}

function colorClass(type: ToastType): string {
  const colors: Record<ToastType, string> = {
    success: 'text-green-500',
    error: 'text-red-500',
    info: 'text-primary'
  }
  return colors[type]
}
</script>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from {
  opacity: 0;
  transform: translateX(100%);
}

.toast-leave-to {
  opacity: 0;
  transform: translateX(100%);
}
</style>
