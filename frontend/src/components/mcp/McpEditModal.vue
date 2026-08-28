<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑 MCP 服务器' : '添加 MCP 服务器'" size="lg">
    <div class="space-y-4">
      <SegmentedPills
        :model-value="editorMode"
        layout-id="mcp-editor-mode"
        dense
        :items="[{ value: 'form', label: '表单' }, { value: 'json', label: 'JSON' }]"
        @update:model-value="onEditorMode"
      />

      <form v-show="editorMode === 'form'" class="space-y-4" @submit.prevent="handleSubmit">
        <AppInput
          v-model="form.name"
          label="服务器名称"
          placeholder="输入服务器名称"
        />

        <div class="grid gap-1.5">
          <Label>类型</Label>
          <SegmentedPills
            :model-value="form.type"
            layout-id="mcp-type-pill"
            full
            dense
            :items="[{ value: 'stdio', label: 'Stdio' }, { value: 'http', label: 'HTTP' }]"
            @update:model-value="onType"
          >
            <template #default="{ item }">
              <Terminal v-if="item.value === 'stdio'" class="size-3.5" />
              <Globe v-else class="size-3.5" />
              {{ item.label }}
            </template>
          </SegmentedPills>
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
          <div class="grid gap-1.5">
            <Label>Headers (KEY=VALUE，可选)</Label>
            <Textarea
              v-model="form.headers"
              class="min-h-24 font-mono text-xs"
              placeholder="Authorization=Bearer xxx&#10;X-API-Key=xxx"
            />
          </div>
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
              :spacing="2"
              class="flex w-full flex-wrap"
              @update:model-value="onPlatforms"
            >
              <ToggleGroupItem value="claude" class="flex-1">
                <BrandIcon provider="claude" />
                Claude
                <Check v-if="form.platforms.claude" />
              </ToggleGroupItem>
              <ToggleGroupItem value="codex" class="flex-1">
                <BrandIcon provider="codex" />
                Codex
                <Check v-if="form.platforms.codex" />
              </ToggleGroupItem>
              <ToggleGroupItem value="gemini" class="flex-1">
                <BrandIcon provider="gemini" />
                Gemini
                <Check v-if="form.platforms.gemini" />
              </ToggleGroupItem>
              <ToggleGroupItem value="opencode" class="flex-1">
                <BrandIcon provider="opencode" />
                OpenCode
                <Check v-if="form.platforms.opencode" />
              </ToggleGroupItem>
              <ToggleGroupItem value="grok" class="flex-1">
                <BrandIcon provider="grok" />
                Grok
                <Check v-if="form.platforms.grok" />
              </ToggleGroupItem>
            </ToggleGroup>
          </div>
        </div>
      </form>

      <div v-if="editorMode === 'json'" class="grid gap-1.5">
        <Label>JSON</Label>
        <CodeEditor
          v-model="jsonText"
          language="json"
          class="min-h-80"
          max-height="24rem"
          placeholder='{"name":"filesystem","type":"stdio","command":"npx","args":["-y","@modelcontextprotocol/server-filesystem"]}'
        />
        <p class="text-xs text-muted-foreground">
          支持完整对象，或 Claude 的 <span class="font-mono">mcpServers</span> 格式。HTTP 可写 headers。
        </p>
      </div>
    </div>

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
import { Check, Globe, Terminal } from '@lucide/vue'
import type { MCPServer } from '@/types'
import { useMcpStore } from '@/stores/mcpStore'
import { useToast } from '@/composables/useToast'
import { errorMessage } from '@/lib/configUrl'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import CodeEditor from '@/components/common/CodeEditor.vue'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'

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
const editorMode = ref<'form' | 'json'>('form')
const jsonText = ref('')

const defaultForm = () => ({
  name: '',
  type: 'stdio' as 'stdio' | 'http',
  command: '',
  args: '',
  env: '',
  url: '',
  headers: '',
  website: '',
  tips: '',
  platforms: {
    claude: true,
    codex: false,
    gemini: false,
    opencode: false,
    grok: false,
  }
})

const form = ref(defaultForm())

const selectedPlatformKeys = computed(() => {
  const keys: string[] = []
  if (form.value.platforms.claude) keys.push('claude')
  if (form.value.platforms.codex) keys.push('codex')
  if (form.value.platforms.gemini) keys.push('gemini')
  if (form.value.platforms.opencode) keys.push('opencode')
  if (form.value.platforms.grok) keys.push('grok')
  return keys
})

