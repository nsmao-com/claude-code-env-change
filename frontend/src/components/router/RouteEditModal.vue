<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑路由' : '添加路由'" size="md">
    <form @submit.prevent="handleSubmit">
      <div v-if="!isEditing" class="mb-4">
        <Label class="mb-1.5">快捷场景</Label>
        <div class="space-y-2">
          <Button
            v-for="preset in presets"
            :key="preset.label"
            type="button"
            variant="outline"
            class="h-auto w-full flex-col items-start gap-1 whitespace-normal py-2.5"
            @click="applyPreset(preset)"
          >
            <span class="flex items-center gap-2">
              <BrandIcon v-if="preset.brand" :provider="preset.brand" class="size-3.5 text-primary" />
              <component :is="preset.icon" v-else class="size-3.5 text-primary" />
              <span class="text-sm font-bold">{{ preset.label }}</span>
            </span>
            <span class="text-xs font-normal text-muted-foreground">{{ preset.hint }}</span>
          </Button>
        </div>
      </div>

      <div class="mb-4">
        <AppInput v-model="form.name" label="路由名称" placeholder="如 glm-claude（用于 URL 路径）" />
      </div>

      <div class="mb-4 grid grid-cols-2 gap-3">
        <div class="grid gap-1.5">
          <Label>客户端协议（谁来连）</Label>
          <Select :model-value="form.source_format" @update:model-value="onSourceFormat">
            <SelectTrigger class="w-full">
              <SelectValue placeholder="选择客户端协议" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="anthropic">Anthropic（Claude Code）</SelectItem>
              <SelectItem value="openai">OpenAI（Codex 等）</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="grid gap-1.5">
          <Label>上游协议（转成什么）</Label>
          <Select :model-value="form.target_format" @update:model-value="onTargetFormat">
            <SelectTrigger class="w-full">
              <SelectValue placeholder="选择上游协议" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="openai">OpenAI 兼容接口</SelectItem>
              <SelectItem value="anthropic">Anthropic 接口</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <div class="mb-4">
        <AppInput v-model="form.base_url" label="上游 Base URL" placeholder="https://api.example.com（一般填到域名或 /v1 之前）" />
      </div>

      <div class="mb-4">
        <AppInput v-model="form.api_key" label="上游 API Key" placeholder="sk-..." type="password" />
      </div>

      <div class="mb-4">
        <AppInput v-model="form.default_model" label="默认模型（可选）" placeholder="未命中映射时使用的模型名" />
      </div>

      <div class="mb-4">
        <div class="mb-1.5 flex items-center justify-between">
          <Label>模型映射</Label>
          <Button type="button" variant="link" size="sm" @click="addMappingRow">添加一行</Button>
        </div>
        <div class="space-y-2">
          <div v-for="(row, i) in mappingRows" :key="i" class="flex items-center">
            <Input
              v-model="row.source"
              class="flex-1 font-mono text-xs"
              placeholder="源模型，如 claude-sonnet-4 或 *"
            />
            <span class="shrink-0 px-2 text-xs text-muted-foreground">→</span>
            <Input
              v-model="row.target"
              class="flex-1 font-mono text-xs"
              placeholder="上游模型"
            />
            <Button type="button" variant="ghost" size="icon-sm" title="删除" @click="removeMappingRow(i)">
              <X />
            </Button>
          </div>
        </div>
        <p class="mt-1 text-[11px] text-muted-foreground">
          精确匹配优先，<span class="font-mono">*</span> 为兜底映射；全部留空则原样转发模型名。
        </p>
      </div>

      <div class="flex items-center gap-2">
        <Switch :checked="form.enabled" @update:checked="onEnabledChange" />
        <Label class="cursor-pointer">启用此路由</Label>
      </div>

      <div v-if="form.name.trim()" class="mt-4 space-y-2 rounded-lg border border-border/50 bg-secondary/40 p-3 text-xs">
        <p class="font-bold text-foreground">接入方式</p>
        <template v-if="form.source_format === 'anthropic'">
          <div v-for="snippet in claudeSnippets" :key="snippet.label" class="space-y-1">
            <div class="flex items-center justify-between">
              <span class="text-muted-foreground">{{ snippet.label }}</span>
              <Button type="button" variant="link" size="sm" @click="copyText(snippet.text, snippet.label)">复制</Button>
            </div>
            <pre class="overflow-x-auto whitespace-pre rounded border border-border/40 bg-background/60 p-2 font-mono">{{ snippet.text }}</pre>
          </div>
          <p class="text-muted-foreground">API Key 任意填写即可（网关使用路由里配置的上游 Key）。</p>
        </template>
        <template v-else>
          <div class="flex items-center justify-between">
            <span class="text-muted-foreground">Codex config.toml（~/.codex/config.toml）</span>
            <Button type="button" variant="link" size="sm" @click="copyText(codexSnippet, 'Codex 配置')">复制</Button>
          </div>
          <pre class="overflow-x-auto whitespace-pre rounded border border-border/40 bg-background/60 p-2 font-mono">{{ codexSnippet }}</pre>
          <p class="text-muted-foreground">新版 Codex 默认走 Responses API，网关已支持；若上游只认 Chat Completions，把 <span class="font-mono">wire_api</span> 改成 <span class="font-mono">"chat"</span> 即可。</p>
        </template>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end">
        <Button type="button" variant="secondary" @click="isOpen = false">取消</Button>
        <Button type="button" @click="handleSubmit">{{ isEditing ? '保存' : '添加' }}</Button>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch, type Component } from 'vue'
