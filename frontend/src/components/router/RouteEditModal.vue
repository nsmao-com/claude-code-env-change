<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑路由' : '添加路由'" size="md">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <p v-if="isAutoRoute" class="rounded-lg border bg-muted/40 px-3 py-2 text-xs leading-relaxed text-muted-foreground">
        这是左上角应用路由自动生成的条目。日常改配置里的「上游格式」并开关对应模型商即可；这里只建议改地址、密钥或模型映射。
      </p>

      <div v-if="!isEditing" class="space-y-2">
        <FieldLabel label="快捷场景" hint="先选场景会填好「给哪个 CLI」和「上游格式」，再补地址和密钥。" />
        <div class="space-y-2">
          <Button
            v-for="preset in presets"
            :key="preset.label"
            type="button"
            variant="outline"
            class="h-auto w-full flex-col items-start whitespace-normal py-2.5"
            @click="applyPreset(preset)"
          >
            <span class="flex items-center gap-2">
              <BrandIcon :provider="preset.client" class="size-3.5" />
              <span class="text-sm font-medium">{{ preset.label }}</span>
            </span>
            <span class="text-xs font-normal text-muted-foreground">{{ preset.hint }}</span>
          </Button>
        </div>
      </div>

      <AppInput
        v-model="form.name"
        label="路由名称"
        placeholder="如 glm-claude（用在本机 URL 路径）"
        :tooltip="tips.name"
        :disabled="isAutoRoute"
      />

      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div class="grid gap-1.5">
          <FieldLabel label="给哪个 CLI 用" :hint="tips.client" />
          <Select :model-value="form.client" :disabled="isAutoRoute" @update:model-value="onClient">
            <SelectTrigger class="w-full">
              <SelectValue placeholder="选择 CLI" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="item in clients" :key="item.value" :value="item.value">
                {{ item.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="grid gap-1.5">
          <FieldLabel label="上游格式" :hint="tips.upstream" />
          <Select v-model="upstreamSelect">
            <SelectTrigger class="w-full">
              <SelectValue placeholder="选择上游 API 格式" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="opt in upstreamOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <AppInput v-model="form.base_url" label="上游 Base URL" placeholder="https://api.example.com" :tooltip="tips.baseUrl" />
      <AppInput v-model="form.api_key" label="上游 API Key" placeholder="sk-..." type="password" :tooltip="tips.apiKey" />
      <AppInput v-model="form.default_model" label="默认模型（可选）" placeholder="未命中映射时使用" :tooltip="tips.model" />

      <div class="space-y-3 border-t pt-3">
        <Button type="button" variant="ghost" size="sm" @click="showAdvanced = !showAdvanced">
          {{ showAdvanced ? '收起高级选项' : '高级选项' }}
        </Button>
        <div v-if="showAdvanced" class="space-y-3">
          <div>
            <div class="mb-1.5 flex items-center justify-between">
              <FieldLabel label="模型映射" :hint="tips.mapping" />
              <Button type="button" variant="link" size="sm" @click="addMappingRow">添加一行</Button>
            </div>
            <div class="space-y-2">
              <div v-for="(row, i) in mappingRows" :key="i" class="flex items-center">
                <Input v-model="row.source" class="flex-1 font-mono text-xs" placeholder="源模型，如 claude-sonnet-4 或 *" />
                <span class="shrink-0 px-2 text-xs text-muted-foreground">→</span>
                <Input v-model="row.target" class="flex-1 font-mono text-xs" placeholder="上游模型" />
                <AppTooltip content="删除这一行">
                  <Button type="button" variant="ghost" size="icon-sm" @click="removeMappingRow(i)">
                    <X />
                  </Button>
                </AppTooltip>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="flex items-center">
        <Switch :checked="form.enabled" @update:checked="onEnabledChange" />
        <Label class="ml-2 cursor-pointer">启用此路由</Label>
      </div>

      <div v-if="form.name.trim()" class="space-y-1.5 rounded-lg border bg-muted/40 p-3 text-xs leading-relaxed text-muted-foreground">
        <p class="font-medium text-foreground">怎么接</p>
        <p v-if="isProviderRoute">打开左上角「{{ clientLabel }}」路由开关后，会自动把该 CLI 指到 <span class="font-mono text-foreground">{{ routeUrl }}</span>。</p>
        <p v-else>自定义路由不会自动改 CLI。把对应工具的 Base URL 指到 <span class="font-mono text-foreground">{{ accessUrl }}</span> 即可。</p>
      </div>
    </form>

    <template #footer>
      <Button type="button" variant="secondary" @click="isOpen = false">取消</Button>
      <Button type="button" @click="handleSubmit">{{ isEditing ? '保存' : '添加' }}</Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { X } from '@lucide/vue'
import type { APIRoute, APIFormat, Provider } from '@/types'
import { useRouterStore } from '@/stores/routerStore'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import FieldLabel from '@/components/common/FieldLabel.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

const AUTO_PROVIDERS: Provider[] = ['claude', 'codex', 'gemini', 'opencode', 'grok']

interface Props {
  modelValue: boolean
  editRoute?: APIRoute | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const routerStore = useRouterStore()
const configStore = useConfigStore()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
})

const isEditing = computed(() => props.editRoute != null)
const showAdvanced = ref(false)

const clients: { value: Provider; label: string }[] = [
  { value: 'claude', label: 'Claude Code' },
  { value: 'codex', label: 'Codex' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'opencode', label: 'OpenCode' },
  { value: 'grok', label: 'Grok' },
]

const tips = {
  name: '会出现在本机网关路径里，例如 http://127.0.0.1:端口/名称。应用路由自动生成的条目名称就是模型商 id。',
  client: '谁来连本机网关。Claude Code 说 Anthropic；Codex / OpenCode / Grok 说 OpenAI 系。',
  upstream: '上游实际返回的协议。和配置里高级选项的「上游格式」同一套：原生可透传，其它格式由网关转换。',
  baseUrl: '真实上游地址，一般填到域名或 /v1 之前。',
  apiKey: '转发给上游时使用的密钥。CLI 里可以随便填占位。',
  model: '请求没带模型名，或映射没命中时使用。',
  mapping: '把 CLI 发出的模型名换成上游认识的名字。* 为兜底。',
}

interface Preset {
  label: string
  hint: string
  client: Provider
  upstream: string
}

const presets: Preset[] = [
  {
    label: 'Claude ← Chat Completions',
    hint: '上游是 OpenAI Chat。打开 Claude 路由开关后即可转换。',
    client: 'claude',
    upstream: 'chat_completions',
  },
  {
    label: 'Codex ← Chat Completions',
    hint: '上游只有 Chat，没有 Responses。打开 Codex 路由开关后转换。',
    client: 'codex',
    upstream: 'chat_completions',
  },
  {
    label: 'Codex ← Anthropic Messages',
    hint: '上游是 Anthropic 协议，给 Codex 用。',
    client: 'codex',
    upstream: 'anthropic_messages',
  },
  {
    label: '同协议透传',
    hint: '上游就是该 CLI 的原生格式，只做转发或模型改名。',
    client: 'claude',
    upstream: 'native',
  },
]

function clientFromFilter(): Provider {
  const filter = configStore.currentFilter
  if (filter === 'codex' || filter === 'gemini' || filter === 'opencode' || filter === 'grok') return filter
  return 'claude'
}

const defaultForm = () => ({
  name: '',
  client: clientFromFilter(),
  upstream: 'native',
  base_url: '',
  api_key: '',
  default_model: '',
  enabled: true,
})

const form = ref(defaultForm())
const mappingRows = ref<{ source: string; target: string }[]>([{ source: '', target: '' }])

const isAutoRoute = computed(() => {
  const name = (props.editRoute?.name || '').toLowerCase()
  const desc = props.editRoute?.description || ''
  return AUTO_PROVIDERS.includes(name as Provider) || desc.includes('应用路由')
})

const isProviderRoute = computed(() => AUTO_PROVIDERS.includes(form.value.name.trim().toLowerCase() as Provider))
const clientLabel = computed(() => clients.find(item => item.value === form.value.client)?.label || form.value.client)

const upstreamOptions = computed(() => {
  const extra: Record<Provider, { value: string; label: string }[]> = {
    claude: [
      { value: 'native', label: 'Anthropic Messages（原生）' },
      { value: 'chat_completions', label: 'Chat Completions（需开启路由）' },
    ],
    codex: [
      { value: 'native', label: 'Responses（原生）' },
      { value: 'chat_completions', label: 'Chat Completions（需开启路由）' },
      { value: 'anthropic_messages', label: 'Anthropic Messages（需开启路由）' },
    ],
    gemini: [
      { value: 'native', label: 'Gemini（原生）' },
      { value: 'chat_completions', label: 'Chat Completions（需开启路由）' },
      { value: 'anthropic_messages', label: 'Anthropic Messages（需开启路由）' },
    ],
    opencode: [
      { value: 'native', label: 'Chat Completions（原生）' },
      { value: 'anthropic_messages', label: 'Anthropic Messages（需开启路由）' },
      { value: 'responses', label: 'Responses（需开启路由）' },
    ],
    grok: [
      { value: 'native', label: 'Responses（原生）' },
      { value: 'chat_completions', label: 'Chat Completions（需开启路由）' },
      { value: 'anthropic_messages', label: 'Anthropic Messages（需开启路由）' },
    ],
  }
  return extra[form.value.client] || extra.claude
})

const upstreamSelect = computed({
  get: () => form.value.upstream || 'native',
  set: (value: string) => {
    form.value.upstream = value || 'native'
  },
})

function sourceOf(client: Provider): APIFormat {
  return client === 'claude' ? 'anthropic' : 'openai'
}

function targetOf(client: Provider, format: string): APIFormat {
  if (format === 'anthropic_messages') return 'anthropic'
  if (format === 'chat_completions') return 'openai'
  if (format === 'responses') return 'responses'
  if (client === 'claude') return 'anthropic'
  if (client === 'opencode' || client === 'gemini') return 'openai'
  return 'responses'
}

function clientFromRoute(route: APIRoute): Provider {
  const name = (route.name || '').toLowerCase()
  if (AUTO_PROVIDERS.includes(name as Provider)) return name as Provider
  return route.source_format === 'anthropic' ? 'claude' : 'codex'
}

function upstreamFromRoute(route: APIRoute, client: Provider): string {
  const target = route.target_format
  const source = route.source_format
  if (target === 'responses') {
    return client === 'codex' || client === 'grok' ? 'native' : 'responses'
  }
  if (target === 'anthropic') {
    return source === 'anthropic' ? 'native' : 'anthropic_messages'
  }
  if (client === 'opencode' || client === 'gemini') return 'native'
  return 'chat_completions'
}

const routeUrl = computed(() => `http://127.0.0.1:${routerStore.config.port || 8790}/${form.value.name.trim()}`)
const accessUrl = computed(() => {
  const base = routeUrl.value
  return form.value.client === 'claude' ? base : `${base}/v1`
})

function addMappingRow() {
  mappingRows.value.push({ source: '', target: '' })
}

function removeMappingRow(index: number) {
  mappingRows.value.splice(index, 1)
  if (mappingRows.value.length === 0) {
    mappingRows.value.push({ source: '', target: '' })
  }
}

function mappingFromRecord(mapping?: Record<string, string>) {
  const entries = Object.entries(mapping || {})
  if (entries.length === 0) return [{ source: '', target: '' }]
  return entries.map(([source, target]) => ({ source, target }))
}

function mappingToRecord(): Record<string, string> | undefined {
  const mapping: Record<string, string> = {}
  for (const row of mappingRows.value) {
    const key = row.source.trim()
    const value = row.target.trim()
    if (key && value) mapping[key] = value
  }
  return Object.keys(mapping).length > 0 ? mapping : undefined
}

watch(
  () => props.modelValue,
  (open) => {
    if (!open) {
      form.value = defaultForm()
      mappingRows.value = [{ source: '', target: '' }]
      showAdvanced.value = false
      return
    }
    if (props.editRoute) {
      const route = props.editRoute
      const client = clientFromRoute(route)
      form.value = {
        name: route.name || '',
        client,
        upstream: upstreamFromRoute(route, client),
        base_url: route.base_url || '',
        api_key: route.api_key || '',
        default_model: route.default_model || '',
        enabled: route.enabled !== false,
      }
      mappingRows.value = mappingFromRecord(route.model_mapping)
      showAdvanced.value = Object.keys(route.model_mapping || {}).length > 0
    } else {
      form.value = defaultForm()
      mappingRows.value = [{ source: '', target: '' }]
      showAdvanced.value = false
    }
  },
)

function applyPreset(preset: Preset) {
  form.value.client = preset.client
  form.value.upstream = preset.upstream
  if (!form.value.name.trim()) form.value.name = preset.client
}

function onClient(value: unknown) {
  if (value === 'claude' || value === 'codex' || value === 'gemini' || value === 'opencode' || value === 'grok') {
    form.value.client = value
    const allowed = upstreamOptions.value.some(opt => opt.value === form.value.upstream)
    if (!allowed) form.value.upstream = 'native'
  }
}

function onEnabledChange(checked: boolean) {
  form.value.enabled = checked
  toast.success(checked ? '已启用此路由' : '已停用此路由')
}

async function handleSubmit() {
  const name = form.value.name.trim()
  if (!name) {
    toast.error('请输入路由名称')
    return
  }
  if (!/^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/.test(name)) {
    toast.error('路由名称仅允许字母/数字/连字符/下划线')
    return
  }
  if (!form.value.base_url.trim()) {
    toast.error('请输入上游 Base URL')
    return
  }

  const duplicate = routerStore.config.routes.some(
    route => route.name.toLowerCase() === name.toLowerCase() && route.name !== props.editRoute?.name,
  )
  if (duplicate) {
    toast.error('路由名称已存在')
    return
  }

  const route: APIRoute = {
    name,
    description: props.editRoute?.description,
    source_format: sourceOf(form.value.client),
    target_format: targetOf(form.value.client, form.value.upstream),
    base_url: form.value.base_url.trim(),
    api_key: form.value.api_key.trim() || undefined,
    default_model: form.value.default_model.trim() || undefined,
    model_mapping: mappingToRecord(),
    enabled: form.value.enabled,
  }

  const routes = [...routerStore.config.routes]
  const existIndex = props.editRoute ? routes.findIndex(item => item.name === props.editRoute!.name) : -1
  if (existIndex >= 0) routes[existIndex] = route
  else routes.push(route)

  try {
    await routerStore.saveConfig({ ...routerStore.config, routes })
    toast.success(isEditing.value ? '路由已保存' : '路由已添加')
    isOpen.value = false
    emit('saved')
  } catch (e: unknown) {
    toast.error('保存失败: ' + (e instanceof Error ? e.message : String(e)))
  }
}
</script>
