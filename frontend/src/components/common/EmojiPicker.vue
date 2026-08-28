<template>
  <Dialog :open="show" @update:open="onOpen">
    <DialogContent class="gap-3 p-3 sm:max-w-[320px]" :show-close-button="false">
      <DialogHeader>
        <DialogTitle>选择图标</DialogTitle>
      </DialogHeader>
      <div class="flex flex-wrap gap-1">
        <Button
          v-for="group in EMOJI_GROUPS"
          :key="group.id"
          type="button"
          size="sm"
          :variant="activeGroup === group.id ? 'default' : 'ghost'"
          class="h-7 px-2 text-xs"
          @click="activeGroup = group.id"
        >
          {{ group.label }}
        </Button>
      </div>
      <div class="grid grid-cols-5 gap-1">
        <Button
          v-for="emoji in currentItems"
          :key="emoji"
          type="button"
          variant="ghost"
          size="icon"
          class="h-9 w-full text-lg"
          @click="select(emoji)"
        >
          {{ emoji }}
        </Button>
      </div>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { EMOJI_GROUPS } from '@/lib/emojis'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  close: []
  select: [emoji: string]
}>()

const activeGroup = ref(EMOJI_GROUPS[0].id)
const currentItems = computed(() =>
  EMOJI_GROUPS.find(group => group.id === activeGroup.value)?.items || EMOJI_GROUPS[0].items,
)

function onOpen(open: boolean) {
  if (!open) emit('close')
}

function select(emoji: string) {
  emit('select', emoji)
  emit('close')
}
</script>
