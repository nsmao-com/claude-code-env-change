<template>
  <div class="flex flex-wrap items-center gap-1.5">
    <AppTooltip
      v-for="item in PLATFORM_ITEMS"
      :key="item.key"
      :content="on.has(item.key) ? `从 ${item.label} 移除` : `加入 ${item.label}`"
    >
      <button
        type="button"
        :disabled="disabled"
        :class="[
          'inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[10px] transition-colors',
          on.has(item.key) ? item.onClass : item.offClass,
          disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer hover:opacity-90',
          compact ? 'h-6 px-1' : '',
        ]"
        @click.stop="$emit('toggle', item.key)"
      >
        <BrandIcon :provider="item.brand" class="size-3" />
        <span v-if="!compact">{{ item.label }}</span>
      </button>
    </AppTooltip>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { PLATFORM_ITEMS } from '@/lib/platforms'
import AppTooltip from '@/components/common/AppTooltip.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'

const props = defineProps<{
  enabled: string[]
  compact?: boolean
  disabled?: boolean
}>()

defineEmits<{
  toggle: [platform: string]
}>()

const on = computed(() => new Set(props.enabled || []))
</script>
