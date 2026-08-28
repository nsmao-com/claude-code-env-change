<template>
  <div class="grid gap-1.5">
    <Label v-if="label">
      <span>{{ label }}</span>
      <AppTooltip v-if="tooltip" :content="tooltip" wrap>
        <span class="inline-flex cursor-help text-muted-foreground">
          <CircleHelp class="size-3.5" />
        </span>
      </AppTooltip>
    </Label>
    <div class="relative">
      <Input
        :model-value="modelValue"
        :type="type"
        :placeholder="placeholder"
        :disabled="disabled"
        :class="$slots.suffix ? 'pr-10' : undefined"
        @update:model-value="onUpdate"
        @focus="$emit('focus', $event)"
        @blur="$emit('blur', $event)"
      />
      <div v-if="$slots.suffix" class="absolute right-2 top-1/2 z-10 -translate-y-1/2">
        <slot name="suffix" />
      </div>
    </div>
    <p v-if="hint" class="text-xs text-muted-foreground">{{ hint }}</p>
  </div>
</template>

<script setup lang="ts">
import { CircleHelp } from '@lucide/vue'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import AppTooltip from '@/components/common/AppTooltip.vue'

interface Props {
  modelValue?: string
  type?: string
  placeholder?: string
  label?: string
  hint?: string
  tooltip?: string
  disabled?: boolean
}

withDefaults(defineProps<Props>(), {
  modelValue: '',
  type: 'text',
  placeholder: '',
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  focus: [event: FocusEvent]
  blur: [event: FocusEvent]
}>()

function onUpdate(value: string | number) {
  emit('update:modelValue', String(value ?? ''))
}
</script>