function onType(value: string) {
  if (value === 'stdio' || value === 'http') {
    form.value.type = value
  }
}

function onPlatforms(value: unknown) {
  const keys = Array.isArray(value) ? value : []
  form.value.platforms.claude = keys.includes('claude')
  form.value.platforms.codex = keys.includes('codex')
  form.value.platforms.gemini = keys.includes('gemini')
  form.value.platforms.opencode = keys.includes('opencode')
  form.value.platforms.grok = keys.includes('grok')
}

function onEditorMode(value: string) {
  if (value !== 'form' && value !== 'json') return
  if (value === editorMode.value) return
  if (value === 'json') {
    jsonText.value = serializeJson()
  } else {
    const err = applyJsonToForm(jsonText.value)
    if (err) {
      toast.error(err)
      return
    }
  }
  editorMode.value = value
}

function parseKv(text: string): Record<string, string> {
  const out: Record<string, string> = {}
  if (!text.trim()) return out
  for (const line of text.split('\n')) {
    const idx = line.indexOf('=')
    if (idx <= 0) continue
    const key = line.slice(0, idx).trim()
    if (!key) continue
    out[key] = line.slice(idx + 1).trim()
  }
  return out
}

function formatKv(map?: Record<string, string>): string {
  if (!map) return ''
  return Object.entries(map).map(([k, v]) => `${k}=${v}`).join('\n')
}

function platformsFromForm(): string[] {
  const enablePlatform: string[] = []
  if (form.value.platforms.claude) enablePlatform.push('claude-code')
  if (form.value.platforms.codex) enablePlatform.push('codex')
  if (form.value.platforms.gemini) enablePlatform.push('gemini')
  if (form.value.platforms.opencode) enablePlatform.push('opencode')
  if (form.value.platforms.grok) enablePlatform.push('grok')
  return enablePlatform
}

function applyPlatforms(list?: string[]) {
  const platforms = list || []
  form.value.platforms.claude = platforms.includes('claude-code')
  form.value.platforms.codex = platforms.includes('codex')
  form.value.platforms.gemini = platforms.includes('gemini')
  form.value.platforms.opencode = platforms.includes('opencode')
  form.value.platforms.grok = platforms.includes('grok')
}

function fillFormFromServer(server: MCPServer) {
  form.value.name = server.name || ''
  form.value.type = (server.type === 'http' || server.type === 'sse' ? 'http' : 'stdio')
  form.value.command = server.command || ''
  form.value.args = (server.args || []).join('\n')
  form.value.env = formatKv(server.env)
  form.value.url = server.url || ''
  form.value.headers = formatKv(server.headers)
  form.value.website = server.website || ''
  form.value.tips = server.tips || ''
  applyPlatforms(server.enable_platform)
}

function serverFromForm(): MCPServer | string {
  const name = form.value.name.trim()
  if (!name) return '请输入服务器名称'

  const exists = mcpStore.servers.some(
    (s, i) => s.name === name && i !== props.editIndex
  )
  if (exists) return '服务器名称已存在'

  if (form.value.type === 'http' && !form.value.url.trim()) return '请输入 URL'
  if (form.value.type === 'stdio' && !form.value.command.trim()) return '请输入 Command'

  const args = form.value.args.trim()
    ? form.value.args.split('\n').map(s => s.trim()).filter(s => s)
    : []

  return {
    name,
    type: form.value.type,
    command: form.value.type === 'stdio' ? form.value.command.trim() : undefined,
    args: form.value.type === 'stdio' ? args : undefined,
    env: form.value.type === 'stdio' ? parseKv(form.value.env) : undefined,
    url: form.value.type === 'http' ? form.value.url.trim() : undefined,
    headers: form.value.type === 'http' ? parseKv(form.value.headers) : undefined,
    website: form.value.website.trim() || undefined,
    tips: form.value.tips.trim() || undefined,
    enable_platform: platformsFromForm(),
    enabled_in_claude: false,
    enabled_in_codex: false,
    enabled_in_gemini: false,
    missing_placeholders: []
  }
}