import { ArrowLeftRight, X } from '@lucide/vue'
import type { APIRoute, APIFormat } from '@/types'
import { useRouterStore } from '@/stores/routerStore'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

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
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isEditing = computed(() => props.editRoute != null)

interface Preset {
  label: string
  hint: string
  icon?: Component
  brand?: string
  source: APIFormat
  target: APIFormat
}

const presets: Preset[] = [
  {
    label: 'OpenAI 兼容接口 → Claude Code',
    hint: '把 GLM/DeepSeek 等 OpenAI 格式接口转换成 Anthropic 协议，供 Claude Code 直接使用',
    brand: 'claude',
    source: 'anthropic',
    target: 'openai'
  },
  {
    label: 'Claude 接口 → Codex',
    hint: '把 Anthropic 接口转换成 OpenAI Responses，供新版 Codex（wire_api = responses）使用',
    brand: 'codex',
    source: 'openai',
    target: 'anthropic'
  },
  {
    label: '同协议直连（仅改模型名）',
    hint: '协议一致只做模型名映射转发，用于中转站或模型改名',
    icon: ArrowLeftRight,
    source: 'anthropic',
    target: 'anthropic'
  }
]

const defaultForm = () => ({
  name: '',
  source_format: 'anthropic' as APIFormat,
  target_format: 'openai' as APIFormat,
  base_url: '',
  api_key: '',
  default_model: '',
  enabled: true
})

const form = ref(defaultForm())
const mappingRows = ref<{ source: string; target: string }[]>([{ source: '', target: '' }])

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
  if (entries.length === 0) {
    return [{ source: '', target: '' }]
  }
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
      return
    }
    if (props.editRoute) {
      const r = props.editRoute
      form.value = {
        name: r.name || '',
        source_format: r.source_format || 'anthropic',
        target_format: r.target_format || 'openai',
        base_url: r.base_url || '',
        api_key: r.api_key || '',
        default_model: r.default_model || '',
        enabled: r.enabled !== false
      }
      mappingRows.value = mappingFromRecord(r.model_mapping)
    } else {
      form.value = defaultForm()
      mappingRows.value = [{ source: '', target: '' }]
    }
  }
)

function applyPreset(preset: Preset) {
  form.value.source_format = preset.source
  form.value.target_format = preset.target
}

function onSourceFormat(value: unknown) {
  if (value === 'anthropic' || value === 'openai') {
    form.value.source_format = value
  }
}

function onTargetFormat(value: unknown) {
  if (value === 'anthropic' || value === 'openai') {
    form.value.target_format = value
  }
}

function onEnabledChange(checked: boolean) {
  form.value.enabled = checked
}

const routeUrl = computed(() => `http://127.0.0.1:${routerStore.config.port || 8790}/${form.value.name.trim()}`)

const claudeSnippets = computed(() => [
  {
    label: 'Linux / macOS（bash）',
    text: `export ANTHROPIC_BASE_URL="${routeUrl.value}"\nexport ANTHROPIC_AUTH_TOKEN="local-router"`
  },
  {
    label: 'Windows（PowerShell）',
    text: `$env:ANTHROPIC_BASE_URL = "${routeUrl.value}"\n$env:ANTHROPIC_AUTH_TOKEN = "local-router"`
  }
])

const codexSnippet = computed(() => {
  const provider = form.value.name.trim().replace(/[^a-zA-Z0-9_-]/g, '-')
  return `# ~/.codex/config.toml\nmodel_provider = "${provider}"\n\n[model_providers.${provider}]\nname = "${provider}"\nbase_url = "${routeUrl.value}/v1"\nwire_api = "responses"`
})

async function copyText(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(`${label} 已复制`)
  } catch {
    toast.error('复制失败，请手动选择复制')
  }
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
    (r) => r.name.toLowerCase() === name.toLowerCase() && r.name !== props.editRoute?.name
  )
  if (duplicate) {
    toast.error('路由名称已存在')
    return
  }

  const route: APIRoute = {
    name,
    description: props.editRoute?.description,
    source_format: form.value.source_format,
    target_format: form.value.target_format,
    base_url: form.value.base_url.trim(),
    api_key: form.value.api_key.trim() || undefined,
    default_model: form.value.default_model.trim() || undefined,
    model_mapping: mappingToRecord(),
    enabled: form.value.enabled
  }

  const routes = [...routerStore.config.routes]
  const existIndex = props.editRoute ? routes.findIndex((r) => r.name === props.editRoute!.name) : -1
  if (existIndex >= 0) {
    routes[existIndex] = route
  } else {
    routes.push(route)
  }

  try {
    await routerStore.saveConfig({ ...routerStore.config, routes })
    toast.success(isEditing.value ? '路由已保存' : '路由已添加')
    isOpen.value = false
    emit('saved')
  } catch (e: any) {
    toast.error('保存失败: ' + (e?.message || String(e)))
  }
}
</script>
