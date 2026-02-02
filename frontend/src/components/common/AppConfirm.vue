<template>
  <Teleport to="body">
    <Transition name="confirm">
      <div
        v-if="isOpen"
        class="fixed inset-0 z-50 flex items-center justify-center p-4"
      >
        <div class="modal-overlay absolute inset-0" @click="cancel"></div>
        <div class="modal-content relative w-full max-w-sm rounded-xl p-6">
          <div class="flex items-start gap-4">
            <div
              class="w-10 h-10 rounded-full flex items-center justify-center flex-shrink-0"
              :class="iconBgClass"
            >
              <i :class="['fas', iconClass, iconColorClass]"></i>
            </div>
            <div class="flex-1 min-w-0">
              <h3 class="font-semibold text-lg mb-1">{{ title }}</h3>
              <p class="text-sm text-muted-foreground">{{ message }}</p>
            </div>
          </div>
          <div class="flex justify-end gap-2 mt-6">
            <button class="btn btn-secondary" @click="cancel">取消</button>
            <button :class="['btn', confirmBtnClass]" @click="confirm">确定</button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useConfirm } from '@/composables/useConfirm'

const { isOpen, title, message, confirmType, confirm, cancel } = useConfirm()

const iconClass = computed(() => {
  const icons: Record<string, string> = {
    danger: 'fa-exclamation-triangle',
    warning: 'fa-exclamation-circle',
    info: 'fa-info-circle'
  }
  return icons[confirmType.value]
})

const iconBgClass = computed(() => {
  const classes: Record<string, string> = {
    danger: 'bg-red-100 dark:bg-red-900/30',
    warning: 'bg-yellow-100 dark:bg-yellow-900/30',
    info: 'bg-blue-100 dark:bg-blue-900/30'
  }
  return classes[confirmType.value]
})

const iconColorClass = computed(() => {
  const colors: Record<string, string> = {
    danger: 'text-red-500',
    warning: 'text-yellow-500',
    info: 'text-blue-500'
  }
  return colors[confirmType.value]
})

const confirmBtnClass = computed(() => {
  const classes: Record<string, string> = {
    danger: 'bg-red-500 text-white hover:bg-red-600',
    warning: 'bg-yellow-500 text-white hover:bg-yellow-600',
    info: 'btn-primary'
  }
  return classes[confirmType.value]
})
</script>

<style scoped>
.confirm-enter-active,
.confirm-leave-active {
  transition: opacity 0.2s ease;
}

.confirm-enter-from,
.confirm-leave-to {
  opacity: 0;
}
</style>
