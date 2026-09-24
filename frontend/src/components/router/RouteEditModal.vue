<template>
  <AppModal v-model="isOpen" :title="isEditing ? t('router.edit.titleEdit') : t('router.edit.titleAdd')" size="md">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <p v-if="isAutoRoute" class="rounded-lg border bg-muted/40 px-3 py-2 text-xs leading-relaxed text-muted-foreground">
        {{ t('router.edit.autoNote') }}
      </p>

      <div v-if="!isEditing" class="space-y-2">
        <FieldLabel :label="t('router.edit.presets')" :hint="t('router.edit.presetsHint')" />
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
        :label="t('router.edit.name')"
        :placeholder="t('router.edit.namePlaceholder')"
        :tooltip="tips.name"
        :disabled="isAutoRoute"
      />

      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div class="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-1.5">
          <FieldLabel :label="t('router.edit.client')" :hint="tips.client" />
          <Select :model-value="form.client" :disabled="isAutoRoute" @update:model-value="onClient">
            <SelectTrigger class="w-full">
              <SelectValue :placeholder="t('router.edit.clientPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="item in clients" :key="item.value" :value="item.value">
                {{ item.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-1.5">
          <FieldLabel :label="t('router.edit.upstream')" :hint="tips.upstream" />
          <Select v-model="upstreamSelect">
            <SelectTrigger class="w-full min-w-0 overflow-hidden">
              <SelectValue :placeholder="t('router.edit.upstreamPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="opt in upstreamOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <AppInput v-model="form.base_url" :label="t('router.edit.baseUrl')" placeholder="https://api.example.com" :tooltip="tips.baseUrl" />
      <AppInput v-model="form.api_key" :label="t('router.edit.apiKey')" placeholder="sk-..." type="password" :tooltip="tips.apiKey" />
      <AppInput v-model="form.default_model" :label="t('router.edit.defaultModel')" :placeholder="t('router.edit.defaultModelPlaceholder')" :tooltip="tips.model" />

      <div class="space-y-2">
        <div class="flex items-center justify-between gap-2">
          <FieldLabel :label="t('router.edit.fallbacks')" :hint="tips.fallbacks" />
          <div class="flex items-center gap-1">
            <Select v-if="importableEnvs.length > 0" :model-value="''" @update:model-value="importFallbackFromEnv">
              <SelectTrigger size="sm" class="h-7 w-auto gap-1 text-xs">
                <SelectValue :placeholder="t('router.edit.importFromConfig')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="env in importableEnvs" :key="env.name" :value="env.name">
                  {{ env.name }}
                </SelectItem>
              </SelectContent>
            </Select>
            <Button type="button" variant="link" size="sm" @click="addFallbackRow">{{ t('router.edit.addFallback') }}</Button>
          </div>
        </div>
        <p v-if="fallbackRows.length === 0" class="text-xs text-muted-foreground">
          {{ t('router.edit.fallbackEmpty') }}
        </p>
        <div v-for="(row, i) in fallbackRows" :key="i" class="flex items-center gap-2">
          <span class="w-5 shrink-0 text-center text-xs text-muted-foreground">{{ i + 1 }}</span>
          <Input v-model="row.base_url" class="flex-[3] font-mono text-xs" :placeholder="t('router.edit.fallbackBaseUrl')" />
          <Input v-model="row.api_key" type="password" class="flex-[2] font-mono text-xs" :placeholder="t('router.edit.fallbackApiKey')" />
          <AppTooltip :content="t('router.edit.removeFallback')">
            <Button type="button" variant="ghost" size="icon-sm" @click="removeFallbackRow(i)">
              <X />
            </Button>
          </AppTooltip>
        </div>
      </div>

      <div class="space-y-3 border-t pt-3">
        <Button type="button" variant="ghost" size="sm" @click="showAdvanced = !showAdvanced">
          {{ showAdvanced ? t('router.edit.advancedCollapse') : t('router.edit.advanced') }}
        </Button>
        <div v-if="showAdvanced" class="space-y-3">
          <div>
            <div class="mb-1.5 flex items-center justify-between">
              <FieldLabel :label="t('router.edit.mapping')" :hint="tips.mapping" />
              <Button type="button" variant="link" size="sm" @click="addMappingRow">{{ t('router.edit.addRow') }}</Button>
            </div>
            <div class="space-y-2">
              <div v-for="(row, i) in mappingRows" :key="i" class="flex items-center">
                <Input v-model="row.source" class="flex-1 font-mono text-xs" :placeholder="t('router.edit.sourceModel')" />
                <span class="shrink-0 px-2 text-xs text-muted-foreground">→</span>
                <Input v-model="row.target" class="flex-1 font-mono text-xs" :placeholder="t('router.edit.targetModel')" />
                <AppTooltip :content="t('router.edit.removeRow')">
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
        <Label class="ml-2 cursor-pointer">{{ t('router.edit.enabled') }}</Label>
      </div>

      <div v-if="form.name.trim()" class="space-y-1.5 rounded-lg border bg-muted/40 p-3 text-xs leading-relaxed text-muted-foreground">
        <p class="font-medium text-foreground">{{ t('router.edit.howTo') }}</p>
        <p v-if="isProviderRoute">{{ t('router.edit.howToProviderBefore', { name: clientLabel }) }} <span class="font-mono text-foreground">{{ routeUrl }}</span> {{ t('router.edit.howToProviderAfter') }}</p>
        <p v-else>{{ t('router.edit.howToCustomBefore') }} <span class="font-mono text-foreground">{{ accessUrl }}</span> {{ t('router.edit.howToCustomAfter') }}</p>
      </div>
    </form>

    <template #footer>
      <Button type="button" variant="secondary" @click="isOpen = false">{{ t('common.cancel') }}</Button>
      <Button type="button" @click="handleSubmit">{{ isEditing ? t('common.save') : t('router.edit.add') }}</Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { ref, computed, watch } from 'vue'
import { X } from '@lucide/vue'
import type { APIRoute, APIFormat, EnvConfig, Provider } from '@/types'
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

const { t } = useI18n()

const AUTO_PROVIDERS: Provider[] = ['claude', 'claude_desktop', 'codex', 'antigravity', 'opencode', 'grok']

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
  { value: 'claude_desktop', label: 'Claude Desktop' },
  { value: 'codex', label: 'Codex' },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'opencode', label: 'OpenCode' },
  { value: 'grok', label: 'Grok' },
]

