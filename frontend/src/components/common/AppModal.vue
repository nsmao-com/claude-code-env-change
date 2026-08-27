<template>
  <div v-if="plain" class="flex h-full min-h-0 flex-col overflow-hidden bg-background">
    <div v-if="title || $slots.header" class="flex shrink-0 items-end justify-between gap-4 px-6 pt-4 pb-4">
      <div class="min-w-0">
        <slot name="header">
          <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">{{ title }}</h1>
        </slot>
      </div>
      <div class="flex shrink-0 flex-wrap items-center justify-end gap-2 pb-0.5">
        <slot name="actions">
          <ToolFilterChips v-if="toolFilter" />
        </slot>
      </div>
    </div>
    <ScrollArea class="min-h-0 flex-1">
      <div class="px-6 pb-8 pt-1">
        <slot />
      </div>
    </ScrollArea>
    <div v-if="$slots.footer" class="shrink-0 border-t px-6 py-3">
      <slot name="footer" />
    </div>
  </div>
  <Dialog v-else :open="modelValue" @update:open="onOpen">
    <DialogContent
      :class="sizeClass"
      :show-close-button="showClose"
      @pointer-down-outside="onPointerDownOutside"
      @interact-outside="onPointerDownOutside"
    >
      <DialogHeader v-if="title || $slots.header">
        <slot name="header">
          <DialogTitle>{{ title }}</DialogTitle>
        </slot>
      </DialogHeader>
      <div class="max-h-[70vh] overflow-y-auto overflow-x-hidden p-0.5">
        <slot />
      </div>
      <DialogFooter v-if="$slots.footer">
        <slot name="footer" />
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import ToolFilterChips from '@/components/layout/ToolFilterChips.vue'

interface Props {
  modelValue: boolean
  title?: string
  size?: 'sm' | 'md' | 'lg' | 'xl' | 'full'
  showClose?: boolean
  closeOnOverlay?: boolean
  plain?: boolean
  toolFilter?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md',
  showClose: true,
  closeOnOverlay: true,
  plain: false,
  toolFilter: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const sizeClass = computed(() => {
  const sizes: Record<string, string> = {
    sm: 'sm:max-w-sm',
    md: 'sm:max-w-lg',
    lg: 'sm:max-w-2xl',
    xl: 'sm:max-w-4xl',
    full: 'sm:max-w-[90vw]',
  }
  return sizes[props.size]
})

function onOpen(open: boolean) {
  emit('update:modelValue', open)
}

// closeOnOverlay=false 时只拦截遮罩点击，不影响 X / Esc 关闭
function onPointerDownOutside(event: Event) {
  if (!props.closeOnOverlay) event.preventDefault()
}
</script>
