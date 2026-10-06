<template>
  <Card class="mb-4">
    <CardContent>
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0">
          <Label class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{{ tx('可观测性导出', 'Observability export') }}</Label>
          <p class="mt-1 text-xs leading-relaxed text-muted-foreground">
            {{
              tx(
                '把每次网关请求（模型、上游、Token、耗时、费用估算）以 OTLP Trace 发到 Langfuse 或任意 OpenTelemetry Collector。后台批量发送，不影响请求。',
                'Sends each gateway request (model, upstream, tokens, latency, estimated cost) as an OTLP trace to Langfuse or any OpenTelemetry collector, in the background.',
              )
            }}
          </p>
        </div>
        <Switch :checked="form.enabled" :disabled="busy" :aria-label="tx('启用导出', 'Enable export')" @update:checked="toggle" />
      </div>

      <div class="mt-4 space-y-3">
        <SegmentedPills v-model="mode" layout-id="otel-mode" dense :items="modes" />
        <template v-if="mode === 'langfuse'">
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <AppInput v-model="form.langfuse_host" :label="tx('Langfuse 地址', 'Langfuse host')" placeholder="https://cloud.langfuse.com" />
            <AppInput v-model="form.langfuse_public_key" label="Public Key" placeholder="pk-lf-…" />
            <AppInput v-model="form.langfuse_secret_key" type="password" label="Secret Key" placeholder="sk-lf-…" />
          </div>
        </template>
        <template v-else>
          <AppInput
            v-model="form.endpoint"
            :label="tx('Collector 地址（自动追加 /v1/traces）', 'Collector URL (/v1/traces is appended)')"
            placeholder="http://localhost:4318"
          />
          <div>
            <div class="flex items-center justify-between">
              <Label class="text-xs">{{ tx('请求头（可选）', 'Headers (optional)') }}</Label>
              <Button variant="ghost" size="sm" class="h-7 px-2 text-xs" type="button" @click="headers.push({ key: '', value: '' })">
                <Plus class="size-3.5" />{{ tx('添加', 'Add') }}
              </Button>
            </div>
            <div v-for="(h, i) in headers" :key="i" class="mt-2 flex items-center gap-2">
              <Input v-model="h.key" class="h-8 w-48 font-mono text-xs" placeholder="Authorization" :aria-label="tx('请求头名称', 'Header name')" />
              <Input v-model="h.value" class="h-8 flex-1 font-mono text-xs" type="password" placeholder="Bearer …" :aria-label="tx('请求头值', 'Header value')" />
              <Button variant="ghost" size="icon-sm" type="button" :aria-label="tx('删除', 'Remove')" @click="headers.splice(i, 1)">
                <X />
              </Button>
            </div>
          </div>
        </template>
        <label class="flex items-start gap-2 text-xs">
          <Checkbox :model-value="form.bodies" class="mt-0.5" @update:model-value="form.bodies = $event === true" />
          <span>
            <span class="font-medium">{{ tx('附带请求与响应内容', 'Include request and response bodies') }}</span>
            <span class="block text-muted-foreground">{{ tx('每条最多 256 KB，Key 与 Token 会被遮盖；内容包含提示词，请确认接收方可信。', 'Up to 256 KB each with keys masked. Bodies contain prompts, so only send them somewhere you trust.') }}</span>
          </span>
        </label>
        <div class="flex flex-wrap items-center gap-2">
          <Button size="sm" variant="outline" type="button" :disabled="busy" @click="test">{{ tx('发送测试', 'Send test') }}</Button>
          <Button size="sm" type="button" :disabled="busy" @click="save()">{{ tx('保存', 'Save') }}</Button>
          <span v-if="status" class="text-xs text-muted-foreground">
            {{ tx(`已发送 ${status.sent} 条`, `${status.sent} sent`) }}
            <span v-if="status.last_error" class="text-destructive"> · {{ status.last_error }}</span>
          </span>
        </div>
      </div>
    </CardContent>
  </Card>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { Plus, X } from '@lucide/vue'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { useToast } from '@/composables/useToast'
import AppInput from '@/components/common/AppInput.vue'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

interface OtelSettings {
  enabled: boolean
  mode?: string
  endpoint?: string
  headers?: Record<string, string>
  langfuse_host?: string
  langfuse_public_key?: string
  langfuse_secret_key?: string
  bodies?: boolean
}
interface OtelStatus {
  settings: OtelSettings
  sent: number
  last_error?: string
}

const { tx } = useWorkbench()
const toast = useToast()
const form = reactive<Required<Omit<OtelSettings, 'headers'>>>({
  enabled: false,
  mode: 'langfuse',
  endpoint: '',
  langfuse_host: 'https://cloud.langfuse.com',
  langfuse_public_key: '',
  langfuse_secret_key: '',
  bodies: false,
})
const headers = ref<{ key: string; value: string }[]>([])
const status = ref<OtelStatus | null>(null)
const busy = ref(false)
const mode = computed({ get: () => form.mode, set: (v: string) => (form.mode = v) })
const modes = computed(() => [
  { value: 'langfuse', label: 'Langfuse' },
  { value: 'otlp', label: tx('OTLP Collector', 'OTLP collector') },
])

function payload(enabled = form.enabled): OtelSettings {
  const h: Record<string, string> = {}
  for (const row of headers.value) if (row.key.trim()) h[row.key.trim()] = row.value
  return { ...form, enabled, headers: h }
}

async function load() {
  try {
    status.value = await workbench<OtelStatus>('GetOtelStatus')
    const s = status.value.settings
    Object.assign(form, {
      enabled: !!s.enabled,
      mode: s.mode || 'langfuse',
      endpoint: s.endpoint || '',
      langfuse_host: s.langfuse_host || 'https://cloud.langfuse.com',
      langfuse_public_key: s.langfuse_public_key || '',
      langfuse_secret_key: s.langfuse_secret_key || '',
      bodies: !!s.bodies,
    })
    headers.value = Object.entries(s.headers || {}).map(([key, value]) => ({ key, value }))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  }
}
void load()

async function save(enabled = form.enabled): Promise<boolean> {
  busy.value = true
  try {
    await workbench('SaveOtelSettings', payload(enabled))
    form.enabled = enabled
    toast.success(tx('导出设置已保存', 'Export settings saved'))
    status.value = await workbench<OtelStatus>('GetOtelStatus')
    return true
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
    return false
  } finally {
    busy.value = false
  }
}

async function toggle(on: boolean) {
  await save(on)
}

async function test() {
  busy.value = true
  try {
    await workbench('TestOtelExport', payload(true))
    toast.success(tx('测试 span 已发送，请到接收端查看', 'Test span sent; check your collector'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    busy.value = false
  }
}
</script>
