<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑配置' : '新建配置'" size="lg">
    <form @submit.prevent="handleSubmit">
      <!-- Basic Info -->
      <div class="grid grid-cols-2 gap-4 mb-6">
        <div class="col-span-2 sm:col-span-1">
          <AppInput
            v-model="form.name"
            label="配置名称"
            placeholder="输入配置名称"
          />
        </div>
        <div class="col-span-2 sm:col-span-1">
          <label class="block text-sm font-medium mb-1.5">图标</label>
          <div class="relative">
            <button
              type="button"
              class="w-10 h-10 rounded-lg border border-border flex items-center justify-center text-xl hover:bg-muted"
              @click="showEmojiPicker = !showEmojiPicker"
            >
              {{ form.icon }}
            </button>
            <EmojiPicker
              :show="showEmojiPicker"
              @close="showEmojiPicker = false"
              @select="selectIcon"
            />
          </div>
        </div>
        <div class="col-span-2">
          <AppInput
            v-model="form.description"
            label="描述"
            placeholder="可选的配置描述"
          />
        </div>
      </div>

      <!-- Provider Tabs -->
      <div class="provider-tabs">
        <button
          v-for="p in providers"
          :key="p.value"
          type="button"
          :class="['provider-tab', { active: form.provider === p.value }]"
          @click="form.provider = p.value"
        >
          <i :class="[p.icon, 'mr-2']"></i>
          {{ p.label }}
        </button>
      </div>

      <!-- Claude Fields -->
      <div v-if="form.provider === 'claude'" class="space-y-4">
        <AppInput
          v-model="form.claude.baseUrl"
          label="Base URL"
          placeholder="https://api.anthropic.com"
        >
          <template #suffix>
            <button
              type="button"
              class="w-6 h-6 rounded hover:bg-muted flex items-center justify-center text-muted-foreground"
              @click="testLatency(form.claude.baseUrl)"
            >
              <i class="fas fa-bolt text-xs"></i>
            </button>
          </template>
        </AppInput>
        <AppInput
          v-model="form.claude.authToken"
          label="Auth Token"
          placeholder="可选"
        />
        <AppInput
          v-model="form.claude.model"
          label="Model"
          placeholder="claude-sonnet-4-20250514"
        />
        <AppInput
          v-model="form.claude.apiKey"
          label="API Key"
          type="password"
          placeholder="sk-ant-..."
        />

        <!-- Claude Code 优化选项 -->
        <div class="pt-3 border-t border-border space-y-3">
          <div class="text-xs font-medium text-muted-foreground uppercase tracking-wide">Claude Code 环境变量</div>

          <div class="flex items-center justify-between">
            <div>
              <div class="text-sm font-medium">Attribution Header</div>
              <div class="text-[11px] text-muted-foreground font-mono">CLAUDE_CODE_ATTRIBUTION_HEADER</div>
            </div>
            <div class="flex items-center gap-1 p-0.5 bg-muted rounded-lg">
              <button
                type="button"
                :class="['px-3 py-1 text-xs font-medium rounded-md transition-all', form.claude.attributionHeader === '' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground']"
                @click="form.claude.attributionHeader = ''"
              >不设置</button>
              <button
                type="button"
                :class="['px-3 py-1 text-xs font-medium rounded-md transition-all', form.claude.attributionHeader === '0' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground']"
                @click="form.claude.attributionHeader = '0'"
              >0</button>
              <button
                type="button"
                :class="['px-3 py-1 text-xs font-medium rounded-md transition-all', form.claude.attributionHeader === '1' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground']"
                @click="form.claude.attributionHeader = '1'"
              >1</button>
            </div>
          </div>

          <div class="flex items-center justify-between">
            <div>
              <div class="text-sm font-medium">Disable Nonessential Traffic</div>
              <div class="text-[11px] text-muted-foreground font-mono">CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC</div>
            </div>
            <div class="flex items-center gap-1 p-0.5 bg-muted rounded-lg">
              <button
                type="button"
                :class="['px-3 py-1 text-xs font-medium rounded-md transition-all', form.claude.disableNonessentialTraffic === '' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground']"
                @click="form.claude.disableNonessentialTraffic = ''"
              >不设置</button>
              <button
                type="button"
                :class="['px-3 py-1 text-xs font-medium rounded-md transition-all', form.claude.disableNonessentialTraffic === '0' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground']"
                @click="form.claude.disableNonessentialTraffic = '0'"
              >0</button>
              <button
                type="button"
                :class="['px-3 py-1 text-xs font-medium rounded-md transition-all', form.claude.disableNonessentialTraffic === '1' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground']"
                @click="form.claude.disableNonessentialTraffic = '1'"
              >1</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Codex Fields -->
      <div v-if="form.provider === 'codex'" class="space-y-4">
        <AppInput
          v-model="form.codex.baseUrl"
          label="Base URL"
          placeholder="https://api.openai.com/v1"
        >
          <template #suffix>
            <button
              type="button"
              class="w-6 h-6 rounded hover:bg-muted flex items-center justify-center text-muted-foreground"
              @click="testLatency(form.codex.baseUrl)"
            >
              <i class="fas fa-bolt text-xs"></i>
            </button>
          </template>
        </AppInput>
        <AppInput
          v-model="form.codex.apiKey"
          label="API Key"
          type="password"
          placeholder="sk-..."
        />
        <AppInput
          v-model="form.codex.model"
          label="Model"
          placeholder="gpt-4"
        />

        <!-- Templates -->
        <div>
          <label class="block text-sm font-medium mb-1.5">config.toml 模板</label>
          <textarea
            v-model="form.codex.configTemplate"
            class="input h-32 font-mono text-xs"
            placeholder="TOML 配置模板..."
          ></textarea>
        </div>
        <div>
          <label class="block text-sm font-medium mb-1.5">auth.json 模板</label>
          <textarea
            v-model="form.codex.authTemplate"
            class="input h-24 font-mono text-xs"
            placeholder="JSON 认证模板..."
          ></textarea>
        </div>
      </div>

      <!-- Gemini Fields -->
      <div v-if="form.provider === 'gemini'" class="space-y-4">
        <AppInput
          v-model="form.gemini.baseUrl"
          label="Base URL"
          placeholder="https://generativelanguage.googleapis.com"
        >
          <template #suffix>
            <button
              type="button"
              class="w-6 h-6 rounded hover:bg-muted flex items-center justify-center text-muted-foreground"
              @click="testLatency(form.gemini.baseUrl)"
            >
              <i class="fas fa-bolt text-xs"></i>
            </button>
          </template>
        </AppInput>
        <AppInput
          v-model="form.gemini.apiKey"
          label="API Key"
          type="password"
          placeholder="API Key"
        />
        <AppInput
          v-model="form.gemini.model"
          label="Model"
          placeholder="gemini-pro"
        />

        <!-- Templates -->
        <div>
          <label class="block text-sm font-medium mb-1.5">.env 模板</label>
          <textarea
            v-model="form.gemini.envTemplate"
            class="input h-24 font-mono text-xs"
            placeholder="环境变量模板..."
          ></textarea>
        </div>
        <div>
          <label class="block text-sm font-medium mb-1.5">settings.json 模板</label>
          <textarea
            v-model="form.gemini.settingsTemplate"
            class="input h-24 font-mono text-xs"
            placeholder="JSON 设置模板..."
          ></textarea>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="isOpen = false">
          取消
        </button>
        <button type="button" class="btn btn-primary" @click="handleSubmit">
          {{ isEditing ? '保存' : '创建' }}
        </button>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { EnvConfig, Provider } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import EmojiPicker from '@/components/common/EmojiPicker.vue'

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

