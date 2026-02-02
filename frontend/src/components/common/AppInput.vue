<template>
  <div class="relative">
    <label v-if="label" class="block text-sm font-medium mb-1.5">
      {{ label }}
    </label>
    <div class="relative">
      <input
        :type="type"
        :value="modelValue"
        :placeholder="placeholder"
        :disabled="disabled"
        class="input"
        :class="{ 'pr-10': $slots.suffix }"
        @input="onInput"
        @focus="$emit('focus', $event)"
        @blur="$emit('blur', $event)"
      />
      <div v-if="$slots.suffix" class="absolute right-2 top-1/2 -translate-y-1/2">
        <slot name="suffix" />
      </div>
    </div>
    <p v-if="hint" class="text-xs text-muted-foreground mt-1">{{ hint }}</p>
  </div>
</template>

<script setup lang="ts">
interface Props {
  modelValue?: string
  type?: string
  placeholder?: string
  label?: string
  hint?: string
  disabled?: boolean
}

withDefaults(defineProps<Props>(), {
  modelValue: '',
  type: 'text',
  placeholder: '',
  disabled: false
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  focus: [event: FocusEvent]
  blur: [event: FocusEvent]
}>()

function onInput(event: Event) {
  const target = event.target as HTMLInputElement
  emit('update:modelValue', target.value)
}
</script>
