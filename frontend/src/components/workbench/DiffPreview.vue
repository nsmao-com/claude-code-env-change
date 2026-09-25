<script setup lang="ts">
import { useWorkbench } from '@/composables/useWorkbench'
import type { FileChange } from '@/types/workbench'
defineProps<{ changes: FileChange[] }>()
const selected = defineModel<string[]>({ default: () => [] })
const { tx } = useWorkbench()
</script>
<template>
  <div class="space-y-3">
    <p class="wb-hint">
      {{
        tx(
          '密钥已脱敏。勾选要恢复的文件；确认时会重新检查版本。',
          'Secrets are redacted. Select files to restore; versions are rechecked on confirmation.',
        )
      }}
    </p>
    <details v-for="change in changes" :key="change.path" class="rounded-lg border border-border p-3">
      <summary class="cursor-pointer break-all text-sm">
        <label class="wb-check inline-flex" @click.stop
          ><input type="checkbox" v-model="selected" :value="change.path" />{{ change.path }}</label
        ><span class="ml-3 text-xs text-muted-foreground">{{
          change.changed ? tx('有变化', 'Changed') : tx('内容相同', 'Unchanged')
        }}</span>
      </summary>
      <div class="wb-grid mt-4">
        <div>
          <h3>{{ tx('当前本地', 'Current local') }}</h3>
          <pre>{{ change.before || tx('文件不存在', 'File does not exist') }}</pre>
        </div>
        <div>
          <h3>{{ tx('将恢复为', 'Restore to') }}</h3>
          <pre>{{ change.after }}</pre>
        </div>
      </div>
    </details>
  </div>
</template>
