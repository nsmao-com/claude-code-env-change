<template>
  <div v-if="plain" class="flex h-full min-h-0 flex-col overflow-hidden bg-background">
    <div v-if="title || $slots.header" class="flex shrink-0 flex-wrap items-end justify-between gap-4 px-6 pt-4 pb-4" :class="plainWidthClass">
      <div class="min-w-0">
        <slot name="header">
          <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">{{ title }}</h1>
        </slot>
      </div>
      <div class="flex min-w-0 max-w-full flex-wrap items-center justify-end gap-2 pb-0.5">
        <slot name="actions">
          <ToolFilterChips v-if="toolFilter" />
        </slot>
      </div>
    </div>
    <!-- [&>*]:shrink-0：内容超高时靠滚动，而不是把卡片等直接子元素压扁 -->
    <div class="flex min-h-0 flex-1 flex-col overflow-y-auto px-6 pb-6 pt-2 [&>*]:shrink-0" :class="plainWidthClass">
      <slot />
    </div>
    <div v-if="$slots.footer" class="shrink-0 border-t px-6 py-3" :class="plainWidthClass">
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
      <div class="max-h-[70vh] min-w-0 overflow-x-hidden overflow-y-auto">
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
import ToolFilterChips from '@/components/layout/ToolFilterChips.vue'

interface Props {
  modelValue: boolean
  title?: string
  size?: 'sm' | 'md' | 'lg' | 'xl' | 'full'
  showClose?: boolean
  closeOnOverlay?: boolean
  plain?: boolean
  toolFilter?: boolean
  /** plain 模式下的内容宽度：form = 设置/表单页，wide = 列表页；不传则占满窗口 */
  width?: 'form' | 'wide'
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md',
  showClose: true,
  closeOnOverlay: true,
  plain: false,
  toolFilter: false,
  width: undefined,
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

// 表单/设置页在大窗口下整行拉伸很难看，居中限宽；标题、内容、页脚同宽对齐
const plainWidthClass = computed(() => {
  if (props.width === 'form') return 'mx-auto w-full max-w-4xl'
  if (props.width === 'wide') return 'mx-auto w-full max-w-6xl'
  return ''
})

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
