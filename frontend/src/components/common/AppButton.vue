<template>
  <Button
    :variant="mappedVariant"
    :size="mappedSize"
    :disabled="disabled || loading"
    @click="$emit('click', $event)"
  >
    <Loader2 v-if="loading" class="animate-spin" />
    <slot />
  </Button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Loader2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import type { ButtonVariants } from '@/components/ui/button'

interface Props {
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'destructive'
  size?: 'sm' | 'md' | 'lg'
  disabled?: boolean
  loading?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'primary',
  size: 'md',
  disabled: false,
  loading: false
})

defineEmits<{
  click: [event: MouseEvent]
}>()

const mappedVariant = computed<ButtonVariants['variant']>(() =>
  props.variant === 'primary' ? 'default' : props.variant
)

const mappedSize = computed<ButtonVariants['size']>(() =>
  props.size === 'md' ? 'default' : props.size
)
</script>
