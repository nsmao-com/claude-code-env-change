<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { useConfigStore } from '@/stores/configStore'
import { useConfirm } from '@/composables/useConfirm'
import type { ModelProfile, WorkbenchConfig, Result } from '@/types/workbench'
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
    <label
      >{{ tx('环境配置', 'Environment')
      }}<select v-model="selected" :disabled="busy">
        <option value="" disabled>{{ tx('选择环境', 'Select environment') }}</option>
        <option v-for="e in envs" :key="`${e.provider}/${e.name}`" :value="`${e.provider}/${e.name}`">
          {{ e.provider }} · {{ e.name }}
        </option>
      </select></label
    >
    <div v-if="!envs.length" class="wb-empty mt-4">
      {{ tx('请先在环境页添加 API 配置', 'Add an API environment first') }}
    </div>
    <form v-else @submit.prevent="save" class="mt-4">
      <div class="wb-row">
        <button type="button" :disabled="busy || !env" @click="discover">
          {{ tx('获取模型列表', 'Discover models') }}</button
        ><span class="text-xs text-muted-foreground">{{ models.length }} {{ tx('个模型', 'models') }}</span>
      </div>
      <label
        >{{ tx('模型名称', 'Model name')
        }}<input v-model="form.model" list="discovered-models" required placeholder="gpt-5.4" /><datalist
          id="discovered-models"
        >
          <option v-for="m in models" :key="m.id" :value="m.id">{{ m.name }}</option>
        </datalist></label
      >
      <div class="wb-grid mt-4">
        <label v-for="field in fields" :key="field.key"
          >{{ tx(field.zh, field.en)
          }}<input
            v-model.number="form[field.key]"
            type="number"
            min="0"
            :step="field.key.includes('price') ? '0.001' : '1'"
        /></label>
      </div>
      <div class="wb-row mt-4">
        <button class="primary" :disabled="busy || !env">{{ tx('保存档案', 'Save profile') }}</button
        ><button type="button" :disabled="busy || !env || !form.model" @click="test">
          {{ tx('测试模型（可能计费）', 'Test model (may incur cost)') }}
        </button>
      </div>
    </form>
    <p v-if="result" class="wb-hint">{{ result }}</p>
    <div v-if="env?.provider === 'codex'" class="wb-row">
      <span class="text-xs"
        >{{ tx('Codex 认证', 'Codex auth') }} ·
        {{ env.variables.AI_ENV_AUTH_MODE || tx('兼容模式', 'Legacy') }}</span
      ><button :disabled="busy" @click="auth('mixed')">
        {{ tx('API + 保留官方登录', 'API + keep official login') }}</button
      ><button :disabled="busy" @click="auth('api')">{{ tx('API 专用', 'API only') }}</button>
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
                <button :disabled="busy" @click="Object.assign(form, p)">{{ tx('编辑', 'Edit') }}</button
                ><button :disabled="busy" @click="apply(p)">{{ tx('应用', 'Apply') }}</button
                ><button :disabled="busy" @click="remove(p)">{{ tx('删除', 'Delete') }}</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
