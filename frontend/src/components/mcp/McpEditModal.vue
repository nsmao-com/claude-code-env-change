<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑 MCP 服务器' : '添加 MCP 服务器'" size="md">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <AppInput
        v-model="form.name"
        label="服务器名称"
        placeholder="输入服务器名称"
      />

      <div class="grid gap-1.5">
        <Label>类型</Label>
        <ToggleGroup
          type="single"
          :model-value="form.type"
          variant="outline"
          class="w-full"
          @update:model-value="onType"
        >
          <ToggleGroupItem value="stdio" class="flex-1">
            <Terminal />
            Stdio
          </ToggleGroupItem>
          <ToggleGroupItem value="http" class="flex-1">
            <Globe />
            HTTP
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      <div v-if="form.type === 'stdio'" class="space-y-4">
        <AppInput
          v-model="form.command"
          label="Command"
          placeholder="npx"
        />
        <div class="grid gap-1.5">
          <Label>Args (每行一个)</Label>
          <Textarea
            v-model="form.args"
            class="min-h-24 font-mono text-xs"
            placeholder="-y&#10;@modelcontextprotocol/server-filesystem"
          />
        </div>
        <div class="grid gap-1.5">
          <Label>环境变量 (KEY=VALUE)</Label>
          <Textarea
            v-model="form.env"
            class="min-h-24 font-mono text-xs"
            placeholder="API_KEY=xxx&#10;DEBUG=true"
          />
        </div>
      </div>

      <div v-if="form.type === 'http'" class="space-y-4">
        <AppInput
          v-model="form.url"
          label="URL"
          placeholder="http://localhost:3000"
        />
      </div>

      <div class="space-y-4 border-t border-border pt-4">
        <AppInput
          v-model="form.website"
          label="官网 (可选)"
          placeholder="https://..."
        />
        <AppInput
          v-model="form.tips"
          label="备注 (可选)"
          placeholder="服务器说明..."
        />

        <div class="grid gap-1.5">
          <Label>启用平台</Label>
          <ToggleGroup
            type="multiple"
            :model-value="selectedPlatformKeys"
            variant="outline"
            class="w-full"
            @update:model-value="onPlatforms"
          >
            <ToggleGroupItem value="claude" class="flex-1">
              <Bot />
              Claude
              <Check v-if="form.platforms.claude" />
            </ToggleGroupItem>
            <ToggleGroupItem value="codex" class="flex-1">
              <Terminal />
              Codex
              <Check v-if="form.platforms.codex" />
            </ToggleGroupItem>
            <ToggleGroupItem value="gemini" class="flex-1">
              <Gem />
              Gemini
              <Check v-if="form.platforms.gemini" />
            </ToggleGroupItem>
          </ToggleGroup>
        </div>
      </div>
    </form>

    <template #footer>
      <Button type="button" variant="outline" @click="isOpen = false">
        取消
      </Button>
      <Button type="button" @click="handleSubmit">
        {{ isEditing ? '保存' : '添加' }}
      </Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Bot, Check, Gem, Globe, Terminal } from '@lucide/vue'
import type { MCPServer } from '@/types'
import { useMcpStore } from '@/stores/mcpStore'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

