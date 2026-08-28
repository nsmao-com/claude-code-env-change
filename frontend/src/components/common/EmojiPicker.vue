<template>
  <Dialog :open="show" @update:open="onOpen">
    <DialogContent class="gap-3 p-3 sm:max-w-[360px]" :show-close-button="false">
      <DialogHeader>
        <DialogTitle>选择图标</DialogTitle>
      </DialogHeader>
      <SegmentedPills
        :model-value="kind"
        layout-id="config-icon-kind"
        full
        dense
        :items="[{ value: 'emoji', label: 'Emoji' }, { value: 'lucide', label: 'Lucide' }]"
        @update:model-value="onKind"
      />
      <div class="flex flex-wrap gap-1">
        <Button
          v-for="group in groups"
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
      <div class="grid max-h-64 grid-cols-6 gap-1 overflow-y-auto">
        <template v-if="kind === 'emoji'">
          <Button
            v-for="emoji in emojiItems"
            :key="emoji"
            type="button"
            variant="ghost"
            size="icon"
            class="h-9 w-full text-lg"
            @click="select(emoji)"
          >
            {{ emoji }}
          </Button>
        </template>
        <template v-else>
          <Button
            v-for="name in lucideItems"
            :key="name"
            type="button"
            variant="ghost"
            size="icon"
            class="h-9 w-full"
            @click="select(lucideIconValue(name))"
          >
            <component :is="LUCIDE_ICON_MAP[name]" class="size-4" />
          </Button>
        </template>
      </div>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { EMOJI_GROUPS } from '@/lib/emojis'
import { LUCIDE_GROUPS, LUCIDE_ICON_MAP, isLucideIcon, lucideIconValue } from '@/lib/configIcons'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const props = defineProps<{
  show: boolean
  current?: string
}>()

const emit = defineEmits<{
  close: []
  select: [icon: string]
}>()

const kind = ref<'emoji' | 'lucide'>(isLucideIcon(props.current) ? 'lucide' : 'emoji')
const activeGroup = ref(EMOJI_GROUPS[0].id)

const groups = computed(() => kind.value === 'lucide' ? LUCIDE_GROUPS : EMOJI_GROUPS)
const emojiItems = computed(() =>
  EMOJI_GROUPS.find(group => group.id === activeGroup.value)?.items || EMOJI_GROUPS[0].items,
)
const lucideItems = computed(() =>
  LUCIDE_GROUPS.find(group => group.id === activeGroup.value)?.items || LUCIDE_GROUPS[0].items,
)

watch(() => props.show, (open) => {
  if (!open) return
  kind.value = isLucideIcon(props.current) ? 'lucide' : 'emoji'
  activeGroup.value = groups.value[0].id
})

function onKind(value: string) {
  if (value !== 'emoji' && value !== 'lucide') return
  kind.value = value
  activeGroup.value = groups.value[0].id
}

function onOpen(open: boolean) {
  if (!open) emit('close')
}

function select(icon: string) {
  emit('select', icon)
  emit('close')
}
</script>