function serializeJson(): string {
  const built = serverFromForm()
  const doc = typeof built === 'string'
    ? {
        name: form.value.name.trim() || undefined,
        type: form.value.type,
        command: form.value.command.trim() || undefined,
        args: form.value.args.trim() ? form.value.args.split('\n').map(s => s.trim()).filter(Boolean) : undefined,
        env: parseKv(form.value.env),
        url: form.value.url.trim() || undefined,
        headers: parseKv(form.value.headers),
        website: form.value.website.trim() || undefined,
        tips: form.value.tips.trim() || undefined,
        enable_platform: platformsFromForm(),
      }
    : {
        name: built.name,
        type: built.type,
        command: built.command,
        args: built.args,
        env: built.env,
        url: built.url,
        headers: built.headers,
        website: built.website,
        tips: built.tips,
        enable_platform: built.enable_platform,
      }
  return JSON.stringify(doc, null, 2)
}

function applyJsonToForm(text: string): string | null {
  const raw = text.trim()
  if (!raw) return 'JSON 内容为空'
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return 'JSON 无法解析'
  }

  let name = form.value.name
  let entry: Record<string, unknown> | null = null

  if (Array.isArray(parsed) && parsed.length > 0 && parsed[0] && typeof parsed[0] === 'object') {
    entry = parsed[0] as Record<string, unknown>
  } else if (parsed && typeof parsed === 'object') {
    const obj = parsed as Record<string, unknown>
    const servers = obj.mcpServers ?? obj.mcp_servers
    if (servers && typeof servers === 'object' && !Array.isArray(servers)) {
      const map = servers as Record<string, unknown>
      const keys = Object.keys(map)
      if (keys.length === 0) return 'mcpServers 为空'
      const key = keys.includes(name) ? name : keys[0]
      name = key
      entry = (map[key] && typeof map[key] === 'object') ? map[key] as Record<string, unknown> : null
    } else {
      entry = obj
      if (typeof obj.name === 'string' && obj.name.trim()) name = obj.name.trim()
    }
  }

  if (!entry) return '无法识别的 JSON 结构'

  if (typeof entry.name === 'string' && entry.name.trim()) name = entry.name.trim()
  form.value.name = name

  const typeRaw = String(entry.type || '')
  if (typeRaw === 'http' || typeRaw === 'sse' || (!typeRaw && typeof entry.url === 'string' && entry.url)) {
    form.value.type = 'http'
  } else {
    form.value.type = 'stdio'
  }

  form.value.command = typeof entry.command === 'string' ? entry.command : ''
  form.value.url = typeof entry.url === 'string' ? entry.url : ''
  form.value.website = typeof entry.website === 'string' ? entry.website : ''
  form.value.tips = typeof entry.tips === 'string' ? entry.tips : ''

  if (Array.isArray(entry.args)) {
    form.value.args = entry.args.map(v => String(v)).join('\n')
  } else {
    form.value.args = ''
  }

  form.value.env = formatKv(asStringMap(entry.env))
  form.value.headers = formatKv(asStringMap(entry.headers))

  if (Array.isArray(entry.enable_platform)) {
    applyPlatforms(entry.enable_platform.map(v => String(v)))
  }

  return null
}

function asStringMap(value: unknown): Record<string, string> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    if (v == null) continue
    out[k] = String(v)
  }
  return out
}

function syncFormFromProps() {
  editorMode.value = 'form'
  if (props.editServer) {
    fillFormFromServer(props.editServer)
    jsonText.value = serializeJson()
    return
  }
  form.value = defaultForm()
  jsonText.value = serializeJson()
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      syncFormFromProps()
      return
    }
    form.value = defaultForm()
    jsonText.value = ''
    editorMode.value = 'form'
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
  if (editorMode.value === 'json') {
    const err = applyJsonToForm(jsonText.value)
    if (err) {
      toast.error(err)
      return
    }
  }

  const built = serverFromForm()
  if (typeof built === 'string') {
    toast.error(built)
    return
  }

  try {
    if (isEditing.value && props.editIndex !== undefined) {
      await mcpStore.updateServer(props.editIndex, built)
    } else {
      await mcpStore.addServer(built)
    }
    toast.success('MCP 服务器已保存')
    isOpen.value = false
    emit('saved')
  } catch (e: unknown) {
    toast.error('保存失败: ' + errorMessage(e))
  }
}
</script>