function selectIcon(emoji: string) {
  form.value.icon = emoji
}

const providers = [
  { value: 'claude' as Provider, label: 'Claude', icon: 'fas fa-robot' },
  { value: 'codex' as Provider, label: 'Codex', icon: 'fas fa-terminal' },
  { value: 'gemini' as Provider, label: 'Gemini', icon: 'fas fa-gem' }
]

const defaultForm = () => ({
  name: '',
  description: '',
  icon: '📦',
  provider: 'claude' as Provider,
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
  }
})

const form = ref(defaultForm())
const originalName = ref('')

// Watch for edit config changes
watch(() => props.editConfig, (config) => {
  if (config) {
    originalName.value = config.name
    form.value.name = config.name
    form.value.description = config.description || ''
    form.value.icon = config.icon || '📦'
    form.value.provider = config.provider

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
    }
  } else {
    form.value = defaultForm()
    originalName.value = ''
  }
}, { immediate: true })

// Reset form when modal closes
watch(isOpen, (open) => {
  if (!open) {
    form.value = defaultForm()
    originalName.value = ''
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
  } catch (e) {
    toast.error('测速失败')
  }
}

async function handleSubmit() {
  if (!form.value.name.trim()) {
    toast.error('请输入配置名称')
    return
  }

  // Check duplicate name
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
  }

  const configData: EnvConfig = {
    name: form.value.name.trim(),
    description: form.value.description.trim(),
    provider: form.value.provider,
    variables,
    templates,
    icon: form.value.icon,
    // Claude Code 特有配置 (始终传递，空字符串表示不设置)
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

<style scoped>
textarea.input {
  resize: vertical;
  min-height: 4rem;
}
</style>
