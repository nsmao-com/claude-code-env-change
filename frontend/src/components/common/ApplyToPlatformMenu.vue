<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <Button size="sm" variant="outline" :disabled="disabled || applying">
        <Loader2 v-if="applying" class="animate-spin" />
        <Plus v-else />
        {{ applying ? '加入中...' : '一键加入' }}
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" class="w-44">
      <DropdownMenuLabel>加入到哪个平台</DropdownMenuLabel>
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
