<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑配置' : '新建配置'" size="lg">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div class="grid grid-cols-2 gap-4">
        <div class="col-span-2 sm:col-span-1">
          <AppInput v-model="form.name" label="配置名称" placeholder="输入配置名称" />
        </div>
        <div class="col-span-2 sm:col-span-1">
          <Label>图标</Label>
          <div class="relative mt-1.5">
            <Button type="button" variant="outline" size="icon" class="text-xl" @click="showEmojiPicker = !showEmojiPicker">
              {{ form.icon }}
            </Button>
            <EmojiPicker :show="showEmojiPicker" @close="showEmojiPicker = false" @select="selectIcon" />
          </div>
        </div>
        <div class="col-span-2">
          <AppInput v-model="form.description" label="描述" placeholder="可选的配置描述" />
        </div>
      </div>

      <SegmentedPills
        :model-value="form.provider"
        layout-id="config-provider-pill"
        full
        dense
        :items="providers.map(p => ({ value: p.value, label: p.label }))"
        @update:model-value="onProvider"
      >
        <template #default="{ item }">
          <BrandIcon :provider="item.value" class="size-3.5" />
          {{ item.label }}
        </template>
      </SegmentedPills>

      <div v-if="form.provider === 'claude'" class="space-y-4">
        <AppInput v-model="form.claude.baseUrl" label="Base URL" placeholder="https://api.anthropic.com">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="testLatency(form.claude.baseUrl)">
              <Zap />
            </Button>
          </template>
        </AppInput>
        <div class="grid gap-1.5">
          <Label>上游格式</Label>
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
          <p v-if="form.upstreamFormat" class="text-xs leading-relaxed text-muted-foreground">
            应用时由本地路由做协议转换，Claude Code 会指向本机网关；真实上游仍用上面的地址与密钥，需保持路由运行。
          </p>
        </div>
        <AppInput v-model="form.claude.authToken" label="Auth Token" placeholder="可选" />
        <AppInput v-model="form.claude.model" label="Model" placeholder="claude-sonnet-4-20250514" />
        <AppInput
          v-model="form.claude.apiKey"
          label="API Key"
          :type="showApiKey.claude ? 'text' : 'password'"
          placeholder="sk-ant-..."
        >
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="toggleApiKeyVisibility('claude')">
              <EyeOff v-if="showApiKey.claude" />
              <Eye v-else />
            </Button>
          </template>
        </AppInput>

        <div class="space-y-3 border-t pt-3">
          <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Claude Code 环境变量</p>
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="text-sm font-medium">Attribution Header</div>
              <div class="font-mono text-[11px] text-muted-foreground">CLAUDE_CODE_ATTRIBUTION_HEADER</div>
            </div>
            <ToggleGroup type="single" variant="outline" size="sm" :model-value="triValue(form.claude.attributionHeader)" @update:model-value="v => form.claude.attributionHeader = fromTri(v)">
              <ToggleGroupItem value="unset">不设置</ToggleGroupItem>
              <ToggleGroupItem value="0">0</ToggleGroupItem>
              <ToggleGroupItem value="1">1</ToggleGroupItem>
            </ToggleGroup>
          </div>
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="text-sm font-medium">Disable Nonessential Traffic</div>
              <div class="font-mono text-[11px] text-muted-foreground">CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC</div>
            </div>
            <ToggleGroup type="single" variant="outline" size="sm" :model-value="triValue(form.claude.disableNonessentialTraffic)" @update:model-value="v => form.claude.disableNonessentialTraffic = fromTri(v)">
              <ToggleGroupItem value="unset">不设置</ToggleGroupItem>
              <ToggleGroupItem value="0">0</ToggleGroupItem>
              <ToggleGroupItem value="1">1</ToggleGroupItem>
            </ToggleGroup>
          </div>
        </div>
      </div>

      <div v-if="form.provider === 'codex'" class="space-y-4">
        <AppInput v-model="form.codex.baseUrl" label="Base URL" placeholder="https://api.openai.com/v1">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="testLatency(form.codex.baseUrl)">
              <Zap />
            </Button>
          </template>
        </AppInput>
        <div class="grid gap-1.5">
          <Label>上游格式</Label>
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
          <p v-if="form.upstreamFormat" class="text-xs leading-relaxed text-muted-foreground">
            应用时由本地路由把 Responses 请求转成上游格式，Codex 会指向本机网关（wire_api 保持 responses）；真实上游仍用上面的地址与密钥。
          </p>
        </div>
        <AppInput
          v-model="form.codex.apiKey"
          label="API Key"
          :type="showApiKey.codex ? 'text' : 'password'"
          placeholder="sk-..."
        >
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="toggleApiKeyVisibility('codex')">
              <EyeOff v-if="showApiKey.codex" />
              <Eye v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput v-model="form.codex.model" label="Model" placeholder="gpt-4" />
        <div class="grid gap-1.5">
          <Label>config.toml 模板</Label>
          <Textarea v-model="form.codex.configTemplate" class="min-h-32 font-mono text-xs" placeholder="TOML 配置模板..." />
        </div>
        <div class="grid gap-1.5">
          <Label>auth.json 模板</Label>
          <Textarea v-model="form.codex.authTemplate" class="min-h-24 font-mono text-xs" placeholder="JSON 认证模板..." />
        </div>
      </div>

      <div v-if="form.provider === 'gemini'" class="space-y-4">
        <AppInput v-model="form.gemini.baseUrl" label="Base URL" placeholder="https://generativelanguage.googleapis.com">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="testLatency(form.gemini.baseUrl)">
              <Zap />
            </Button>
          </template>
        </AppInput>
        <AppInput
          v-model="form.gemini.apiKey"
          label="API Key"
          :type="showApiKey.gemini ? 'text' : 'password'"
          placeholder="API Key"
        >
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="toggleApiKeyVisibility('gemini')">
              <EyeOff v-if="showApiKey.gemini" />
              <Eye v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput v-model="form.gemini.model" label="Model" placeholder="gemini-pro" />
        <div class="grid gap-1.5">
          <Label>.env 模板</Label>
          <Textarea v-model="form.gemini.envTemplate" class="min-h-24 font-mono text-xs" placeholder="环境变量模板..." />
        </div>
        <div class="grid gap-1.5">
          <Label>settings.json 模板</Label>
          <Textarea v-model="form.gemini.settingsTemplate" class="min-h-24 font-mono text-xs" placeholder="JSON 设置模板..." />
        </div>
      </div>

      <div v-if="form.provider === 'opencode'" class="space-y-4">
        <div class="rounded-lg border bg-muted/40 p-3">
          <p class="text-xs leading-relaxed text-muted-foreground">
            OpenCode 配置默认写入
            <span class="font-mono">~/.config/opencode/opencode.json</span>，
            并支持 <span class="font-mono">OPENCODE_CONFIG_DIR / OPENCODE_CONFIG</span> 覆盖路径。
            填了 Base URL 时会以 OpenAI 兼容自定义 provider 接入网关。
          </p>
        </div>
        <AppInput v-model="form.opencode.baseUrl" label="Base URL" placeholder="https://your-gateway/v1">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="testLatency(form.opencode.baseUrl)">
              <Zap />
            </Button>
          </template>
        </AppInput>
        <AppInput
          v-model="form.opencode.apiKey"
          label="API Key"
          :type="showApiKey.opencode ? 'text' : 'password'"
          placeholder="可选"
        >
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="toggleApiKeyVisibility('opencode')">
              <EyeOff v-if="showApiKey.opencode" />
              <Eye v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput v-model="form.opencode.model" label="Model" placeholder="anthropic/claude-sonnet-4" />
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <AppInput v-model="form.opencode.configDir" label="OPENCODE_CONFIG_DIR（可选）" placeholder="~/.config/opencode" />
          <AppInput v-model="form.opencode.configPath" label="OPENCODE_CONFIG（可选）" placeholder="~/.config/opencode/opencode.json" />
        </div>
        <div class="grid gap-1.5">
          <Label>opencode.json 模板（可选）</Label>
          <Textarea v-model="form.opencode.configTemplate" class="min-h-32 font-mono text-xs" placeholder="JSON 模板，支持 {{OPENCODE_MODEL}} / {{OPENCODE_BASE_URL}} / {{OPENCODE_API_KEY}} 占位符..." />
        </div>
      </div>

      <div v-if="form.provider === 'grok'" class="space-y-4">
        <div class="rounded-lg border bg-muted/40 p-3">
          <p class="text-xs leading-relaxed text-muted-foreground">
            Grok 配置写入
            <span class="font-mono">~/.grok/config.toml</span>
            （保留已有 MCP / Skills 段）。CLI 读取
            <span class="font-mono">XAI_API_KEY</span>
            和模型的 <span class="font-mono">api_key</span>。
          </p>
        </div>
        <AppInput v-model="form.grok.baseUrl" label="Base URL" placeholder="https://api.x.ai/v1">
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="testLatency(form.grok.baseUrl || 'https://api.x.ai/v1')">
              <Zap />
            </Button>
          </template>
        </AppInput>
        <AppInput
          v-model="form.grok.apiKey"
          label="API Key"
          :type="showApiKey.grok ? 'text' : 'password'"
          placeholder="xai-..."
        >
          <template #suffix>
            <Button type="button" variant="ghost" size="icon-xs" @click="toggleApiKeyVisibility('grok')">
              <EyeOff v-if="showApiKey.grok" />
              <Eye v-else />
            </Button>
          </template>
        </AppInput>
        <AppInput v-model="form.grok.model" label="Model" placeholder="grok-4.6" />
        <div class="grid gap-1.5">
          <Label>API Backend</Label>
          <SegmentedPills
            :model-value="form.grok.apiBackend"
            layout-id="grok-backend-pill"
            full
            dense
            :items="grokBackends"
            @update:model-value="v => { if (v === 'responses' || v === 'chat_completions' || v === 'messages') form.grok.apiBackend = v }"
          />
        </div>
        <AppInput v-model="form.grok.homeDir" label="GROK_HOME（可选）" placeholder="~/.grok" />
        <div class="grid gap-1.5">
          <Label>config.toml 模板（可选）</Label>
          <Textarea v-model="form.grok.configTemplate" class="min-h-32 font-mono text-xs" placeholder="留空则按上面的字段生成" />
        </div>
      </div>
    </form>

    <template #footer>
      <Button type="button" variant="secondary" @click="isOpen = false">取消</Button>
      <Button type="button" @click="handleSubmit">{{ isEditing ? '保存' : '创建' }}</Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Eye, EyeOff, Zap } from '@lucide/vue'