const tips = computed(() => ({
  name: t('router.edit.tips.name'),
  client: t('router.edit.tips.client'),
  upstream: t('router.edit.tips.upstream'),
  baseUrl: t('router.edit.tips.baseUrl'),
  apiKey: t('router.edit.tips.apiKey'),
  model: t('router.edit.tips.model'),
  mapping: t('router.edit.tips.mapping'),
  fallbacks: t('router.edit.tips.fallbacks'),
}))

interface Preset {
  label: string
  hint: string
  client: Provider
  upstream: string
}

const presets = computed<Preset[]>(() => [
  {
    label: 'Claude ← Chat Completions',
    hint: t('router.edit.presetHint.claudeChat'),
    client: 'claude',
    upstream: 'chat_completions',
  },
  {
    label: 'Codex ← Chat Completions',
    hint: t('router.edit.presetHint.codexChat'),
    client: 'codex',
    upstream: 'chat_completions',
  },
  {
    label: 'Codex ← Anthropic Messages',
    hint: t('router.edit.presetHint.codexAnthropic'),
    client: 'codex',
    upstream: 'anthropic_messages',
  },
  {
    label: t('router.edit.passthrough'),
    hint: t('router.edit.presetHint.passthrough'),
    client: 'claude',
    upstream: 'native',
  },
])

