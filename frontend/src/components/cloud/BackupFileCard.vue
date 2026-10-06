<template>
  <Card class="mb-4">
    <CardHeader>
      <CardTitle>{{ tx('加密备份文件', 'Encrypted backup file') }}</CardTitle>
      <CardDescription>
        {{
          tx(
            '不用对象存储也能换电脑：把环境、MCP、Skills、路由、监控和工作台配置加密导出成一个文件，在新电脑导入后按文件选择恢复。',
            'Move to another computer without object storage: export environments, MCP, skills, routes, monitoring and workbench settings into one encrypted file, then import and pick what to restore.',
          )
        }}
      </CardDescription>
    </CardHeader>
    <CardContent>
      <SegmentedPills v-model="mode" layout-id="backup-file-mode" dense :items="modes" />

      <form v-if="mode === 'export'" class="mt-4 space-y-3" @submit.prevent="exportFile">
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <AppInput
            v-model="passphrase"
            type="password"
            :label="tx('加密口令（至少 8 个字符）', 'Passphrase (8+ characters)')"
          />
          <AppInput
            v-model="confirmPassphrase"
            type="password"
            :label="tx('再次输入口令', 'Repeat passphrase')"
          />
        </div>
        <p v-if="mismatch" role="alert" class="text-xs text-destructive">{{ tx('两次输入的口令不一致', 'Passphrases do not match') }}</p>
        <p class="text-[11px] leading-relaxed text-muted-foreground">
          {{
            tx(
              '文件包含 API Key，使用 scrypt + AES-256-GCM 加密；忘记口令将无法恢复，请妥善保管。',
              'The file contains API keys and is encrypted with scrypt + AES-256-GCM. It cannot be restored without the passphrase.',
            )
          }}
        </p>
        <Button type="submit" size="sm" :disabled="busy || !canExport">
          <Loader2 v-if="busy" class="animate-spin" />
          <FileDown v-else />
          {{ tx('导出备份文件', 'Export backup file') }}
        </Button>
      </form>

      <div v-else class="mt-4 space-y-3">
        <form class="flex flex-wrap items-end gap-3" @submit.prevent="previewFile">
          <div class="min-w-[240px] flex-1">
            <AppInput
              v-model="importPassphrase"
              type="password"
              :label="tx('导出时设置的口令', 'Passphrase used when exporting')"
            />
          </div>
          <Button type="submit" size="sm" variant="outline" :disabled="busy || !importPassphrase.trim()">
            <Loader2 v-if="busy" class="animate-spin" />
            <FileUp v-else />
            {{ tx('选择备份文件并预览', 'Choose file and preview') }}
          </Button>
        </form>
        <div v-if="preview" class="workbench">
          <DiffPreview v-model="selected" :changes="preview.changes" />
          <Button class="mt-4" size="sm" :disabled="busy || !selected.length" @click="restore">
            {{ tx('恢复所选文件', 'Restore selected files') }}
          </Button>
        </div>
      </div>
    </CardContent>
  </Card>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { FileDown, FileUp, Loader2 } from '@lucide/vue'
import { useWorkbench } from '@/composables/useWorkbench'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { callService } from '@/services/appBridge'
import type { HistoryPreview, Result } from '@/types/workbench'
import AppInput from '@/components/common/AppInput.vue'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import DiffPreview from '@/components/workbench/DiffPreview.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const emit = defineEmits<{ restored: [] }>()
const { tx } = useWorkbench()
const toast = useToast()
const confirm = useConfirm()

const mode = ref('export')
const modes = computed(() => [
  { value: 'export', label: tx('导出', 'Export') },
  { value: 'import', label: tx('导入', 'Import') },
])
const passphrase = ref('')
const confirmPassphrase = ref('')
const importPassphrase = ref('')
const busy = ref(false)
const preview = ref<HistoryPreview | null>(null)
const selected = ref<string[]>([])

const mismatch = computed(() => confirmPassphrase.value !== '' && confirmPassphrase.value !== passphrase.value)
const canExport = computed(() => passphrase.value.trim().length >= 8 && passphrase.value === confirmPassphrase.value)

watch(mode, () => {
  preview.value = null
})

async function exportFile() {
  if (!canExport.value || busy.value) return
  busy.value = true
  try {
    const path = await callService<string>('CloudSyncService', 'ExportBackupFile', passphrase.value)
    if (path) {
      toast.success(tx(`已导出到 ${path}`, `Exported to ${path}`))
      passphrase.value = ''
      confirmPassphrase.value = ''
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    busy.value = false
  }
}

async function previewFile() {
  if (busy.value || !importPassphrase.value.trim()) return
  busy.value = true
  try {
    const result = await callService<HistoryPreview>('CloudSyncService', 'PreviewBackupFile', importPassphrase.value)
    if (!result?.token) return
    preview.value = result
    selected.value = result.changes.filter((c) => c.changed).map((c) => c.path)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    busy.value = false
  }
}

async function restore() {
  if (!preview.value || busy.value) return
  const ok = await confirm.show(
    tx('从备份文件恢复', 'Restore from backup file'),
    tx(
      `将用备份中的内容覆盖本机的 ${selected.value.length} 个配置文件，覆盖前会自动留存 .bak 备份。确定恢复？`,
      `${selected.value.length} local configuration files will be replaced by the backup; a .bak copy is kept first. Restore?`,
    ),
    'warning',
  )
  if (!ok) return
  busy.value = true
  try {
    const result = await callService<Result>('CloudSyncService', 'ConfirmBackupFileRestore', preview.value.token, selected.value)
    if (!result.success) throw new Error(result.message)
    toast.success(result.message)
    preview.value = null
    importPassphrase.value = ''
    emit('restored')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    busy.value = false
  }
}
</script>
