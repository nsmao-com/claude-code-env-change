<template>
  <AppModal v-model="isOpen" title="导出 MCP 配置" size="xl">
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <SegmentedPills
          :model-value="targetId"
          layout-id="mcp-export-target"
          dense
          :items="pillItems"
          @update:model-value="targetId = $event"
        />
        <span class="text-xs text-muted-foreground">{{ servers.length }} 个服务器</span>
      </div>

      <p class="text-xs leading-relaxed text-muted-foreground">
        {{ target.label }} 写入位置：{{ target.pathHint }}。生成的是配置片段，
        对 <span class="font-mono">~/.claude.json</span>、
        <span class="font-mono">config.toml</span> 这类混合配置请合并粘贴，不要整文件覆盖。
      </p>

      <div
        v-if="servers.length === 0"
        class="flex h-40 items-center justify-center rounded-xl border border-dashed border-border text-sm text-muted-foreground"
      >
        当前没有 MCP 服务器，先「添加」或「JSON 导入」
      </div>
      <template v-else>
        <CodeEditor :model-value="content" :language="target.language" max-height="52vh" />
        <p class="text-[11px] text-muted-foreground">
          提示：可以在这里直接微调再复制；切换目标格式会重新生成。Secret Key 等敏感值会原样导出，注意保管。
        </p>
      </template>
    </div>

    <template #footer>
      <div class="flex w-full items-center justify-end gap-2">
        <Button variant="outline" size="sm" :disabled="servers.length === 0" @click="copy">
          <Check v-if="copied" class="text-emerald-600" />
          <Copy v-else />
          {{ copied ? '已复制' : '复制内容' }}
        </Button>
        <Button size="sm" :disabled="servers.length === 0 || saving" @click="save">
          <Loader2 v-if="saving" class="animate-spin" />
          <Download v-else />
          保存文件
        </Button>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { Check, Copy, Download, Loader2 } from '@lucide/vue'
import type { MCPServer } from '@/types'
import { MCP_EXPORT_TARGETS } from '@/lib/mcpExport'
import { useToast } from '@/composables/useToast'
import { callApp } from '@/services/appBridge'
import AppModal from '@/components/common/AppModal.vue'
import CodeEditor from '@/components/common/CodeEditor.vue'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import { Button } from '@/components/ui/button'

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

const pillItems = MCP_EXPORT_TARGETS.map(item => ({ value: item.id, label: item.label }))
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
    toast.error('复制失败：' + (e instanceof Error ? e.message : String(e)))
  }
}

const saving = ref(false)
async function save() {
  saving.value = true
  try {
    const path = await callApp<string>('SaveTextFile', content.value, target.value.fileName)
    if (path) toast.success('已保存到 ' + path)
  } catch (e: unknown) {
    toast.error('保存失败：' + (e instanceof Error ? e.message : String(e)))
  } finally {
    saving.value = false
  }
}

onBeforeUnmount(() => clearTimeout(copiedTimer))
</script>
