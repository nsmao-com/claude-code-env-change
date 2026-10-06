<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Loader2, RotateCcw, Trash2 } from '@lucide/vue'
import { ref } from 'vue'
import { useWorkbench } from '@/composables/useWorkbench'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import { callService } from '@/services/appBridge'
import type { TrashedSession } from '@/types/workbench'

const emit = defineEmits<{ restored: [] }>()
const { tx } = useWorkbench()
const confirm = useConfirm()
const toast = useToast()
const items = ref<TrashedSession[]>([])
const loading = ref(false)
const busyId = ref('')
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    items.value = (await callService<TrashedSession[]>('SessionService', 'ListTrashedSessions')) || []
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}
void load()
defineExpose({ load })

function size(bytes: number) {
  if (bytes >= 1 << 20) return `${(bytes / (1 << 20)).toFixed(1)} MB`
  if (bytes >= 1 << 10) return `${(bytes / (1 << 10)).toFixed(0)} KB`
  return `${bytes} B`
}

async function restore(item: TrashedSession) {
  if (busyId.value) return
  busyId.value = item.id
  try {
    await callService('SessionService', 'RestoreTrashedSession', item.id)
    toast.success(tx('会话已恢复到原位置', 'Session restored to its original location'))
    emit('restored')
    await load()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    busyId.value = ''
  }
}

async function purge(item?: TrashedSession) {
  if (busyId.value) return
  const title = item ? tx('永久删除会话', 'Delete session permanently') : tx('清空回收站', 'Empty recycle bin')
  const message = item
    ? tx(`将永久删除「${item.title}」的原始日志，无法恢复。`, `The original log of “${item.title}” will be deleted permanently. This cannot be undone.`)
    : tx(`将永久删除回收站中的 ${items.value.length} 个会话日志，无法恢复。`, `All ${items.value.length} session logs in the recycle bin will be deleted permanently. This cannot be undone.`)
  if (!(await confirm.show(title, message, 'danger'))) return
  busyId.value = item?.id || '*'
  try {
    const n = await callService<number>('SessionService', 'PurgeTrashedSessions', item ? [item.id] : [])
    toast.success(tx(`已永久删除 ${n} 个会话`, `${n} sessions deleted permanently`))
    await load()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    busyId.value = ''
  }
}
</script>

<template>
  <div class="wb-card">
    <div class="wb-row justify-between !mb-0">
      <div>
        <h2>{{ tx('回收站', 'Recycle bin') }}</h2>
        <p class="wb-hint !mb-0">
          {{
            tx(
              '在会话详情里删除的会话会先移到这里（含 Claude Code 的子代理与工具结果目录），可以随时恢复到原位置；清空后才会真正删除。',
              'Sessions deleted from the detail view land here (with Claude Code’s subagent and tool-result folders) and can be restored to their original place; emptying the bin deletes them for good.',
            )
          }}
        </p>
      </div>
      <Button
        v-if="items.length"
        variant="ghost"
        size="sm"
        type="button"
        class="text-destructive"
        :disabled="!!busyId"
        @click="purge()"
      >
        <Trash2 />
        {{ tx('清空回收站', 'Empty bin') }}
      </Button>
    </div>
  </div>
  <p v-if="error" role="alert" class="wb-error">{{ error }}</p>
  <div v-if="loading && !items.length" class="space-y-2" aria-busy="true">
    <div v-for="i in 3" :key="i" class="h-16 animate-pulse rounded-xl bg-muted" />
  </div>
  <div v-else-if="!items.length" class="wb-empty">
    {{ tx('回收站是空的。', 'The recycle bin is empty.') }}
  </div>
  <ul v-else class="divide-y divide-border rounded-xl border border-border bg-card">
    <li v-for="item in items" :key="item.id" class="flex flex-wrap items-center gap-3 px-4 py-3">
      <div class="min-w-0 flex-1">
        <p class="truncate text-sm font-medium" :title="item.title">{{ item.title }}</p>
        <p class="truncate text-xs text-muted-foreground" :title="item.original">
          {{ item.provider === 'claude' ? 'Claude Code' : item.provider === 'codex' ? 'Codex' : item.provider }}
          · {{ tx('删除于', 'Deleted') }} {{ new Date(item.deleted_at).toLocaleString() }} · {{ size(item.size) }}
        </p>
        <p class="truncate text-[11px] text-muted-foreground">{{ item.project }}</p>
      </div>
      <Button variant="outline" size="sm" type="button" :disabled="!!busyId" @click="restore(item)">
        <Loader2 v-if="busyId === item.id" class="animate-spin" />
        <RotateCcw v-else />
        {{ tx('恢复', 'Restore') }}
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        type="button"
        :aria-label="tx('永久删除', 'Delete permanently')"
        :title="tx('永久删除', 'Delete permanently')"
        :disabled="!!busyId"
        @click="purge(item)"
      >
        <Trash2 class="text-destructive" />
      </Button>
    </li>
  </ul>
</template>