function clientFromFilter(): Provider {
  const filter = configStore.currentFilter
  if (filter === 'codex' || filter === 'antigravity' || filter === 'opencode' || filter === 'grok') return filter
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
const fallbackRows = ref<{ base_url: string; api_key: string }[]>([])

function addFallbackRow() {
  fallbackRows.value.push({ base_url: '', api_key: '' })
}

function removeFallbackRow(index: number) {
  fallbackRows.value.splice(index, 1)
}

// upstreamOfEnv 取配置的真实上游地址与 Key（与后端 upstreamVarsForEnv 一致）
function upstreamOfEnv(env: EnvConfig): { base_url: string; api_key: string } {
  const v = env.variables || {}
  switch (env.provider) {
    case 'claude':
    case 'claude_desktop':
      return { base_url: v.ANTHROPIC_BASE_URL || '', api_key: v.ANTHROPIC_AUTH_TOKEN || v.ANTHROPIC_API_KEY || '' }
    case 'codex':
      return { base_url: v.base_url || '', api_key: v.OPENAI_API_KEY || '' }
    case 'antigravity':
      return { base_url: v.GOOGLE_GEMINI_BASE_URL || '', api_key: v.GEMINI_API_KEY || v.GOOGLE_API_KEY || '' }
    case 'opencode':
      return { base_url: v.OPENCODE_BASE_URL || '', api_key: v.OPENCODE_API_KEY || '' }
    case 'grok':
      return { base_url: v.XAI_BASE_URL || '', api_key: v.XAI_API_KEY || '' }
    default:
      return { base_url: '', api_key: '' }
  }
}

// 同一模型商下、带有地址的其它配置都可以一键设为备用（Claude Code 与 Desktop 协议相同，互通）
const importableEnvs = computed(() => {
  const client = form.value.client
  const sameFamily = (p: Provider) => p === client || ((client === 'claude' || client === 'claude_desktop') && (p === 'claude' || p === 'claude_desktop'))
  return configStore.environments.filter(env =>
    sameFamily(env.provider) && !env.official_login && /^https?:\/\//i.test(upstreamOfEnv(env).base_url.trim()),
  )
})

function importFallbackFromEnv(value: unknown) {
  const env = importableEnvs.value.find(item => item.name === value)
  if (!env) return
  const up = upstreamOfEnv(env)
  const base = up.base_url.trim().replace(/\/+$/, '')
  const exists = fallbackRows.value.some(row => row.base_url.trim().replace(/\/+$/, '') === base && row.api_key.trim() === up.api_key.trim())
  if (exists || (base === form.value.base_url.trim().replace(/\/+$/, '') && up.api_key.trim() === form.value.api_key.trim())) {
    toast.info(t('router.edit.alreadyInList', { name: env.name }))
    return
  }
  fallbackRows.value.push({ base_url: base, api_key: up.api_key.trim() })
}

const isAutoRoute = computed(() => {
  const name = (props.editRoute?.name || '').toLowerCase()
  const desc = props.editRoute?.description || ''
  // 描述由后端写入（中文固定文案），这里只是识别标记，不随界面语言变化
  return AUTO_PROVIDERS.includes(name as Provider) || desc.includes('应用路由')
})

const isProviderRoute = computed(() => AUTO_PROVIDERS.includes(form.value.name.trim().toLowerCase() as Provider))
const clientLabel = computed(() => clients.find(item => item.value === form.value.client)?.label || form.value.client)

const upstreamOptions = computed(() => {
  const native = (name: string) => t('router.edit.nativeOpt', { name })
  const routed = (name: string) => t('router.edit.routedOpt', { name })
  const extra: Record<Provider, { value: string; label: string }[]> = {
    claude_desktop: [
      { value: 'native', label: native('Anthropic Messages') },
      { value: 'chat_completions', label: routed('Chat Completions') },
      { value: 'responses', label: routed('Responses') },
    ],
    claude: [
      { value: 'native', label: native('Anthropic Messages') },
      { value: 'chat_completions', label: routed('Chat Completions') },
      { value: 'responses', label: routed('Responses') },
    ],
    codex: [
      { value: 'native', label: native('Responses') },
      { value: 'chat_completions', label: routed('Chat Completions') },
      { value: 'anthropic_messages', label: routed('Anthropic Messages') },
      { value: 'responses', label: routed('Responses') },
    ],
    antigravity: [
      { value: 'native', label: native('Antigravity') },
      { value: 'chat_completions', label: routed('Chat Completions') },
      { value: 'anthropic_messages', label: routed('Anthropic Messages') },
      { value: 'responses', label: routed('Responses') },
    ],
    opencode: [
      { value: 'native', label: native('Chat Completions') },
      { value: 'anthropic_messages', label: routed('Anthropic Messages') },
      { value: 'responses', label: routed('Responses') },
    ],
    grok: [
      { value: 'native', label: native('Responses') },
      { value: 'chat_completions', label: routed('Chat Completions') },
      { value: 'anthropic_messages', label: routed('Anthropic Messages') },
      { value: 'responses', label: routed('Responses') },
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
  return client === 'claude' || client === 'claude_desktop' ? 'anthropic' : 'openai'
}

function targetOf(client: Provider, format: string): APIFormat {
  if (format === 'anthropic_messages') return 'anthropic'
  if (format === 'chat_completions') return 'openai'
  if (format === 'responses') return 'responses'
  if (client === 'claude' || client === 'claude_desktop') return 'anthropic'
  if (client === 'opencode' || client === 'antigravity') return 'openai'
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
  if (client === 'opencode' || client === 'antigravity') return 'native'
  return 'chat_completions'
}

const routeUrl = computed(() => `http://127.0.0.1:${routerStore.config.port || 8790}/${form.value.name.trim()}`)
const accessUrl = computed(() => {
  const base = routeUrl.value
  return form.value.client === 'claude' || form.value.client === 'claude_desktop' ? base : `${base}/v1`
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
      fallbackRows.value = []
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
      fallbackRows.value = (route.fallbacks || []).map(fb => ({ base_url: fb.base_url || '', api_key: fb.api_key || '' }))
      showAdvanced.value = Object.keys(route.model_mapping || {}).length > 0
    } else {
      form.value = defaultForm()
      mappingRows.value = [{ source: '', target: '' }]
      fallbackRows.value = []
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
  if (value === 'claude' || value === 'claude_desktop' || value === 'codex' || value === 'antigravity' || value === 'opencode' || value === 'grok') {
    form.value.client = value
    const allowed = upstreamOptions.value.some(opt => opt.value === form.value.upstream)
    if (!allowed) form.value.upstream = 'native'
  }
}

function onEnabledChange(checked: boolean) {
  form.value.enabled = checked
}

async function handleSubmit() {
  const name = form.value.name.trim()
  if (!name) {
    toast.error(t('router.edit.nameRequired'))
    return
  }
  if (!/^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/.test(name)) {
    toast.error(t('router.edit.nameInvalid'))
    return
  }
  if (!form.value.base_url.trim()) {
    toast.error(t('router.edit.baseUrlRequired'))
    return
  }
  const fallbacks = fallbackRows.value
    .map(row => ({ base_url: row.base_url.trim(), api_key: row.api_key.trim() || undefined }))
    .filter(row => row.base_url)
  if (fallbacks.some(row => !/^https?:\/\//i.test(row.base_url))) {
    toast.error(t('router.edit.fallbackUrlInvalid'))
    return
  }

  const duplicate = routerStore.config.routes.some(
    route => route.name.toLowerCase() === name.toLowerCase() && route.name !== props.editRoute?.name,
  )
  if (duplicate) {
    toast.error(t('router.edit.nameExists'))
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
    fallbacks: fallbacks.length > 0 ? fallbacks : undefined,
  }

  const routes = [...routerStore.config.routes]
  const existIndex = props.editRoute ? routes.findIndex(item => item.name === props.editRoute!.name) : -1
  if (existIndex >= 0) routes[existIndex] = route
  else routes.push(route)

  try {
    await routerStore.saveConfig({ ...routerStore.config, routes })
    toast.success(isEditing.value ? t('router.edit.saved') : t('router.edit.added'))
    isOpen.value = false
    emit('saved')
  } catch (e: unknown) {
    toast.error(t('router.edit.saveFailed', { error: e instanceof Error ? e.message : String(e) }))
  }
}
</script>
