<template>
  <Dialog :open="show" @update:open="onOpen">
    <DialogContent class="gap-2 p-3 sm:max-w-[360px]">
      <DialogHeader>
        <DialogTitle>选择图标</DialogTitle>
      </DialogHeader>
      <Picker
        :native="true"
        :disable-skin-tones="true"
        :disable-sticky-group-names="true"
        theme="auto"
        @select="onSelectEmoji"
      />
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import 'vue3-emoji-picker/css'

const Picker = defineAsyncComponent(() => import('vue3-emoji-picker'))

defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  close: []
  select: [emoji: string]
}>()

function onOpen(open: boolean) {
  if (!open) emit('close')
}

function onSelectEmoji(emoji: { i: string; n: string[]; r: string; t: string; u: string }) {
  emit('select', emoji.i)
  emit('close')
}
</script>
