<template>
  <AppModal v-model="isOpen" size="lg" :title="tx('内置工具', 'Built-in tools')">
    <p class="mb-4 text-xs leading-relaxed text-muted-foreground">
      {{
        tx(
          'AI ENV 自带两个 MCP 工具：生成图片与联网搜索。添加后由本程序以 stdio 方式运行，Agent 调用时读取这里的设置，修改设置无需重启 Agent。',
          'AI ENV ships two MCP tools: image generation and web search. Once added they run from this program over stdio and read these settings on every call, so changes need no Agent restart.',
        )
      }}
    </p>

    <section class="rounded-xl border p-4">
      <div class="flex items-center justify-between gap-3">
        <div>
          <p class="text-sm font-semibold">{{ tx('生成图片', 'Image generation') }} <span class="font-mono text-xs font-normal text-muted-foreground">generate_image</span></p>
          <p class="mt-0.5 text-xs text-muted-foreground">
            {{ tx('使用支持 OpenAI 图片接口（/v1/images/generations）的环境，图片保存到项目的 generated-images 目录。', 'Uses an environment with the OpenAI images API (/v1/images/generations); images are saved to generated-images in the project.') }}
          </p>
        </div>
        <Badge v-if="info.image_installed" variant="secondary">{{ tx('已添加', 'Added') }}</Badge>
      </div>
      <div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div class="grid gap-1.5">
          <Label class="text-xs">{{ tx('环境', 'Environment') }}</Label>
          <Select v-model="imageEnv">
            <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="tx('选择环境', 'Select environment')" /></SelectTrigger>
            <SelectContent position="popper" align="start">
              <SelectItem v-for="e in envs" :key="`${e.provider}/${e.name}`" :value="`${e.provider}/${e.name}`">
                {{ e.provider }} · {{ e.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <AppInput v-model="form.image_model" :label="tx('图片模型', 'Image model')" placeholder="gpt-image-1" />
        <div class="grid gap-1.5">
          <Label class="text-xs">{{ tx('默认尺寸', 'Default size') }}</Label>
          <Select v-model="sizeValue">
            <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
            <SelectContent position="popper" align="start">
              <SelectItem v-for="s in sizes" :key="s" :value="s">{{ s }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </section>

    <section class="mt-4 rounded-xl border p-4">
      <div class="flex items-center justify-between gap-3">
        <div>
          <p class="text-sm font-semibold">{{ tx('联网搜索', 'Web search') }} <span class="font-mono text-xs font-normal text-muted-foreground">web_search</span></p>
          <p class="mt-0.5 text-xs text-muted-foreground">
            {{ tx('选择搜索服务并填写它的 API Key。', 'Pick a search service and enter its API key.') }}
          </p>
        </div>
        <Badge v-if="info.search_installed" variant="secondary">{{ tx('已添加', 'Added') }}</Badge>
      </div>
      <div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div class="grid gap-1.5">
          <Label class="text-xs">{{ tx('搜索服务', 'Search service') }}</Label>
          <Select v-model="searchValue">
            <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="tx('选择搜索服务', 'Select a service')" /></SelectTrigger>
            <SelectContent position="popper" align="start">
              <SelectItem v-for="p in searchProviders" :key="p.value" :value="p.value">{{ p.label }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <AppInput v-model="form.search_key" type="password" label="API Key" :placeholder="keyPlaceholder" />
      </div>
      <div class="mt-3 flex flex-wrap items-center gap-2">
        <Button size="sm" variant="outline" type="button" :disabled="busy || !form.search_provider || !form.search_key" @click="testSearch">
          {{ tx('测试搜索', 'Test search') }}
        </Button>
        <span v-if="testText" class="text-xs text-muted-foreground">{{ testText }}</span>
      </div>
    </section>

    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-between gap-2">
        <div class="flex gap-2">
          <Button size="sm" variant="outline" type="button" :disabled="busy || info.image_installed || !form.image_env" @click="install('image')">
            {{ tx('添加图片工具到 MCP', 'Add image tool to MCP') }}
          </Button>
          <Button size="sm" variant="outline" type="button" :disabled="busy || info.search_installed || !form.search_provider" @click="install('search')">
            {{ tx('添加搜索工具到 MCP', 'Add search tool to MCP') }}
          </Button>
        </div>
        <Button size="sm" type="button" :disabled="busy" @click="save">{{ tx('保存设置', 'Save settings') }}</Button>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { useToast } from '@/composables/useToast'
import { useConfigStore } from '@/stores/configStore'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

interface BuiltinToolSettings {
  image_provider?: string
  image_env?: string
  image_model?: string
  image_size?: string
  search_provider?: string
  search_key?: string
}
interface BuiltinToolsInfo {
  settings: BuiltinToolSettings
  image_installed: boolean
  search_installed: boolean
  command: string
}

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; installed: [] }>()
const { tx } = useWorkbench()
const toast = useToast()
const config = useConfigStore()

const isOpen = computed({ get: () => props.modelValue, set: (v) => emit('update:modelValue', v) })
const info = ref<BuiltinToolsInfo>({ settings: {}, image_installed: false, search_installed: false, command: '' })
const form = reactive<Required<BuiltinToolSettings>>({
  image_provider: '',
  image_env: '',
  image_model: 'gpt-image-1',
  image_size: '1024x1024',
  search_provider: '',
  search_key: '',
})
const busy = ref(false)
const testText = ref('')
const sizes = ['1024x1024', '1536x1024', '1024x1536', 'auto']
const searchProviders = [
  { value: 'tavily', label: 'Tavily' },
  { value: 'brave', label: 'Brave Search' },
  { value: 'exa', label: 'Exa' },
  { value: 'bocha', label: '博查 Bocha' },
]
const envs = computed(() => config.environments.filter((e) => !e.official_login))
const imageEnv = computed({
  get: () => (form.image_env ? `${form.image_provider}/${form.image_env}` : ''),
  set: (v: string) => {
    const env = envs.value.find((e) => `${e.provider}/${e.name}` === v)
    form.image_provider = env?.provider || ''
    form.image_env = env?.name || ''
  },
})
const sizeValue = computed({ get: () => form.image_size || '1024x1024', set: (v: string) => (form.image_size = v) })
const searchValue = computed({ get: () => form.search_provider, set: (v: string) => (form.search_provider = v) })
const keyPlaceholder = computed(() => ({ tavily: 'tvly-…', brave: 'BSA…', exa: 'exa-…', bocha: 'sk-…' } as Record<string, string>)[form.search_provider] || '')

watch(
  isOpen,
  async (open) => {
    if (!open) return
    testText.value = ''
    try {
      await config.loadConfig()
      info.value = await workbench<BuiltinToolsInfo>('GetBuiltinTools')
      Object.assign(form, { image_model: 'gpt-image-1', image_size: '1024x1024' }, info.value.settings)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  },
  { immediate: true },
)

async function save(): Promise<boolean> {
  busy.value = true
  try {
    await workbench('SaveBuiltinTools', { ...form })
    toast.success(tx('内置工具设置已保存', 'Built-in tool settings saved'))
    return true
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
    return false
  } finally {
    busy.value = false
  }
}

async function install(kind: 'image' | 'search') {
  if (!(await save())) return
  busy.value = true
  try {
    await workbench('InstallBuiltinMCP', kind)
    info.value = await workbench<BuiltinToolsInfo>('GetBuiltinTools')
    toast.success(tx('已添加到 MCP，并启用在 Claude Code 与 Codex', 'Added to MCP and enabled for Claude Code and Codex'))
    emit('installed')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    busy.value = false
  }
}

async function testSearch() {
  busy.value = true
  testText.value = ''
  try {
    const text = await workbench<string>('TestWebSearch', form.search_provider, form.search_key, 'Model Context Protocol')
    const first = text.split('\n').find((line) => /^\d+\./.test(line.trim()))
    testText.value = tx(`搜索成功：${first?.trim() || '已返回结果'}`, `OK: ${first?.trim() || 'results returned'}`)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    busy.value = false
  }
}
</script>
