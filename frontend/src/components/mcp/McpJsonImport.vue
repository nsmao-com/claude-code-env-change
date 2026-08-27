<template>
  <AppModal v-model="isOpen" title="导入 MCP 服务器" size="lg">
    <div class="space-y-4">
      <p class="text-sm text-muted-foreground">
        粘贴 MCP 配置 JSON，支持 Claude/Codex/Gemini 格式
      </p>

      <div class="rounded-lg bg-muted p-3 font-mono text-xs text-muted-foreground">
        <p>• mcpServers 对象: {"mcpServers": {...}}</p>
        <p>• 服务器列表对象: {"server1": {...}, "server2": {...}}</p>
        <p>• 单个服务器: {"command": "npx", "args": [...]}</p>
      </div>

      <div class="grid gap-1.5">
        <Label>导入到平台</Label>
        <ToggleGroup
          type="multiple"
          :model-value="selectedPlatformKeys"
          variant="outline"
          class="w-full"
          @update:model-value="onPlatforms"
        >
          <ToggleGroupItem value="claude" class="flex-1">
            <BrandIcon provider="claude" />
            Claude
            <Check v-if="selectedPlatforms.claude" />
          </ToggleGroupItem>
          <ToggleGroupItem value="codex" class="flex-1">
            <BrandIcon provider="codex" />
            Codex
            <Check v-if="selectedPlatforms.codex" />
          </ToggleGroupItem>
          <ToggleGroupItem value="gemini" class="flex-1">
            <BrandIcon provider="gemini" />
            Gemini
            <Check v-if="selectedPlatforms.gemini" />
          </ToggleGroupItem>
          <ToggleGroupItem value="opencode" class="flex-1">
            <BrandIcon provider="opencode" />
            OpenCode
            <Check v-if="selectedPlatforms.opencode" />
          </ToggleGroupItem>
          <ToggleGroupItem value="grok" class="flex-1">
            <BrandIcon provider="grok" />
            Grok
            <Check v-if="selectedPlatforms.grok" />
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      <div class="grid gap-1.5">
        <Label>JSON 内容</Label>
        <Textarea
          v-model="jsonInput"
          class="min-h-64 font-mono text-xs"
          placeholder='{"mcpServers": {"filesystem": {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem"]}}}'
        />
      </div>
    </div>

    <template #footer>
      <Button type="button" variant="outline" @click="isOpen = false">
        取消
      </Button>
      <Button
        type="button"
        :disabled="isImporting || !hasSelectedPlatform"
        @click="handleImport"
      >
        <Loader2 v-if="isImporting" class="animate-spin" />
        导入
      </Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Check, Loader2 } from '@lucide/vue'
import { useMcpStore } from '@/stores/mcpStore'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

interface Props {
  modelValue: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  imported: []
}>()

const mcpStore = useMcpStore()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const jsonInput = ref('')
const isImporting = ref(false)
const selectedPlatforms = ref({
  claude: true,
  codex: false,
  gemini: false,
  opencode: false,
  grok: false,
})

const hasSelectedPlatform = computed(() => {
  return selectedPlatforms.value.claude || selectedPlatforms.value.codex || selectedPlatforms.value.gemini || selectedPlatforms.value.opencode || selectedPlatforms.value.grok
})

const selectedPlatformKeys = computed(() => {
  const keys: string[] = []
  if (selectedPlatforms.value.claude) keys.push('claude')
  if (selectedPlatforms.value.codex) keys.push('codex')
  if (selectedPlatforms.value.gemini) keys.push('gemini')
  if (selectedPlatforms.value.opencode) keys.push('opencode')
  if (selectedPlatforms.value.grok) keys.push('grok')
  return keys
})

function onPlatforms(value: unknown) {
  const keys = Array.isArray(value) ? value : []
  selectedPlatforms.value = {
    claude: keys.includes('claude'),
    codex: keys.includes('codex'),
    gemini: keys.includes('gemini'),
    opencode: keys.includes('opencode'),
    grok: keys.includes('grok'),
  }
}

watch(isOpen, (open) => {
  if (!open) {
    jsonInput.value = ''
    selectedPlatforms.value = { claude: true, codex: false, gemini: false, opencode: false, grok: false }
  }
})

async function handleImport() {
  if (!jsonInput.value.trim()) {
    toast.error('请输入 JSON 内容')
    return
  }

  if (!hasSelectedPlatform.value) {
    toast.error('请至少选择一个平台')
    return
  }

  isImporting.value = true
  try {
    const servers = await mcpStore.importFromJSON(jsonInput.value)
    if (!servers || servers.length === 0) {
      toast.error('没有找到有效的服务器配置')
      return
    }

    const platforms: string[] = []
    if (selectedPlatforms.value.claude) platforms.push('claude-code')
    if (selectedPlatforms.value.codex) platforms.push('codex')
    if (selectedPlatforms.value.gemini) platforms.push('gemini')
    if (selectedPlatforms.value.opencode) platforms.push('opencode')
    if (selectedPlatforms.value.grok) platforms.push('grok')

    servers.forEach(server => {
      server.enable_platform = platforms
    })

    await mcpStore.addServers(servers)
    toast.success(`成功导入 ${servers.length} 个 MCP 服务器`)
    isOpen.value = false
    emit('imported')
  } catch (e: any) {
    toast.error('导入失败: ' + e.message)
  } finally {
    isImporting.value = false
  }
}
</script>
