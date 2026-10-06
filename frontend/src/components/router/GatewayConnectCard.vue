<template>
  <Card class="mb-4">
    <CardContent>
      <Label class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{{ t('router.connect.title') }}</Label>
      <p class="mt-1 text-xs leading-relaxed text-muted-foreground">{{ t('router.connect.desc') }}</p>
      <p v-if="!routes.length" class="mt-3 text-xs text-muted-foreground">{{ t('router.connect.noRoutes') }}</p>
      <template v-else>
        <div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div class="grid gap-1.5">
            <Label class="text-xs">{{ t('router.connect.route') }}</Label>
            <Select v-model="routeName">
              <SelectTrigger class="h-8 w-full"><SelectValue /></SelectTrigger>
              <SelectContent position="popper" align="start">
                <SelectItem v-for="r in routes" :key="r.name" :value="r.name">{{ r.name }} · {{ formatLabel(r.source_format) }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="grid gap-1.5">
            <Label class="text-xs">{{ t('router.connect.address') }}</Label>
            <Select v-model="host">
              <SelectTrigger class="h-8 w-full"><SelectValue /></SelectTrigger>
              <SelectContent position="popper" align="start">
                <SelectItem value="127.0.0.1">{{ t('router.connect.local') }}</SelectItem>
                <SelectItem v-for="a in lanAddresses" :key="a" :value="a">{{ a }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="grid gap-1.5">
            <Label class="text-xs">{{ t('router.connect.key') }}</Label>
            <Select v-model="keyId">
              <SelectTrigger class="h-8 w-full"><SelectValue /></SelectTrigger>
              <SelectContent position="popper" align="start">
                <SelectItem v-if="host === '127.0.0.1'" value="none">{{ t('router.connect.noKey') }}</SelectItem>
                <SelectItem v-for="k in enabledKeys" :key="k.id" :value="k.id">{{ k.name }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <p v-if="host !== '127.0.0.1' && !enabledKeys.length" class="mt-2 text-[11px] text-destructive">{{ t('router.connect.keyHint') }}</p>
        <div class="mt-4">
          <SegmentedPills v-model="tab" layout-id="connect-tab" dense :items="tabs" />
        </div>
        <div class="relative mt-3">
          <pre class="max-h-72 overflow-auto rounded-lg bg-muted p-3 pr-12 font-mono text-[11px] leading-relaxed whitespace-pre">{{ snippet }}</pre>
          <Button
            variant="ghost"
            size="icon-sm"
            type="button"
            class="absolute right-2 top-2"
            :aria-label="t('router.connect.copy')"
            :title="t('router.connect.copy')"
            @click="copy"
          >
            <Copy />
          </Button>
        </div>
      </template>
    </CardContent>
  </Card>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Copy } from '@lucide/vue'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useRouterStore } from '@/stores/routerStore'
import type { APIFormat, APIRoute, GatewayAccessInfo } from '@/types'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const props = defineProps<{ access: GatewayAccessInfo | null }>()
const { t } = useI18n()
const toast = useToast()
const routerStore = useRouterStore()

const routes = computed<APIRoute[]>(() => routerStore.config.routes.filter((r) => r.enabled))
const routeName = ref('')
const host = ref('127.0.0.1')
const keyId = ref('none')
const tab = ref('env')

const lanAddresses = computed(() => (props.access?.lan_share ? props.access.addresses : []))
const enabledKeys = computed(() => (props.access?.keys || []).filter((k) => k.enabled))
const port = computed(() => props.access?.port || routerStore.status?.port || routerStore.config.port || 8790)
const route = computed(() => routes.value.find((r) => r.name === routeName.value) || routes.value[0])

watch(
  routes,
  (list) => {
    if (!list.some((r) => r.name === routeName.value)) routeName.value = list[0]?.name || ''
  },
  { immediate: true },
)
watch([host, enabledKeys], () => {
  const valid = enabledKeys.value.some((k) => k.id === keyId.value)
  if (host.value !== '127.0.0.1' && !valid) keyId.value = enabledKeys.value[0]?.id || ''
  else if (host.value === '127.0.0.1' && !valid) keyId.value = 'none'
})
watch(lanAddresses, (list) => {
  if (host.value !== '127.0.0.1' && !list.includes(host.value)) host.value = '127.0.0.1'
})

const tabs = computed(() => [
  { value: 'env', label: t('router.connect.envTab') },
  { value: 'curl', label: 'curl' },
  { value: 'python', label: 'Python' },
  { value: 'node', label: 'Node.js' },
])

function formatLabel(f: APIFormat) {
  return f === 'anthropic' ? 'Anthropic Messages' : f === 'responses' ? 'OpenAI Responses' : 'Chat Completions'
}

const apiKey = computed(() => enabledKeys.value.find((k) => k.id === keyId.value)?.key || 'aienv')
const model = computed(() => {
  const r = route.value
  if (!r) return 'MODEL'
  return r.default_model || r.model_mapping?.['*'] || Object.values(r.model_mapping || {}).find(Boolean) || 'MODEL'
})
const root = computed(() => `http://${host.value}:${port.value}/${route.value?.name || ''}`)

const snippet = computed(() => {
  const r = route.value
  if (!r) return ''
  const key = apiKey.value
  const m = model.value
  if (r.source_format === 'anthropic') {
    switch (tab.value) {
      case 'curl':
        return `curl ${root.value}/v1/messages \\\n  -H "x-api-key: ${key}" \\\n  -H "anthropic-version: 2023-06-01" \\\n  -H "content-type: application/json" \\\n  -d '{"model":"${m}","max_tokens":256,"messages":[{"role":"user","content":"Hello"}]}'`
      case 'python':
        return `import anthropic\n\nclient = anthropic.Anthropic(base_url="${root.value}", api_key="${key}")\nmsg = client.messages.create(\n    model="${m}",\n    max_tokens=256,\n    messages=[{"role": "user", "content": "Hello"}],\n)\nprint(msg.content[0].text)`
      case 'node':
        return `import Anthropic from '@anthropic-ai/sdk'\n\nconst client = new Anthropic({ baseURL: '${root.value}', apiKey: '${key}' })\nconst msg = await client.messages.create({\n  model: '${m}',\n  max_tokens: 256,\n  messages: [{ role: 'user', content: 'Hello' }],\n})\nconsole.log(msg.content[0].text)`
      default:
        return `# PowerShell\n$env:ANTHROPIC_BASE_URL = "${root.value}"\n$env:ANTHROPIC_AUTH_TOKEN = "${key}"\n\n# bash / zsh\nexport ANTHROPIC_BASE_URL="${root.value}"\nexport ANTHROPIC_AUTH_TOKEN="${key}"`
    }
  }
  const base = `${root.value}/v1`
  const responses = r.source_format === 'responses'
  switch (tab.value) {
    case 'curl':
      return responses
        ? `curl ${base}/responses \\\n  -H "Authorization: Bearer ${key}" \\\n  -H "content-type: application/json" \\\n  -d '{"model":"${m}","input":"Hello"}'`
        : `curl ${base}/chat/completions \\\n  -H "Authorization: Bearer ${key}" \\\n  -H "content-type: application/json" \\\n  -d '{"model":"${m}","messages":[{"role":"user","content":"Hello"}]}'`
    case 'python':
      return responses
        ? `from openai import OpenAI\n\nclient = OpenAI(base_url="${base}", api_key="${key}")\nresp = client.responses.create(model="${m}", input="Hello")\nprint(resp.output_text)`
        : `from openai import OpenAI\n\nclient = OpenAI(base_url="${base}", api_key="${key}")\nresp = client.chat.completions.create(\n    model="${m}",\n    messages=[{"role": "user", "content": "Hello"}],\n)\nprint(resp.choices[0].message.content)`
    case 'node':
      return responses
        ? `import OpenAI from 'openai'\n\nconst client = new OpenAI({ baseURL: '${base}', apiKey: '${key}' })\nconst resp = await client.responses.create({ model: '${m}', input: 'Hello' })\nconsole.log(resp.output_text)`
        : `import OpenAI from 'openai'\n\nconst client = new OpenAI({ baseURL: '${base}', apiKey: '${key}' })\nconst resp = await client.chat.completions.create({\n  model: '${m}',\n  messages: [{ role: 'user', content: 'Hello' }],\n})\nconsole.log(resp.choices[0].message.content)`
    default:
      return `# PowerShell\n$env:OPENAI_BASE_URL = "${base}"\n$env:OPENAI_API_KEY = "${key}"\n\n# bash / zsh\nexport OPENAI_BASE_URL="${base}"\nexport OPENAI_API_KEY="${key}"`
  }
})

async function copy() {
  try {
    await navigator.clipboard.writeText(snippet.value)
    toast.success(t('router.connect.copied'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  }
}
</script>
