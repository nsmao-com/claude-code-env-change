<script setup lang="ts">
import { CheckboxGroupRoot } from 'reka-ui'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ref, reactive, watch, computed } from 'vue'
import ModelCombobox from './ModelCombobox.vue'
import { callService } from '@/services/appBridge'
import { pendingImportLink } from '@/lib/deepLink'
import { configBaseUrl } from '@/lib/configUrl'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { useConfigStore } from '@/stores/configStore'
import { useConfirm } from '@/composables/useConfirm'
import type { EnvConfig } from '@/types'
const { tx, busy, error, run } = useWorkbench(),
  config = useConfigStore(),
  confirm = useConfirm()
const link = ref(''),
  imports = ref<EnvConfig[]>([]),
  selected = ref<number[]>([]),
  message = ref(''),
  fromLink = ref(false)
const universal = reactive({
  name: '',
  base_url: '',
  api_key: '',
  model: '',
  format: 'chat_completions',
  providers: ['claude', 'codex'],
})
const tools = [
  { id: 'claude', name: 'Claude Code' },
  { id: 'codex', name: 'Codex' },
  { id: 'antigravity', name: 'Gemini' },
  { id: 'opencode', name: 'OpenCode' },
  { id: 'grok', name: 'Grok' },
  { id: 'claude_desktop', name: 'Claude Desktop' },
]
async function preview(text: string) {
  imports.value = await workbench('PreviewExternalImport', text)
  selected.value = imports.value.map((_, i) => i)
}
function hostOf(item: EnvConfig) {
  try {
    return new URL(configBaseUrl(item)).host
  } catch {
    return configBaseUrl(item) || '—'
  }
}
watch(
  pendingImportLink,
  (value) => {
    if (!value) return
    link.value = value
    pendingImportLink.value = ''
    fromLink.value = true
    message.value = ''
    void run(() => preview(value))
  },
  { immediate: true },
)
async function file() {
  fromLink.value = false
  await run(async () => {
    const path = await workbench<string>('PickImportFile', 'config')
    if (!path) return
    await preview(await workbench<string>('ReadExternalImport', path))
  })
}
async function commit() {
  await run(async () => {
    const count = await workbench<number>(
      'CommitExternalImport',
      selected.value.map((i) => imports.value[i]),
    )
    message.value = tx(
      `已导入 ${count} 个环境，同名配置已自动重命名。`,
      `Imported ${count} environments. Name collisions were renamed.`,
    )
    imports.value = []
    link.value = ''
    fromLink.value = false
    await config.loadConfig()
  })
}
// 接入方式：普通 API，或用本机已登录的订阅（经本机网关转发）
interface AccountUpstream {
  kind: string
  available: boolean
  account?: string
  message?: string
}
const access = ref<'api' | 'codex' | 'copilot' | 'claude'>('api')
const accounts = ref<AccountUpstream[]>([])
const accountModels = ref<{ id: string; name: string }[]>([])
const accountInfo = computed(() => accounts.value.find((a) => a.kind === access.value))
const accessValue = computed({
  get: () => access.value,
  set: (v: string) => {
    access.value = v as typeof access.value
    accountModels.value = []
    if (v === 'api') {
      if (universal.base_url.startsWith('account://')) universal.base_url = ''
      return
    }
    universal.base_url = `account://${v}`
    universal.api_key = ''
    universal.format = v === 'codex' ? 'responses' : v === 'claude' ? 'anthropic_messages' : 'chat_completions'
  },
})
void callService<AccountUpstream[]>('RouterService', 'GetAccountUpstreams')
  .then((list) => (accounts.value = list || []))
  .catch(() => {})
