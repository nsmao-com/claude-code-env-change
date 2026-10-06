<script setup lang="ts">
import { Button } from '@/components/ui/button'
import ModelCombobox from './ModelCombobox.vue'
import WorkbenchNumberInput from './WorkbenchNumberInput.vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ref, reactive, computed, watch } from 'vue'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { useConfigStore } from '@/stores/configStore'
import { useConfirm } from '@/composables/useConfirm'
import type { ModelProfile, WorkbenchConfig, Result, CatalogModel, CatalogStatus } from '@/types/workbench'
const { tx, busy, error, run } = useWorkbench()
const config = useConfigStore(),
  confirm = useConfirm()
const selected = ref(''),
  models = ref<{ id: string; name: string }[]>([]),
  profiles = ref<ModelProfile[]>([]),
  result = ref('')
const envs = computed(() => config.environments.filter((e) => !e.official_login))
const env = computed(() => envs.value.find((e) => `${e.provider}/${e.name}` === selected.value))
const initial = (): ModelProfile => ({
  provider: '',
  environment: '',
  model: '',
  context: 0,
  output: 0,
  compact: 0,
  input_price: 0,
  output_price: 0,
  cache_read_price: 0,
  cache_write_price: 0,
})
const form = reactive(initial())
const fields = [
  { key: 'context', zh: '上下文窗口', en: 'Context window' },
  { key: 'output', zh: '最大输出', en: 'Max output' },
  { key: 'compact', zh: '自动压缩阈值', en: 'Auto compact limit' },
  { key: 'input_price', zh: '输入价格 / 百万 Token', en: 'Input / million tokens' },
  { key: 'output_price', zh: '输出价格 / 百万 Token', en: 'Output / million tokens' },
  { key: 'cache_read_price', zh: '缓存读取价格', en: 'Cache read price' },
  { key: 'cache_write_price', zh: '缓存写入价格', en: 'Cache write price' },
] as const
const filtered = computed(() =>
  profiles.value.filter((p) => `${p.provider}/${p.environment}` === selected.value),
)
watch(selected, () => {
  Object.assign(form, initial())
  models.value = []
  result.value = ''
})
// models.dev 目录：按模型名查参考上限与公开价格
const catalog = ref<CatalogStatus | null>(null)
const syncing = ref(false)
const reference = ref<CatalogModel | null>(null)
let lookupTimer: ReturnType<typeof setTimeout>
watch(
  () => form.model,
  (name) => {
    clearTimeout(lookupTimer)
    reference.value = null
    if (!name.trim()) return
    lookupTimer = setTimeout(async () => {
      try {
        reference.value = (await workbench<CatalogModel | null>('LookupModelCatalog', name)) || null
      } catch {
        reference.value = null
      }
    }, 300)
  },
)
function compactTokens(n?: number) {
  if (!n) return '—'
  return n >= 1e6 ? `${+(n / 1e6).toFixed(2)}M` : n >= 1e3 ? `${Math.round(n / 1e3)}K` : String(n)
}
function fillFromCatalog() {
  const r = reference.value
  if (!r) return
  if (r.context) form.context = r.context
  if (r.output) form.output = r.output
  if (r.priced) {
    form.input_price = r.input || 0
    form.output_price = r.output_cost || 0
    form.cache_read_price = r.cache_read || 0
    form.cache_write_price = r.cache_write || 0
  }
}
async function syncCatalog() {
  if (syncing.value) return
  syncing.value = true
  try {
    catalog.value = await workbench<CatalogStatus>('SyncModelCatalog')
    if (form.model) reference.value = (await workbench<CatalogModel | null>('LookupModelCatalog', form.model)) || null
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    syncing.value = false
  }
}
async function load() {
  profiles.value = (await workbench<WorkbenchConfig>('GetWorkbench')).models || []
}
async function discover() {
  if (env.value)
    await run(async () => {
      models.value = await workbench('DiscoverModels', env.value!.provider, env.value!.name)
    })
}
async function save() {
  if (!env.value) return
  await run(
    async () => {
      await workbench('SaveModelProfile', {
        ...form,
        provider: env.value!.provider,
        environment: env.value!.name,
      })
      await load()
    },
    tx('模型档案已保存', 'Model profile saved'),
  )
}
async function test() {
  if (!env.value || !form.model) return
  if (
    !(await confirm.show(
      tx('测试模型', 'Test model'),
      tx(
        '将发送一次最小模型请求，供应商可能计费。',
        'Send one minimal model request. Your provider may charge for it.',
      ),
    ))
  )
    return
  await run(async () => {
    const r = await workbench<Result>('TestModel', env.value!.provider, env.value!.name, form.model)
    result.value = `${r.message} · ${r.latency} ms`
    if (!r.success) throw new Error(r.message)
  })
}
async function apply(p: ModelProfile) {
  await run(
    async () => {
      await workbench('ApplyModelProfile', p.provider, p.environment, p.model)
      await config.loadConfig()
    },
    tx('模型已应用', 'Model applied'),
  )
}
async function remove(p: ModelProfile) {
  if (!(await confirm.show(tx('删除档案', 'Delete profile'), p.model, 'warning'))) return
  await run(async () => {
    await workbench('DeleteModelProfile', p.provider, p.environment, p.model)
    await load()
  })
}
async function auth(mode: string) {
  if (!env.value) return
  if (
    !(await confirm.show(
      tx('更改 Codex 认证方式', 'Change Codex authentication'),
      mode === 'api'
        ? tx(
            'API 专用模式会备份并移除本机 auth.json。',
            'API-only mode backs up and removes the local auth.json.',
          )
        : tx(
            '保留官方 OAuth 登录，同时将 API Key 放在供应商配置中。',
            'Keep official OAuth login and store the API key in the provider configuration.',
          ),
      'warning',
    ))
  )
    return
  await run(
    async () => {
      await workbench('SetCodexAuthMode', env.value!.name, mode)
      await config.loadConfig()
    },
    tx('认证方式已保存', 'Authentication mode saved'),
  )
}
void workbench<CatalogStatus>('GetModelCatalogStatus').then((s) => (catalog.value = s)).catch(() => {})
void run(async () => {
  await config.loadConfig()
  selected.value = envs.value[0] ? `${envs.value[0].provider}/${envs.value[0].name}` : ''
  await load()
})
</script>
<template>
  <p v-if="error" role="alert" class="wb-error">{{ error }}</p>
  <div class="wb-card">
    <h2>{{ tx('模型目录与档案', 'Model catalog & profiles') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '按环境保存模型参数与美元定价。0 表示使用工具默认值；应用到 Codex 时生成模型目录。',
          'Save model limits and USD pricing per environment. Zero uses tool defaults. Applying to Codex generates a model catalog.',
        )
      }}
    </p>
    <div class="wb-row text-xs text-muted-foreground">
      <span v-if="catalog?.count">
        {{
          tx(
            `models.dev 目录：${catalog.count} 个模型，更新于 ${new Date(catalog.updated_at).toLocaleDateString()}`,
            `models.dev catalog: ${catalog.count} models, updated ${new Date(catalog.updated_at).toLocaleDateString()}`,
          )
        }}
      </span>
      <span v-else>{{ tx('还没有 models.dev 目录，同步后可一键填入上限与公开价格。', 'No models.dev catalog yet. Sync it to fill limits and list prices in one click.') }}</span>
      <Button variant="ghost" size="sm" type="button" :disabled="syncing" @click="syncCatalog">
        {{ syncing ? tx('同步中…', 'Syncing…') : tx('同步目录', 'Sync catalog') }}
      </Button>
    </div>
    <label
      >{{ tx('环境配置', 'Environment')
      }}
      <Select v-model="selected" :disabled="busy">
        <SelectTrigger class="h-9 w-full min-w-0">
          <SelectValue :placeholder="tx('选择环境', 'Select environment')" />
        </SelectTrigger>
        <SelectContent position="popper" align="start">
          <SelectItem v-for="e in envs" :key="`${e.provider}/${e.name}`" :value="`${e.provider}/${e.name}`">
            {{ e.provider }}
            ·
            {{ e.name }}
          </SelectItem>
        </SelectContent>
      </Select></label
    >
    <div v-if="!envs.length" class="wb-empty mt-4">
      {{ tx('请先在环境页添加 API 配置', 'Add an API environment first') }}
    </div>
    <form v-else @submit.prevent="save" class="mt-4">
      <div class="wb-row">
        <Button variant="outline" type="button" :disabled="busy || !env" @click="discover">
          {{ tx('获取模型列表', 'Discover models') }}
        </Button>
          <span class="text-xs text-muted-foreground">{{ models.length }} {{ tx('个模型', 'models') }}</span>
      </div>
      <label
        >{{ tx('模型名称', 'Model name')
        }}
        <ModelCombobox v-model="form.model" :models="models" :disabled="busy" /></label
      >
      <div v-if="reference" class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg bg-muted px-3 py-2 text-xs">
        <span class="font-medium">{{ tx('models.dev 参考', 'models.dev reference') }} · {{ reference.name || reference.id }} ({{ reference.maker }})</span>
        <span>{{ tx('上下文', 'Context') }} {{ compactTokens(reference.context) }}</span>
        <span>{{ tx('输出', 'Output') }} {{ compactTokens(reference.output) }}</span>
        <span v-if="reference.priced">
          ${{ reference.input }} / ${{ reference.output_cost }}
          <template v-if="reference.cache_read || reference.cache_write">
            · {{ tx('缓存', 'cache') }} ${{ reference.cache_read || 0 }} / ${{ reference.cache_write || 0 }}
          </template>
        </span>
        <Button variant="outline" size="sm" type="button" class="ml-auto h-7" :disabled="busy" @click="fillFromCatalog">
          {{ tx('填入这些数值', 'Fill these values') }}
        </Button>
        <span class="basis-full text-muted-foreground">
          {{ tx('价格为厂商官方价；中转站请按实际倍率调整。', 'Prices are the maker’s list prices; adjust for your relay’s rate.') }}
        </span>
      </div>
      <div class="wb-grid mt-4">
        <label v-for="field in fields" :key="field.key"
          >{{ tx(field.zh, field.en)
          }}
          <WorkbenchNumberInput
            v-model="form[field.key]"
            :disabled="busy"
            :min="0"
            :step="field.key.includes('price') ? 0.001 : 1"
          /></label>
      </div>
      <div class="wb-row mt-4">
        <Button variant="default" type="submit" :disabled="busy || !env">
          {{ tx('保存档案', 'Save profile') }}
        </Button>
        <Button variant="outline" type="button" :disabled="busy || !env || !form.model" @click="test">
          {{ tx('测试模型（可能计费）', 'Test model (may incur cost)') }}
        </Button>
      </div>
    </form>
    <p v-if="result" class="wb-hint">{{ result }}</p>
    <div v-if="env?.provider === 'codex'" class="wb-row">
      <span class="text-xs"
        >{{ tx('Codex 认证', 'Codex auth') }} ·
        {{ env.variables.AI_ENV_AUTH_MODE || tx('兼容模式', 'Legacy') }}</span
      >
      <Button variant="outline" type="button" :disabled="busy" @click="auth('mixed')">
        {{ tx('API + 保留官方登录', 'API + keep official login') }}
      </Button>
        <Button variant="outline" type="button" :disabled="busy" @click="auth('api')">
        {{ tx('API 专用', 'API only') }}
      </Button>
    </div>
  </div>
  <div class="wb-card">
    <h2>{{ tx('已保存模型', 'Saved models') }}</h2>
    <div class="wb-empty" v-if="!filtered.length">
      {{ tx('当前环境还没有模型档案', 'No model profiles for this environment') }}
    </div>
    <div class="wb-table" v-else>
      <table>
        <thead>
          <tr>
            <th>{{ tx('模型', 'Model') }}</th>
            <th>{{ tx('上下文 / 输出', 'Context / output') }}</th>
            <th>{{ tx('操作', 'Actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in filtered" :key="p.model">
            <td>{{ p.model }}</td>
            <td>{{ p.context || '—' }} / {{ p.output || '—' }}</td>
            <td>
              <div class="wb-row mb-0">
                <Button variant="outline" type="button" :disabled="busy" @click="Object.assign(form, p)">
                  {{ tx('编辑', 'Edit') }}
                </Button>
                <Button variant="outline" type="button" :disabled="busy" @click="apply(p)">
                  {{ tx('应用', 'Apply') }}
                </Button>
                <Button variant="outline" type="button" :disabled="busy" @click="remove(p)">
                  {{ tx('删除', 'Delete') }}
                </Button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
