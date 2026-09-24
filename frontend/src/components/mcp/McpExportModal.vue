<template>
  <AppModal v-model="isOpen" :title="t('mcp.exportModal.title')" size="xl">
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <SegmentedPills
          :model-value="targetId"
          layout-id="mcp-export-target"
          dense
          :items="pillItems"
          @update:model-value="targetId = $event"
        />
        <span class="text-xs text-muted-foreground">{{ t('mcp.exportModal.serverCount', { count: servers.length }) }}</span>
      </div>

      <p class="text-xs leading-relaxed text-muted-foreground">
        {{ t('mcp.exportModal.pathBefore', { label: targetLabel(target), path: t(`mcp.exportModal.pathHint.${target.id}`) }) }}
        <span class="font-mono">~/.claude.json</span>{{ t('mcp.exportModal.pathMid') }}
        <span class="font-mono">config.toml</span> {{ t('mcp.exportModal.pathAfter') }}
      </p>

      <div
        v-if="servers.length === 0"
        class="flex h-40 items-center justify-center rounded-xl border border-dashed border-border text-sm text-muted-foreground"
      >
        {{ t('mcp.exportModal.empty') }}
      </div>
      <template v-else>
        <CodeEditor :model-value="content" :language="target.language" max-height="52vh" />
        <p class="text-[11px] text-muted-foreground">
          {{ t('mcp.exportModal.tip') }}
        </p>
      </template>
    </div>

    <template #footer>
      <div class="flex w-full items-center justify-end gap-2">
        <Button variant="outline" size="sm" :disabled="servers.length === 0" @click="copy">
          <Check v-if="copied" class="text-emerald-600" />
          <Copy v-else />
          {{ copied ? t('mcp.exportModal.copied') : t('mcp.exportModal.copy') }}
        </Button>
        <Button size="sm" :disabled="servers.length === 0 || saving" @click="save">
          <Loader2 v-if="saving" class="animate-spin" />
          <Download v-else />
          {{ t('mcp.exportModal.saveFile') }}
        </Button>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { computed, onBeforeUnmount, ref } from 'vue'
import { Check, Copy, Download, Loader2 } from '@lucide/vue'
import type { MCPServer } from '@/types'
import { MCP_EXPORT_TARGETS, type McpExportTarget } from '@/lib/mcpExport'
import { useToast } from '@/composables/useToast'
import { callApp } from '@/services/appBridge'
import AppModal from '@/components/common/AppModal.vue'
import CodeEditor from '@/components/common/CodeEditor.vue'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import { Button } from '@/components/ui/button'

const { t } = useI18n()

const props = defineProps<{
  modelValue: boolean
  servers: MCPServer[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
})

const toast = useToast()

function targetLabel(item: McpExportTarget) {
  return item.id === 'generic' ? t('mcp.exportModal.generic') : item.label
}

const pillItems = computed(() => MCP_EXPORT_TARGETS.map(item => ({ value: item.id, label: targetLabel(item) })))
const targetId = ref(MCP_EXPORT_TARGETS[0]!.id)
const target = computed(() => MCP_EXPORT_TARGETS.find(item => item.id === targetId.value) || MCP_EXPORT_TARGETS[0]!)

const content = computed(() => (props.servers.length === 0 ? '' : target.value.build(props.servers)))

const copied = ref(false)
let copiedTimer: ReturnType<typeof setTimeout> | undefined
async function copy() {
  try {
    await navigator.clipboard.writeText(content.value)
    copied.value = true
    clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => {
      copied.value = false
    }, 1600)
  } catch (e: unknown) {
    toast.error(t('mcp.exportModal.copyFailed', { error: e instanceof Error ? e.message : String(e) }))
  }
}

const saving = ref(false)
async function save() {
  saving.value = true
  try {
    const path = await callApp<string>('SaveTextFile', content.value, target.value.fileName)
    if (path) toast.success(t('mcp.exportModal.savedTo', { path }))
  } catch (e: unknown) {
    toast.error(t('mcp.exportModal.saveFailed', { error: e instanceof Error ? e.message : String(e) }))
  } finally {
    saving.value = false
  }
}

onBeforeUnmount(() => clearTimeout(copiedTimer))
</script>