async function loadAccountModels() {
  await run(async () => {
    const ids = await callService<string[]>('RouterService', 'ListAccountModels', access.value)
    accountModels.value = (ids || []).map((id) => ({ id, name: id }))
  })
}
async function saveUniversal() {
  if (
    !(await confirm.show(
      tx('保存通用供应商', 'Save universal provider'),
      tx(
        '将在所选工具中创建关联配置；已有同名关联配置会更新，应用请到环境页操作。',
        'Creates linked environments in the selected tools and updates existing linked entries. Apply them from Environments.',
      ),
    ))
  )
    return
  await run(async () => {
    const count = await workbench<number>('SaveUniversalProvider', { ...universal })
    message.value = tx(`已同步 ${count} 个环境配置。`, `Updated ${count} environments.`)
    universal.api_key = ''
    await config.loadConfig()
  })
}
</script>
<template>
  <p v-if="error" role="alert" class="wb-error">{{ error }}</p>
  <p v-if="message" class="wb-hint">{{ message }}</p>
  <section class="wb-card">
    <h2>{{ tx('从 CC Switch 导入', 'Import from CC Switch') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '支持 CC Switch JSON 导出、CC Switch / aienv:// 供应商分享链接与 AI ENV 配置文件。预览显示名称、工具和请求地址，密钥不会展示。',
          'Supports CC Switch JSON exports, CC Switch / aienv:// provider share links and AI ENV configuration files. The preview shows names, tools and hosts, never keys.',
        )
      }}
    </p>
    <div class="wb-row">
      <Button variant="outline" type="button" :disabled="busy" @click="file">
        {{ tx('选择配置文件', 'Choose configuration file') }}
      </Button>
    </div>
    <form @submit.prevent="run(() => preview(link))">
      <label
        >{{ tx('粘贴分享链接', 'Paste a share link')
        }}
        <Input
          v-model="link"
          type="password"
          autocomplete="off"
          placeholder="aienv://import?…  /  ccswitch://v1/import?…"
          required
        /></label
      >
      <Button variant="outline" type="submit" class="mt-3" :disabled="busy || !link">
        {{ tx('预览导入', 'Preview import') }}
      </Button>
    </form>
    <CheckboxGroupRoot
      v-if="imports.length"
      v-model="selected"
      :disabled="busy"
      :roving-focus="false"
      class="mt-5"
    >
      <p v-if="fromLink" role="alert" class="wb-error">
        {{
          tx(
            '这些配置来自一个 aienv:// 链接。导入后 API Key 与请求内容会发往下方列出的地址，请确认来源可信再导入；不导入则不会保存任何内容。',
            'These come from an aienv:// link. After importing, your prompts and the API key go to the hosts listed below. Import only if you trust the source; nothing is saved otherwise.',
          )
        }}
      </p>
      <label v-for="(item, i) in imports" :key="i" class="wb-check mb-3">
        <Checkbox :value="i" />
        <strong>{{ item.name }}</strong>
        <span class="text-muted-foreground">{{ item.provider }}</span>
        <span class="font-mono text-xs text-muted-foreground">{{ hostOf(item) }}</span>
      </label>
      <Button variant="default" type="button" :disabled="busy || !selected.length" @click="commit">
        {{ tx('导入所选配置', 'Import selected') }}
        (
        {{ selected.length }}
        )
      </Button>
    </CheckboxGroupRoot>
  </section>
  <section class="wb-card">
    <h2>{{ tx('通用供应商', 'Universal provider') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '填写一次连接信息，生成多工具关联配置。跨协议调用需在路由页开启对应工具网关。',
          'Enter connection details once to create linked tool environments. Enable the tool gateway on the Routing page for protocol conversion.',
        )
      }}
    </p>
    <form @submit.prevent="saveUniversal">
      <div class="wb-grid">
        <label
          >{{ tx('供应商名称', 'Provider name')
          }}
          <Input v-model="universal.name" required maxlength="100" /></label
        ><label
          >{{ tx('接入方式', 'Access') }}
          <Select v-model="accessValue" :disabled="busy">
            <SelectTrigger class="h-9 w-full min-w-0">
              <SelectValue />
            </SelectTrigger>
            <SelectContent position="popper" align="start">
              <SelectItem value="api">{{ tx('API 地址 + Key', 'API URL + key') }}</SelectItem>
              <SelectItem value="codex">{{ tx('ChatGPT 订阅（Codex 登录）', 'ChatGPT subscription (Codex sign-in)') }}</SelectItem>
              <SelectItem value="copilot">{{ tx('GitHub Copilot 订阅', 'GitHub Copilot subscription') }}</SelectItem>
              <SelectItem value="claude">{{ tx('Claude 订阅（本机 Claude Code）', 'Claude subscription (local Claude Code)') }}</SelectItem>
            </SelectContent>
          </Select></label
        ><template v-if="access === 'api'"><label
          >Base URL
          <Input v-model="universal.base_url" type="url" placeholder="https://api.example.com/v1" required /></label
        ><label
          >API Key
          <Input v-model="universal.api_key" type="password" autocomplete="off" required /></label
        ></template><label>{{ tx('默认模型', 'Default model') }}
          <Input v-if="access === 'api'" v-model="universal.model" required />
          <ModelCombobox v-else v-model="universal.model" :models="accountModels" :disabled="busy" /></label
        ><label
          >{{ tx('上游协议', 'Upstream protocol')
          }}
          <Select v-model="universal.format" :disabled="busy || access === 'codex' || access === 'claude'">
            <SelectTrigger class="h-9 w-full min-w-0">
              <SelectValue />
            </SelectTrigger>
            <SelectContent position="popper" align="start">
              <SelectItem value="chat_completions">OpenAI Chat Completions</SelectItem>
              <SelectItem value="responses">OpenAI Responses</SelectItem>
              <SelectItem value="anthropic_messages">Anthropic Messages</SelectItem>
            </SelectContent>
          </Select></label
        >
      </div>
      <div v-if="access !== 'api'" class="mt-4 space-y-2 rounded-lg bg-muted px-3 py-2 text-xs">
        <p v-if="accountInfo?.available">
          {{ tx('已检测到登录', 'Signed in') }}{{ accountInfo.account ? `：${accountInfo.account}` : '' }}
          <Button variant="outline" size="sm" type="button" class="ml-2 h-7" :disabled="busy" @click="loadAccountModels">
            {{ tx('读取可用模型', 'Load models') }}
          </Button>
          <span v-if="accountModels.length" class="ml-2 text-muted-foreground">{{ accountModels.length }} {{ tx('个模型', 'models') }}</span>
        </p>
        <p v-else class="text-destructive">{{ accountInfo?.message || tx('未检测到该订阅的登录', 'No sign-in found for this subscription') }}</p>
        <p v-if="access === 'claude'" class="text-muted-foreground">
          {{
            tx(
              '每个请求由本机 claude 命令行生成（保持官方客户端身份），调用方的工具经 MCP 接入；同一对话的后续轮次复用同一进程，前文可命中缓存。需要本机已安装 Claude Code 并用订阅登录。',
              'Each request is generated by the local claude CLI (keeping the official client identity); the caller’s tools are bridged over MCP, and later turns of a conversation reuse the same process so earlier context hits the cache. Needs Claude Code installed and signed in with a subscription.',
            )
          }}
        </p>
        <p class="text-muted-foreground">
          {{
            tx(
              '请求经本机网关、用本机已登录的订阅鉴权，应用环境时会自动打开该工具的应用路由。这属于在官方客户端之外使用订阅，供应商可能视为违反条款并限制账号，请自行评估风险。',
              'Requests go through the local gateway using the subscription signed in on this computer; applying the environment turns on routing for that tool. Using a subscription outside its official client may break the provider’s terms and get the account limited.',
            )
          }}
        </p>
      </div>
      <CheckboxGroupRoot
        v-model="universal.providers"
        :disabled="busy"
        :roving-focus="false"
        class="wb-row mt-4"
      >
        <label class="wb-check" v-for="tool in tools" :key="tool.id">
          <Checkbox :value="tool.id" />
          {{ tool.name }}
        </label>
      </CheckboxGroupRoot>
      <Button variant="default" type="submit" :disabled="busy || !universal.providers.length">
        {{ tx('保存到所选工具', 'Save to selected tools') }}
      </Button>
    </form>
  </section>
</template>
