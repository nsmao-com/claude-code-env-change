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
        v-for="item in PLATFORM_ITEMS"
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
import { PLATFORM_ITEMS, type PlatformKey } from '@/lib/platforms'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

defineProps<{
  disabled?: boolean
  applying?: boolean
}>()

defineEmits<{
  apply: [platform: PlatformKey]
}>()
</script>
