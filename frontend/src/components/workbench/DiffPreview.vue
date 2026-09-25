<script setup lang="ts">
import { CheckboxGroupRoot, CollapsibleRoot, CollapsibleTrigger, CollapsibleContent } from 'reka-ui'
import { ChevronDown } from '@lucide/vue'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import { useWorkbench } from '@/composables/useWorkbench'
import type { FileChange } from '@/types/workbench'
defineProps<{ changes: FileChange[] }>()
const selected = defineModel<string[]>({ default: () => [] })
const { tx } = useWorkbench()
</script>
<template>
  <CheckboxGroupRoot v-model="selected" :roving-focus="false" class="space-y-3">
    <p class="wb-hint">
      {{
        tx(
          '密钥已脱敏。勾选要恢复的文件；确认时会重新检查版本。',
          'Secrets are redacted. Select files to restore; versions are rechecked on confirmation.',
        )
      }}
    </p>
    <CollapsibleRoot
      v-for="change in changes"
      :key="change.path"
      v-slot="{ open }"
      class="rounded-lg border border-border p-3"
    >
      <div class="flex flex-wrap items-center gap-3 text-sm">
        <label class="wb-check flex min-w-0 flex-1 items-center gap-2 break-all">
          <Checkbox :value="change.path" />
          {{ change.path }}
        </label>
        <span class="text-xs text-muted-foreground">
          {{
          change.changed ? tx('有变化', 'Changed') : tx('内容相同', 'Unchanged')
        }}
        </span>
        <CollapsibleTrigger as-child>
          <Button type="button" variant="ghost" size="sm">
            {{ open ? tx('收起差异', 'Hide changes') : tx('查看差异', 'Show changes') }}
            <ChevronDown
              class="size-4 transition-transform motion-reduce:transition-none"
              :class="{ 'rotate-180': open }"
            />
          </Button>
        </CollapsibleTrigger>
      </div>
      <CollapsibleContent class="wb-grid mt-4">
        <div>
          <h3>{{ tx('当前本地', 'Current local') }}</h3>
          <pre>{{ change.before || tx('文件不存在', 'File does not exist') }}</pre>
        </div>
        <div>
          <h3>{{ tx('将恢复为', 'Restore to') }}</h3>
          <pre>{{ change.after }}</pre>
        </div>
      </CollapsibleContent>
    </CollapsibleRoot>
  </CheckboxGroupRoot>
</template>
