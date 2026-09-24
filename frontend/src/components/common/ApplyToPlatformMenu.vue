<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <Button size="sm" variant="outline" :disabled="disabled || applying">
        <Loader2 v-if="applying" class="animate-spin" />
        <Plus v-else />
        {{ applying ? t('ui.applying') : t('ui.applyAll') }}
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" class="w-44">
      <DropdownMenuLabel>{{ t('ui.applyToWhich') }}</DropdownMenuLabel>
      <DropdownMenuItem
        v-for="item in items"
        :key="item.key"
        @click="$emit('apply', item.key)"
      >
        <BrandIcon :provider="item.brand" class="size-3.5" />
        {{ item.label }}
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { Loader2, Plus } from '@lucide/vue'
import { PLATFORM_ITEMS, type PlatformItem } from '@/lib/platforms'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

const { t } = useI18n()

withDefaults(defineProps<{
  disabled?: boolean
  applying?: boolean
  items?: readonly PlatformItem[]
}>(), {
  items: () => PLATFORM_ITEMS,
})

defineEmits<{
  apply: [platform: PlatformItem['key']]
}>()
</script>