import type { EnvConfig, Provider, UpstreamFormat } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import EmojiPicker from '@/components/common/EmojiPicker.vue'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'

interface Props {
  modelValue: boolean
  editConfig?: EnvConfig | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const configStore = useConfigStore()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isEditing = computed(() => !!props.editConfig)
const showEmojiPicker = ref(false)
type ApiKeyProvider = 'claude' | 'codex' | 'gemini' | 'opencode' | 'grok'
const showApiKey = ref<Record<ApiKeyProvider, boolean>>({
  claude: false,
  codex: false,
  gemini: false,
  opencode: false,
  grok: false,
})

function toggleApiKeyVisibility(provider: ApiKeyProvider) {
  showApiKey.value[provider] = !showApiKey.value[provider]
}

function resetApiKeyVisibility() {
  showApiKey.value.claude = false
  showApiKey.value.codex = false
  showApiKey.value.gemini = false
  showApiKey.value.opencode = false
  showApiKey.value.grok = false
}

function selectIcon(emoji: string) {
  form.value.icon = emoji
}

const providers: { value: Provider; label: string }[] = [
  { value: 'claude', label: 'Claude' },
  { value: 'codex', label: 'Codex' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'opencode', label: 'OpenCode' },
  { value: 'grok', label: 'Grok' },
]
const grokBackends = [
  { value: 'responses', label: 'Responses' },
  { value: 'chat_completions', label: 'Chat' },
  { value: 'messages', label: 'Messages' },
]

function onProvider(value: unknown) {
  if (value === 'claude' || value === 'codex' || value === 'gemini' || value === 'opencode' || value === 'grok') {
    form.value.provider = value
    form.value.upstreamFormat = ''
  }
}

// 上游格式选项：按 provider 给出原生格式与可转换格式（与后端 needsRouting 支持的转换矩阵一致）
const upstreamOptions = computed(() => {
  if (form.value.provider === 'claude') {
    return [
      { value: 'native', label: 'Anthropic Messages（原生）' },
      { value: 'chat_completions', label: 'Chat Completions（需开启路由）' },
    ]
  }
  if (form.value.provider === 'codex') {
    return [
      { value: 'native', label: 'Responses（原生）' },
      { value: 'chat_completions', label: 'Chat Completions（需开启路由）' },
      { value: 'anthropic_messages', label: 'Anthropic Messages（需开启路由）' },
    ]
  }
  return []
})

// Select 不接受空字符串值，用 native 哨兵值桥接
const upstreamSelect = computed({
  get: () => form.value.upstreamFormat || 'native',
  set: (value: string) => {
    form.value.upstreamFormat = (value === 'native' ? '' : value) as UpstreamFormat
  },
})

function triValue(value: string) {
  return value === '' ? 'unset' : value
}

function fromTri(value: unknown) {
  if (value === '0' || value === '1') return value
  return ''
}

const defaultForm = () => ({
  name: '',
  description: '',
  icon: '📦',
  provider: 'claude' as Provider,
  upstreamFormat: '' as UpstreamFormat,
  claude: {
    baseUrl: '',
    authToken: '',
    model: '',
    apiKey: '',
    attributionHeader: '',
    disableNonessentialTraffic: ''
  },
  codex: {
    baseUrl: '',
    apiKey: '',
    model: '',
    configTemplate: `model_provider = "duckcoding"
model = "{{model}}"
model_reasoning_effort = "high"
network_access = "enabled"
disable_response_storage = true

[model_providers.duckcoding]
name = "duckcoding"
base_url = "{{base_url}}"
wire_api = "responses"
requires_openai_auth = true`,
    authTemplate: `{
  "OPENAI_API_KEY": "{{OPENAI_API_KEY}}"
}`
  },
  gemini: {
    baseUrl: '',
    apiKey: '',
    model: '',
    envTemplate: `GOOGLE_GEMINI_BASE_URL={{GOOGLE_GEMINI_BASE_URL}}
GEMINI_API_KEY={{GEMINI_API_KEY}}
GEMINI_MODEL={{GEMINI_MODEL}}`,
    settingsTemplate: `{
  "ide": {
    "enabled": true
  },
  "security": {
    "auth": {
      "selectedType": "gemini-api-key"
    }
  }
}`
  },
  opencode: {
    baseUrl: '',
    apiKey: '',
    model: '',
    configDir: '',
    configPath: '',
    configTemplate: ''
  },
  grok: {
    baseUrl: 'https://api.x.ai/v1',
    apiKey: '',
    model: 'grok-4.6',
    apiBackend: 'responses',
    homeDir: '',
    configTemplate: '',
  }
})

const form = ref(defaultForm())
const originalName = ref('')

watch(() => props.editConfig, (config) => {
  if (config) {
    originalName.value = config.name
    form.value.name = config.name
    form.value.description = config.description || ''
    form.value.icon = config.icon || '📦'
    form.value.provider = config.provider
    form.value.upstreamFormat = (config.upstream_format
      && (config.provider === 'claude' || config.provider === 'codex')
      ? config.upstream_format
      : '') as UpstreamFormat

    if (config.provider === 'claude') {
      form.value.claude.baseUrl = config.variables.ANTHROPIC_BASE_URL || ''
      form.value.claude.authToken = config.variables.ANTHROPIC_AUTH_TOKEN || ''
      form.value.claude.model = config.variables.ANTHROPIC_MODEL || ''
      form.value.claude.apiKey = config.variables.ANTHROPIC_API_KEY || ''
      form.value.claude.attributionHeader = config.attribution_header || ''
      form.value.claude.disableNonessentialTraffic = config.disable_nonessential_traffic || ''
    } else if (config.provider === 'codex') {
      form.value.codex.baseUrl = config.variables.base_url || ''
      form.value.codex.apiKey = config.variables.OPENAI_API_KEY || ''
      form.value.codex.model = config.variables.model || ''
      form.value.codex.configTemplate = config.templates?.['config.toml'] || form.value.codex.configTemplate
      form.value.codex.authTemplate = config.templates?.['auth.json'] || form.value.codex.authTemplate
    } else if (config.provider === 'gemini') {
      form.value.gemini.baseUrl = config.variables.GOOGLE_GEMINI_BASE_URL || ''
      form.value.gemini.apiKey = config.variables.GEMINI_API_KEY || ''
      form.value.gemini.model = config.variables.GEMINI_MODEL || ''
      form.value.gemini.envTemplate = config.templates?.['.env'] || form.value.gemini.envTemplate
      form.value.gemini.settingsTemplate = config.templates?.['settings.json'] || form.value.gemini.settingsTemplate
    } else if (config.provider === 'opencode') {
      form.value.opencode.baseUrl = config.variables.OPENCODE_BASE_URL || ''
      form.value.opencode.apiKey = config.variables.OPENCODE_API_KEY || ''
      form.value.opencode.model = config.variables.OPENCODE_MODEL || ''
      form.value.opencode.configDir = config.variables.OPENCODE_CONFIG_DIR || ''
      form.value.opencode.configPath = config.variables.OPENCODE_CONFIG || ''
      form.value.opencode.configTemplate = config.templates?.['opencode.json'] || ''
    } else if (config.provider === 'grok') {
      form.value.grok.baseUrl = config.variables.XAI_BASE_URL || 'https://api.x.ai/v1'
      form.value.grok.apiKey = config.variables.XAI_API_KEY || ''
      form.value.grok.model = config.variables.XAI_MODEL || 'grok-4.6'
      form.value.grok.apiBackend = config.variables.XAI_API_BACKEND || 'responses'
      form.value.grok.homeDir = config.variables.GROK_HOME || ''
      form.value.grok.configTemplate = config.templates?.['config.toml'] || ''
    }
  } else {
    form.value = defaultForm()
    originalName.value = ''
  }
}, { immediate: true })

watch(isOpen, (open) => {
  if (!open) {
    form.value = defaultForm()
    originalName.value = ''
    resetApiKeyVisibility()
  }
})

async function testLatency(url: string) {
  if (!url) {
    toast.error('Base URL 为空')
    return
  }
  try {
    const ms = await configStore.testLatency(url)
    if (ms > 1000) {
      toast.error(`延迟: ${ms}ms`)
    } else if (ms > 300) {
      toast.info(`延迟: ${ms}ms`)
    } else {
      toast.success(`延迟: ${ms}ms`)
    }
  } catch {
    toast.error('测速失败')
  }
}

async function handleSubmit() {
  if (!form.value.name.trim()) {
    toast.error('请输入配置名称')
    return
  }

  const exists = configStore.environments.some(
    c => c.name === form.value.name && c.name !== originalName.value
  )
  if (exists) {
    toast.error('配置名称已存在')
    return
  }

  let variables: Record<string, string> = {}
  let templates: Record<string, string> = {}

  if (form.value.provider === 'claude') {
    variables = {
      ANTHROPIC_BASE_URL: form.value.claude.baseUrl,
      ANTHROPIC_AUTH_TOKEN: form.value.claude.authToken,
      ANTHROPIC_MODEL: form.value.claude.model,
      ANTHROPIC_API_KEY: form.value.claude.apiKey
    }
  } else if (form.value.provider === 'codex') {
    variables = {
      base_url: form.value.codex.baseUrl,
      OPENAI_API_KEY: form.value.codex.apiKey,
      model: form.value.codex.model
    }
    if (form.value.codex.configTemplate) {
      templates['config.toml'] = form.value.codex.configTemplate
    }
    if (form.value.codex.authTemplate) {
      templates['auth.json'] = form.value.codex.authTemplate
    }
  } else if (form.value.provider === 'gemini') {
    variables = {
      GOOGLE_GEMINI_BASE_URL: form.value.gemini.baseUrl,
      GEMINI_API_KEY: form.value.gemini.apiKey,
      GEMINI_MODEL: form.value.gemini.model
    }
    if (form.value.gemini.envTemplate) {
      templates['.env'] = form.value.gemini.envTemplate
    }
    if (form.value.gemini.settingsTemplate) {
      templates['settings.json'] = form.value.gemini.settingsTemplate
    }
  } else if (form.value.provider === 'opencode') {
    variables = {
      OPENCODE_BASE_URL: form.value.opencode.baseUrl,
      OPENCODE_API_KEY: form.value.opencode.apiKey,
      OPENCODE_MODEL: form.value.opencode.model,
      OPENCODE_CONFIG_DIR: form.value.opencode.configDir,
      OPENCODE_CONFIG: form.value.opencode.configPath
    }
    if (form.value.opencode.configTemplate) {
      templates['opencode.json'] = form.value.opencode.configTemplate
    }
  } else if (form.value.provider === 'grok') {
    variables = {
      XAI_BASE_URL: form.value.grok.baseUrl || 'https://api.x.ai/v1',
      XAI_API_KEY: form.value.grok.apiKey,
      XAI_MODEL: form.value.grok.model || 'grok-4.6',
      XAI_API_BACKEND: form.value.grok.apiBackend || 'responses',
      GROK_HOME: form.value.grok.homeDir,
    }
    if (form.value.grok.configTemplate) {
      templates['config.toml'] = form.value.grok.configTemplate
    }
  }

  const configData: EnvConfig = {
    name: form.value.name.trim(),
    description: form.value.description.trim(),
    provider: form.value.provider,
    variables,
    templates,
    icon: form.value.icon,
    upstream_format: form.value.provider === 'claude' || form.value.provider === 'codex'
      ? form.value.upstreamFormat
      : '',
    attribution_header: form.value.provider === 'claude' ? form.value.claude.attributionHeader : '',
    disable_nonessential_traffic: form.value.provider === 'claude' ? form.value.claude.disableNonessentialTraffic : ''
  }

  try {
    if (isEditing.value) {
      await configStore.updateEnv(originalName.value, configData)
    } else {
      await configStore.addEnv(configData)
    }
    toast.success('配置已保存')
    isOpen.value = false
    emit('saved')
  } catch (e: any) {
    toast.error('保存失败: ' + e.message)
  }
}
</script>
