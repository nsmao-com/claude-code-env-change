<script setup lang="ts">
import { NumberFieldRoot, NumberFieldInput, NumberFieldDecrement, NumberFieldIncrement } from 'reka-ui'
import { Minus, Plus } from '@lucide/vue'
import { inputStyles } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useWorkbench } from '@/composables/useWorkbench'
withDefaults(defineProps<{ min?: number; max?: number; step?: number; disabled?: boolean }>(), { min: 0, step: 1 })
const model = defineModel<number>({ required: true })
const { tx } = useWorkbench()
function update(value: number) {
  model.value = Number.isFinite(value) ? value : 0
}
</script>
<template>
  <NumberFieldRoot
    :model-value="model"
    :min="min"
    :max="max"
    :step="step"
    :disabled="disabled"
    :step-snapping="false"
    :format-options="{ useGrouping: false, maximumFractionDigits: step >= 1 ? 0 : 8 }"
    disable-wheel-change
    class="relative"
    @update:model-value="update"
  >
    <NumberFieldInput data-slot="input" :class="cn(inputStyles, 'h-9 px-10 text-center tabular-nums')" />
    <NumberFieldDecrement as-child>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        class="absolute left-0.5 top-0.5 size-8 rounded-lg"
        :aria-label="tx('减少', 'Decrease')"
      >
        <Minus class="size-3.5" />
      </Button>
    </NumberFieldDecrement>
    <NumberFieldIncrement as-child>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        class="absolute right-0.5 top-0.5 size-8 rounded-lg"
        :aria-label="tx('增加', 'Increase')"
      >
        <Plus class="size-3.5" />
      </Button>
    </NumberFieldIncrement>
  </NumberFieldRoot>
</template>