interface Props {
  modelValue: boolean
  editServer?: MCPServer | null
  editIndex?: number
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const mcpStore = useMcpStore()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isEditing = computed(() => props.editServer != null && props.editIndex !== undefined)

const defaultForm = () => ({
  name: '',
  type: 'stdio' as 'stdio' | 'http',
  command: '',
  args: '',
  env: '',
  url: '',
  website: '',
  tips: '',
  platforms: {
    claude: true,
    codex: false,
    gemini: false
  }
})

const form = ref(defaultForm())

const selectedPlatformKeys = computed(() => {
  const keys: string[] = []
  if (form.value.platforms.claude) keys.push('claude')
  if (form.value.platforms.codex) keys.push('codex')
  if (form.value.platforms.gemini) keys.push('gemini')
  return keys
})

function onType(value: unknown) {
  if (value === 'stdio' || value === 'http') {
    form.value.type = value
  }
}

function onPlatforms(value: unknown) {
  const keys = Array.isArray(value) ? value : []
  form.value.platforms.claude = keys.includes('claude')
  form.value.platforms.codex = keys.includes('codex')
  form.value.platforms.gemini = keys.includes('gemini')
}

function fillFormFromServer(server: MCPServer) {
  if (server) {
    form.value.name = server.name || ''
    form.value.type = (server.type || 'stdio') as 'stdio' | 'http'
    form.value.command = server.command || ''
    form.value.args = (server.args || []).join('\n')
    form.value.env = Object.entries(server.env || {})
      .map(([k, v]) => `${k}=${v}`)
      .join('\n')
    form.value.url = server.url || ''
    form.value.website = server.website || ''
    form.value.tips = server.tips || ''
    const platforms = server.enable_platform || []
    form.value.platforms.claude = platforms.includes('claude-code')
    form.value.platforms.codex = platforms.includes('codex')
    form.value.platforms.gemini = platforms.includes('gemini')
  }
}

function syncFormFromProps() {
  if (props.editServer) {
    fillFormFromServer(props.editServer)
    return
  }
  form.value = defaultForm()
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      // 每次打开都同步一次，避免“同一对象二次编辑”不触发 watcher 导致空表单
      syncFormFromProps()
      return
    }
    form.value = defaultForm()
  }
)

watch(
  () => [props.editServer, props.editIndex] as const,
  () => {
    if (props.modelValue) {
      syncFormFromProps()
    }
  }
)

async function handleSubmit() {
  const name = form.value.name.trim()
  if (!name) {
    toast.error('请输入服务器名称')
    return
  }

  const exists = mcpStore.servers.some(
    (s, i) => s.name === name && i !== props.editIndex
  )
  if (exists) {
    toast.error('服务器名称已存在')
    return
  }

  if (form.value.type === 'http' && !form.value.url.trim()) {
    toast.error('请输入 URL')
    return
  }
  if (form.value.type === 'stdio' && !form.value.command.trim()) {
    toast.error('请输入 Command')
    return
  }

  const enablePlatform: string[] = []
  if (form.value.platforms.claude) enablePlatform.push('claude-code')
  if (form.value.platforms.codex) enablePlatform.push('codex')
  if (form.value.platforms.gemini) enablePlatform.push('gemini')

  const args = form.value.args.trim()
    ? form.value.args.split('\n').map(s => s.trim()).filter(s => s)
    : []

  const env: Record<string, string> = {}
  if (form.value.env.trim()) {
    form.value.env.split('\n').forEach(line => {
      const idx = line.indexOf('=')
      if (idx > 0) {
        const key = line.substring(0, idx).trim()
        const value = line.substring(idx + 1).trim()
        if (key) env[key] = value
      }
    })
  }

  const serverData: MCPServer = {
    name,
    type: form.value.type,
    command: form.value.type === 'stdio' ? form.value.command.trim() : undefined,
    args: form.value.type === 'stdio' ? args : undefined,
    env: form.value.type === 'stdio' ? env : undefined,
    url: form.value.type === 'http' ? form.value.url.trim() : undefined,
    website: form.value.website.trim() || undefined,
    tips: form.value.tips.trim() || undefined,
    enable_platform: enablePlatform,
    enabled_in_claude: false,
    enabled_in_codex: false,
    enabled_in_gemini: false,
    missing_placeholders: []
  }

  try {
    if (isEditing.value && props.editIndex !== undefined) {
      await mcpStore.updateServer(props.editIndex, serverData)
    } else {
      await mcpStore.addServer(serverData)
    }
    toast.success('MCP 服务器已保存')
    isOpen.value = false
    emit('saved')
  } catch (e: any) {
    toast.error('保存失败: ' + e.message)
  }
}
</script>
