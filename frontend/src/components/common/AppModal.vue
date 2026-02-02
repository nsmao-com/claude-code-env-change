<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-50 flex items-center justify-center p-4"
      >
        <!-- Overlay -->
        <div
          class="modal-overlay absolute inset-0"
          @click="closeOnOverlay && close()"
        ></div>

        <!-- Modal Content -->
        <div
          class="modal-content relative w-full rounded-xl overflow-hidden"
          :class="sizeClass"
        >
          <!-- Header -->
          <div v-if="title || $slots.header" class="flex items-center justify-between p-4 border-b border-border">
            <slot name="header">
              <h3 class="text-lg font-semibold">{{ title }}</h3>
            </slot>
            <button
              v-if="showClose"
              class="w-8 h-8 rounded-lg hover:bg-muted flex items-center justify-center text-muted-foreground"
              @click="close"
            >
              <i class="fas fa-times"></i>
            </button>
          </div>

          <!-- Body -->
          <div class="p-4 max-h-[70vh] overflow-y-auto">
            <slot />
          </div>

          <!-- Footer -->
          <div v-if="$slots.footer" class="p-4 border-t border-border">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  modelValue: boolean
  title?: string
  size?: 'sm' | 'md' | 'lg' | 'xl' | 'full'
  showClose?: boolean
  closeOnOverlay?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md',
  showClose: true,
  closeOnOverlay: true
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const sizeClass = computed(() => {
  const sizes: Record<string, string> = {
    sm: 'max-w-sm',
    md: 'max-w-lg',
    lg: 'max-w-2xl',
    xl: 'max-w-4xl',
    full: 'max-w-[90vw]'
  }
  return sizes[props.size]
})

function close() {
  emit('update:modelValue', false)
}
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.3s ease;
}

.modal-enter-active .modal-content,
.modal-leave-active .modal-content {
  transition: transform 0.3s ease, opacity 0.3s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-from .modal-content,
.modal-leave-to .modal-content {
  transform: scale(0.95);
  opacity: 0;
}
</style>
