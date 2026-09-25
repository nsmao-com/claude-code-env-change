<script setup lang="ts">
import { ComboboxRoot, ComboboxAnchor, ComboboxInput, ComboboxTrigger, ComboboxPortal, ComboboxContent, ComboboxViewport, ComboboxItem, ComboboxItemIndicator, ComboboxEmpty } from 'reka-ui'
import { Check, ChevronsUpDown } from '@lucide/vue'
import { inputStyles } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useWorkbench } from '@/composables/useWorkbench'
const props = defineProps<{ models: { id: string; name: string }[]; disabled?: boolean }>()
const model = defineModel<string>({ required: true })
const { tx } = useWorkbench()
</script>
<template>
  <ComboboxRoot
    v-model="model"
    :disabled="disabled"
    :reset-search-term-on-blur="false"
    :reset-search-term-on-select="false"
    open-on-click
  >
    <ComboboxAnchor class="relative">
      <ComboboxInput
        v-model="model"
        data-slot="input"
        required
        placeholder="gpt-5.4"
        :class="cn(inputStyles, 'h-9 pr-10')"
        :disabled="disabled"
        :aria-label="tx('模型名称', 'Model name')"
        autocomplete="off"
      />
      <ComboboxTrigger as-child>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          class="absolute right-0.5 top-0.5 size-8 rounded-lg"
          :disabled="disabled"
          :aria-label="tx('显示模型候选', 'Show model suggestions')"
        >
          <ChevronsUpDown class="size-3.5" />
        </Button>
      </ComboboxTrigger>
    </ComboboxAnchor>
    <ComboboxPortal>
      <ComboboxContent
        position="popper"
        align="start"
        :side-offset="6"
        :collision-padding="12"
        class="z-50 w-(--reka-combobox-trigger-width) min-w-56 max-w-[calc(100vw-24px)] overflow-hidden rounded-xl border border-border bg-popover p-1 text-popover-foreground shadow-lg"
      >
        <ComboboxViewport class="max-h-64 overflow-y-auto">
          <ComboboxItem
            v-for="item in models"
            :key="item.id"
            :value="item.id"
            class="relative cursor-default rounded-lg py-2 pl-3 pr-9 text-sm outline-none data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground"
          >
            <span class="block truncate">{{ item.id }}</span>
            <span v-if="item.name !== item.id" class="block truncate text-xs text-muted-foreground">
              {{ item.name }}
            </span>
            <ComboboxItemIndicator class="absolute right-3 top-2.5">
              <Check class="size-4" />
            </ComboboxItemIndicator>
          </ComboboxItem>
          <ComboboxEmpty class="px-3 py-4 text-sm text-muted-foreground">
            {{ props.models.length ? tx('没有匹配项，可直接使用输入的模型名称。', 'No matches. You can use the model name you typed.') : tx('获取模型列表后可在此选择，也可以直接输入名称。', 'Discover models to select one here, or type a name directly.') }}
          </ComboboxEmpty>
        </ComboboxViewport>
      </ComboboxContent>
    </ComboboxPortal>
  </ComboboxRoot>
</template>
