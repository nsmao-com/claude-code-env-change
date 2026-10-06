<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Switch } from '@/components/ui/switch'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import WorkbenchNumberInput from './WorkbenchNumberInput.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Loader2, RefreshCw, EyeOff } from '@lucide/vue'
import { ref, reactive, computed, watch } from 'vue'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'
import { callService } from '@/services/appBridge'
import type {
  BalanceCard,
  BalanceSource,
  QuotaSettings,
  QuotaWindow,
  WarmupSettings,
  WarmupStatus,
  SubscriptionQuota,
  WorkbenchConfig,
} from '@/types/workbench'

const { tx } = useWorkbench()
const config = useConfigStore()
const toast = useToast()
const quota = <T,>(name: string, ...args: unknown[]) => callService<T>('QuotaService', name, ...args)

const subscriptions = ref<SubscriptionQuota[]>([])
const balances = ref<BalanceCard[]>([])
const loadingSubs = ref(false)
const loadingBalances = ref(false)
const subsError = ref('')
const balancesError = ref('')

const MASK_KEY = 'aienv.quota.maskAccounts'
function readMask() {
  try {
    return localStorage.getItem(MASK_KEY) === '1'
  } catch {
    return false
  }
}
const maskAccounts = ref(readMask())
watch(maskAccounts, (v) => {
  try {
    localStorage.setItem(MASK_KEY, v ? '1' : '0')
  } catch {
    /* 本地存储不可用时只在本次会话生效 */
  }
})

const settings = reactive<QuotaSettings>({
  alert_percent: 80,
  monitor_minutes: 0,
  desktop_notify: true,
  warmup: { claude: false, codex: false, times: [], on_reset: false },
})
const warmStatus = ref<WarmupStatus[]>([])
const warming = ref('')
async function loadWarmStatus() {
  try {
    warmStatus.value = (await quota<WarmupStatus[]>('GetWarmupStatus')) || []
  } catch {
    /* 忽略 */
  }
}
function warmText(provider: string) {
  const st = warmStatus.value.find((s) => s.provider === provider)
  if (!st?.last_run) return ''
  const at = new Date(st.last_run).toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  return st.error ? tx(`上次 ${at} 失败：${st.error}`, `Last ${at} failed: ${st.error}`) : tx(`上次 ${at} 成功`, `Last ${at} succeeded`)
}
function addWarmTime() {
  const times = settings.warmup.times || (settings.warmup.times = [])
  if (times.length < 6) times.push('07:00')
}
async function warmNow(provider: string) {
  if (warming.value) return
  warming.value = provider
  try {
    await quota('WarmupNow', provider)
    toast.success(tx('预热请求已发送', 'Warm-up request sent'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    warming.value = ''
    void loadWarmStatus()
  }
}
const savingSettings = ref(false)

async function loadSubscriptions(force: boolean) {
  loadingSubs.value = true
  subsError.value = ''
  try {
    subscriptions.value = (await quota<SubscriptionQuota[]>('GetSubscriptionQuotas', force)) || []
  } catch (e) {
    subsError.value = e instanceof Error ? e.message : String(e)
  } finally {
    loadingSubs.value = false
  }
}
async function loadBalances(force: boolean) {
  loadingBalances.value = true
  balancesError.value = ''
  try {
    balances.value = (await quota<BalanceCard[]>('GetBalances', force)) || []
  } catch (e) {
    balancesError.value = e instanceof Error ? e.message : String(e)
  } finally {
    loadingBalances.value = false
  }
}

function maskAccount(value?: string) {
  if (!value) return ''
  if (!maskAccounts.value) return value
  const [name, domain] = value.split('@')
  if (domain) return `${name.slice(0, 1)}***@${domain}`
  return `${value.slice(0, 1)}***`
}

function windowLabel(name: string) {
  const map: Record<string, [string, string]> = {
    '5h': ['5 小时', '5 hours'],
    '7d': ['7 天', '7 days'],
    '1d': ['1 天', '1 day'],
    Premium: ['高级请求', 'Premium requests'],
    Chat: ['对话', 'Chat'],
    Completions: ['代码补全', 'Completions'],
  }
  if (map[name]) return tx(...map[name])
  if (name.startsWith('7d · ')) return tx('7 天 · ', '7 days · ') + name.slice(5)
  const m = /^(\d+)([hdm])$/.exec(name)
  if (m) {
    const unit = { h: tx('小时', 'h'), d: tx('天', 'd'), m: tx('分钟', 'min') }[m[2] as 'h' | 'd' | 'm']
    return `${m[1]} ${unit}`
  }
  return name
}

function resetText(w: QuotaWindow) {
  if (!w.resets_at) return ''
  const ms = w.resets_at - Date.now()
  const at = new Date(w.resets_at)
  const stamp = at.toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  if (ms <= 0) return tx(`已于 ${stamp} 重置`, `Reset at ${stamp}`)
  const minutes = Math.round(ms / 60000)
  const d = Math.floor(minutes / 1440)
  const h = Math.floor((minutes % 1440) / 60)
  const m = minutes % 60
  const left = d > 0 ? tx(`${d} 天 ${h} 小时`, `${d}d ${h}h`) : h > 0 ? tx(`${h} 小时 ${m} 分`, `${h}h ${m}m`) : tx(`${m} 分钟`, `${m}m`)
  return tx(`${left}后重置 · ${stamp}`, `Resets in ${left} · ${stamp}`)
}

function barColor(used: number) {
  if (used >= 95) return 'var(--destructive)'
  if (settings.alert_percent > 0 && used >= settings.alert_percent) return 'oklch(0.72 0.16 70)'
  return 'var(--brand)'
}

function statusText(q: SubscriptionQuota) {
  switch (q.status) {
    case 'signed_out':
      return {
        claude: tx('未检测到 Claude 订阅登录。在终端运行 claude 并用 Pro / Max 账号登录后再刷新。', 'No Claude subscription sign-in found. Run claude in a terminal, sign in with Pro / Max, then refresh.'),
        codex: tx('未检测到 ChatGPT 登录。在终端运行 codex login 登录后再刷新。', 'No ChatGPT sign-in found. Run codex login, then refresh.'),
        copilot: tx('未检测到 GitHub Copilot 登录（VS Code / Copilot CLI）。', 'No GitHub Copilot sign-in found (VS Code / Copilot CLI).'),
      }[q.provider]
    case 'not_installed':
      return tx('未找到 claude 命令，可在「设置 → CLI」中安装。', 'claude command not found. Install it from Settings → CLI.')
    case 'expired':
      return tx('登录凭证已过期。打开一次对应 CLI（它会自行续期）后再刷新。', 'The sign-in has expired. Open the CLI once (it refreshes itself), then refresh.')
    case 'api_billing':
      return tx('Claude Code 当前按 API Key 计费，没有 5 小时 / 每周额度窗口。', 'Claude Code is billed by API key, so there are no 5-hour / weekly windows.')
    case 'pending':
      return tx('点击刷新读取：会在后台运行一次 claude /usage，与 CLI 自己显示的额度一致。', 'Refresh to read it: runs claude /usage once in the background, same figures as the CLI.')
    case 'error':
      return q.error || tx('读取失败', 'Failed to read')
  }
  return ''
}

function formatClock(ms?: number) {
  return ms ? new Date(ms).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' }) : ''
}

// ===== 余额接口配置 =====
const adapters = computed(() => [
  { value: 'auto', label: tx('自动识别（按 Base URL）', 'Auto (by Base URL)') },
  { value: 'deepseek', label: 'DeepSeek' },
  { value: 'moonshot', label: 'Moonshot / Kimi' },
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'siliconflow', label: 'SiliconFlow' },
  { value: 'stepfun', label: 'StepFun' },
  { value: 'aihubmix', label: 'AiHubMix' },
  { value: 'newapi', label: tx('New API 中转站（令牌余额）', 'New API relay (token balance)') },
  { value: 'oneapi', label: tx('One API 中转站（额度接口）', 'One API relay (billing API)') },
  { value: 'custom', label: tx('自定义接口', 'Custom endpoint') },
])
const keyedEnvs = computed(() => config.environments.filter((e) => !e.official_login))
const savedSources = ref<BalanceSource[]>([])
type SourceForm = BalanceSource & { divisor: number; alert_below: number }
const sourceForm = reactive<SourceForm>({
  provider: '',
  environment: '',
  adapter: 'auto',
  url: '',
  path: '',
  divisor: 0,
  currency: 'USD',
  alert_below: 0,
})
const selectedEnv = ref('')
const sourceBusy = ref(false)
const testResult = ref('')
const hasSaved = computed(() =>
  savedSources.value.some((s) => s.provider === sourceForm.provider && s.environment === sourceForm.environment),
)

watch(selectedEnv, (value) => {
  testResult.value = ''
  const env = keyedEnvs.value.find((e) => `${e.provider}/${e.name}` === value)
  if (!env) return
  const saved = savedSources.value.find((s) => s.provider === env.provider && s.environment === env.name)
  Object.assign(sourceForm, {
    provider: env.provider,
    environment: env.name,
    adapter: 'auto',
    url: '',
    path: '',
    divisor: 0,
    currency: 'USD',
    alert_below: 0,
    ...(saved || {}),
  })
})

function sourcePayload(): BalanceSource {
  const base: BalanceSource = {
    provider: sourceForm.provider,
    environment: sourceForm.environment,
    adapter: sourceForm.adapter,
    alert_below: sourceForm.alert_below || 0,
  }
  if (sourceForm.adapter === 'custom') {
    Object.assign(base, {
      url: sourceForm.url?.trim(),
      path: sourceForm.path?.trim(),
      divisor: sourceForm.divisor || 0,
      currency: sourceForm.currency || 'USD',
    })
  }
  return base
}

async function testSource() {
  if (!sourceForm.provider || sourceBusy.value) return
  sourceBusy.value = true
  testResult.value = ''
  try {
    const r = await quota<{ amount: number; currency: string; message: string }>('TestBalanceSource', sourcePayload())
    testResult.value = tx(`查询成功：${r.message}`, `OK: ${r.message}`)
  } catch (e) {
    testResult.value = ''
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    sourceBusy.value = false
  }
}

async function saveSource(remove = false) {
  if (!sourceForm.provider || sourceBusy.value) return
  sourceBusy.value = true
  try {
    const payload = remove ? { ...sourcePayload(), adapter: '' } : sourcePayload()
    await quota('SaveBalanceSource', payload)
    await reloadSources()
    toast.success(remove ? tx('已移除余额接口配置', 'Balance source removed') : tx('余额接口已保存', 'Balance source saved'))
    void loadBalances(true)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    sourceBusy.value = false
  }
}

async function reloadSources() {
  const wb = await workbench<WorkbenchConfig>('GetWorkbench')
  savedSources.value = wb.costs?.balance_sources || []
}

// ===== 提醒设置 =====
const alertOptions = [0, 70, 80, 90, 95]
const monitorOptions = [0, 15, 30, 60, 180]
const alertSelect = computed({
  get: () => String(settings.alert_percent),
  set: (v: string) => {
    settings.alert_percent = Number(v)
  },
})
const monitorSelect = computed({
  get: () => String(settings.monitor_minutes),
  set: (v: string) => {
    settings.monitor_minutes = Number(v)
  },
})
async function saveSettings() {
  if (savingSettings.value) return
  savingSettings.value = true
  try {
    await quota('SaveQuotaSettings', { ...settings })
    toast.success(tx('提醒设置已保存', 'Alert settings saved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    savingSettings.value = false
  }
}

async function init() {
  try {
    const loaded = await quota<QuotaSettings>('GetQuotaSettings')
    const warmDefaults: WarmupSettings = { claude: false, codex: false, times: [], on_reset: false }
    Object.assign(settings, loaded, { warmup: { ...warmDefaults, ...(loaded.warmup || {}) } })
  } catch {
    /* 使用默认值 */
  }
  void loadWarmStatus()
  void config.loadConfig().catch(() => {})
  void reloadSources().catch(() => {})
  void loadBalances(false)
  await loadSubscriptions(false)
}
void init()
</script>

<template>
  <section class="wb-card" aria-labelledby="quota-subs-title">
    <div class="wb-row justify-between !mb-1">
      <h2 id="quota-subs-title" class="!mb-0">{{ tx('订阅额度', 'Subscription allowances') }}</h2>
      <div class="flex items-center gap-3">
        <label class="wb-check text-xs text-muted-foreground">
          <Switch :checked="maskAccounts" size="sm" @update:checked="maskAccounts = $event" />
          <EyeOff class="size-3.5" aria-hidden="true" />{{ tx('隐藏账号', 'Hide accounts') }}
        </label>
        <Button variant="outline" size="sm" type="button" :disabled="loadingSubs" @click="loadSubscriptions(true)">
          <Loader2 v-if="loadingSubs" class="animate-spin" />
          <RefreshCw v-else />
          {{ loadingSubs ? tx('读取中…', 'Reading…') : tx('刷新', 'Refresh') }}
        </Button>
      </div>
    </div>
    <p class="wb-hint !mb-0">
      {{
        tx(
          '读取本机已登录的 Claude Code、Codex（ChatGPT）、GitHub Copilot 订阅的滚动额度窗口。只读取 CLI 自己保存的登录，不复制、不刷新令牌。',
          'Rolling allowance windows of the Claude Code, Codex (ChatGPT) and GitHub Copilot subscriptions signed in on this computer. Only reads the CLIs’ own sign-ins; tokens are never copied or refreshed.',
        )
      }}
    </p>
    <p v-if="subsError" role="alert" class="wb-error mt-3">{{ subsError }}</p>
    <div v-if="loadingSubs && !subscriptions.length" class="wb-grid mt-4" aria-busy="true">
      <div v-for="i in 3" :key="i" class="h-36 animate-pulse rounded-xl bg-muted" />
    </div>
    <div v-else class="wb-grid mt-4">
      <article v-for="q in subscriptions" :key="q.provider" class="rounded-xl border border-border p-4">
        <header class="flex items-start justify-between gap-2">
          <div class="flex min-w-0 items-center gap-2">
            <BrandIcon :provider="q.provider" class="size-4 shrink-0" />
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold">{{ q.name }}</p>
              <p v-if="q.account" class="truncate text-xs text-muted-foreground">{{ maskAccount(q.account) }}</p>
            </div>
          </div>
          <Badge v-if="q.plan" variant="secondary" class="shrink-0">{{ q.plan }}</Badge>
        </header>
        <div v-if="q.windows.length" class="mt-4 space-y-3">
          <div v-for="w in q.windows" :key="w.name">
            <div class="flex items-baseline justify-between gap-2 text-xs">
              <span class="font-medium">{{ windowLabel(w.name) }}</span>
              <span v-if="w.unlimited" class="text-muted-foreground">{{ tx('不限量', 'Unlimited') }}</span>
              <span v-else class="tabular-nums" :class="w.used >= 95 ? 'font-semibold text-destructive' : ''">
                {{ w.display ? `${w.display} · ` : '' }}{{ Math.round(w.used) }}%
              </span>
            </div>
            <Progress
              v-if="!w.unlimited"
              class="mt-1.5 h-1.5"
              :model-value="Math.min(100, Math.max(0, w.used))"
              :color="barColor(w.used)"
              :aria-label="`${windowLabel(w.name)} ${Math.round(w.used)}%`"
            />
            <p v-if="resetText(w)" class="mt-1 text-[11px] text-muted-foreground">{{ resetText(w) }}</p>
          </div>
        </div>
        <p
          v-if="q.status !== 'ok' || !q.windows.length"
          class="mt-4 text-xs leading-relaxed"
          :class="q.status === 'error' ? 'text-destructive' : 'text-muted-foreground'"
        >
          {{ q.status === 'ok' ? tx('该订阅没有返回额度窗口。', 'This subscription reported no windows.') : statusText(q) }}
        </p>
        <p v-if="q.credits" class="mt-3 text-xs text-muted-foreground">{{ tx('剩余点数', 'Credits left') }}：{{ q.credits }}</p>
        <p v-if="q.read_at" class="mt-3 text-[11px] text-muted-foreground">{{ tx('读取于', 'Read at') }} {{ formatClock(q.read_at) }}</p>
      </article>
    </div>
  </section>

  <section class="wb-card" aria-labelledby="quota-balance-title">
    <div class="wb-row justify-between !mb-1">
      <h2 id="quota-balance-title" class="!mb-0">{{ tx('供应商余额', 'Provider balances') }}</h2>
      <Button variant="outline" size="sm" type="button" :disabled="loadingBalances" @click="loadBalances(true)">
        <Loader2 v-if="loadingBalances" class="animate-spin" />
        <RefreshCw v-else />
        {{ tx('全部刷新', 'Refresh all') }}
      </Button>
    </div>
    <p class="wb-hint !mb-0">
      {{
        tx(
          '按环境的 Base URL 自动识别 DeepSeek、Moonshot、OpenRouter、SiliconFlow、StepFun、AiHubMix 余额；中转站请在下方选择接口类型。同一个 Key 只查询一次，Key 只会发往该环境自己的域名。',
          'Detects DeepSeek, Moonshot, OpenRouter, SiliconFlow, StepFun and AiHubMix from each environment’s Base URL; pick an API type below for relays. Each key is queried once and only sent to its own host.',
        )
      }}
    </p>
    <p v-if="balancesError" role="alert" class="wb-error mt-3">{{ balancesError }}</p>
    <div v-if="loadingBalances && !balances.length" class="mt-4 space-y-2" aria-busy="true">
      <div v-for="i in 2" :key="i" class="h-14 animate-pulse rounded-lg bg-muted" />
    </div>
    <div v-else-if="!balances.length" class="wb-empty mt-4">
      {{
        tx(
          '没有可自动识别余额接口的环境。可在下方「余额接口」中为中转站或其它供应商选择接口类型。',
          'No environment with a recognizable balance API. Use “Balance source” below to set one up for relays or other vendors.',
        )
      }}
    </div>
    <ul v-else class="mt-4 divide-y divide-border rounded-xl border border-border">
      <li v-for="b in balances" :key="b.id" class="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium">
            {{ b.vendor }}
            <span class="ml-1 font-mono text-xs text-muted-foreground">{{ b.key_hint }}</span>
          </p>
          <p class="mt-0.5 truncate text-xs text-muted-foreground" :title="b.environments.join('、')">
            {{ b.host }} · {{ b.environments.join('、') }}
          </p>
        </div>
        <div class="text-right">
          <p v-if="b.error" class="max-w-[320px] text-xs text-destructive">{{ b.error }}</p>
          <p v-else class="text-lg font-semibold tabular-nums" :class="b.low ? 'text-destructive' : ''">{{ b.display }}</p>
          <p v-if="!b.error && b.alert_below" class="text-[11px] text-muted-foreground">
            {{ b.low ? tx('低于提醒线', 'Below alert') : tx('提醒线', 'Alert at') }} {{ b.alert_below }}
          </p>
        </div>
      </li>
    </ul>
  </section>

  <section class="wb-card" aria-labelledby="quota-source-title">
    <h2 id="quota-source-title">{{ tx('余额接口', 'Balance source') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '为某个环境指定余额查询方式与低余额提醒线。中转站常见为 New API / One API；其它供应商可选“自定义接口”，填写同域名的余额地址和返回中的余额字段。',
          'Choose how an environment’s balance is read and when to warn. Relays usually run New API / One API; for others pick “Custom endpoint” with a same-host URL and the balance field.',
        )
      }}
    </p>
    <div class="wb-grid">
      <label>
        {{ tx('环境', 'Environment') }}
        <Select v-model="selectedEnv" :disabled="sourceBusy">
          <SelectTrigger class="h-9 w-full min-w-0">
            <SelectValue :placeholder="tx('选择环境', 'Select environment')" />
          </SelectTrigger>
          <SelectContent position="popper" align="start">
            <SelectItem v-for="e in keyedEnvs" :key="`${e.provider}/${e.name}`" :value="`${e.provider}/${e.name}`">
              {{ e.provider }} · {{ e.name }}
            </SelectItem>
          </SelectContent>
        </Select>
      </label>
      <label>
        {{ tx('接口类型', 'API type') }}
        <Select v-model="sourceForm.adapter" :disabled="sourceBusy || !selectedEnv">
          <SelectTrigger class="h-9 w-full min-w-0">
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper" align="start">
            <SelectItem v-for="a in adapters" :key="a.value" :value="a.value">{{ a.label }}</SelectItem>
          </SelectContent>
        </Select>
      </label>
      <label>
        {{ tx('余额低于此值时提醒（0 不提醒）', 'Warn below (0 = off)') }}
        <WorkbenchNumberInput v-model="sourceForm.alert_below" :min="0" :step="1" :disabled="sourceBusy || !selectedEnv" />
      </label>
    </div>
    <div v-if="sourceForm.adapter === 'custom' && selectedEnv" class="wb-grid mt-4">
      <label>
        {{ tx('余额地址（与 Base URL 同域名）', 'Balance URL (same host as Base URL)') }}
        <Input v-model="sourceForm.url" type="url" placeholder="https://api.example.com/v1/user/balance" />
      </label>
      <label>
        {{ tx('余额字段', 'Balance field') }}
        <Input v-model="sourceForm.path" placeholder="data.balance" />
      </label>
      <label>
        {{ tx('换算除数（0 不换算）', 'Divide by (0 = none)') }}
        <WorkbenchNumberInput v-model="sourceForm.divisor" :min="0" :step="1" />
      </label>
      <label>
        {{ tx('币种', 'Currency') }}
        <Select v-model="sourceForm.currency">
          <SelectTrigger class="h-9 w-full min-w-0">
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper" align="start">
            <SelectItem value="USD">{{ tx('美元 $', 'USD $') }}</SelectItem>
            <SelectItem value="CNY">{{ tx('人民币 ¥', 'CNY ¥') }}</SelectItem>
          </SelectContent>
        </Select>
      </label>
    </div>
    <div class="wb-row mt-4">
      <Button variant="outline" type="button" :disabled="sourceBusy || !selectedEnv" @click="testSource">
        {{ tx('测试查询', 'Test') }}
      </Button>
      <Button type="button" :disabled="sourceBusy || !selectedEnv" @click="saveSource()">
        {{ tx('保存', 'Save') }}
      </Button>
      <Button v-if="hasSaved" variant="ghost" type="button" :disabled="sourceBusy" @click="saveSource(true)">
        {{ tx('移除此配置', 'Remove') }}
      </Button>
      <span v-if="testResult" role="status" class="text-sm font-medium">{{ testResult }}</span>
    </div>
  </section>

  <section class="wb-card" aria-labelledby="quota-alert-title">
    <h2 id="quota-alert-title">{{ tx('提醒', 'Alerts') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '开启后台检查后，订阅窗口用量达到阈值、或余额低于提醒线时提醒一次（每个窗口每个周期只提醒一次）。Claude 只在 Claude Code 有新会话后才会重新读取。',
          'With background checks on, you are told once when a window passes the threshold or a balance drops below its alert line (once per window per period). Claude is re-read only after Claude Code has been used.',
        )
      }}
    </p>
    <form class="wb-grid" @submit.prevent="saveSettings">
      <label>
        {{ tx('额度提醒阈值', 'Allowance threshold') }}
        <Select v-model="alertSelect">
          <SelectTrigger class="h-9 w-full min-w-0">
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper" align="start">
            <SelectItem v-for="p in alertOptions" :key="p" :value="String(p)">
              {{ p === 0 ? tx('关闭', 'Off') : `${p}%` }}
            </SelectItem>
          </SelectContent>
        </Select>
      </label>
      <label>
        {{ tx('后台检查间隔', 'Background check') }}
        <Select v-model="monitorSelect">
          <SelectTrigger class="h-9 w-full min-w-0">
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper" align="start">
            <SelectItem v-for="m in monitorOptions" :key="m" :value="String(m)">
              {{ m === 0 ? tx('关闭', 'Off') : m >= 60 ? tx(`每 ${m / 60} 小时`, `Every ${m / 60} h`) : tx(`每 ${m} 分钟`, `Every ${m} min`) }}
            </SelectItem>
          </SelectContent>
        </Select>
      </label>
      <label class="wb-check self-end pb-2">
        <Checkbox :model-value="settings.desktop_notify" @update:model-value="settings.desktop_notify = $event === true" />
        {{ tx('同时发送系统通知', 'Also send a system notification') }}
      </label>
      <div class="self-end">
        <Button type="submit" :disabled="savingSettings">{{ tx('保存提醒设置', 'Save alerts') }}</Button>
      </div>
    </form>
  </section>

  <section class="wb-card" aria-labelledby="quota-warm-title">
    <h2 id="quota-warm-title">{{ tx('额度预热', 'Window warm-up') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '订阅的 5 小时窗口从第一次使用开始计时。在每天开工前（或窗口刚重置时）自动发一句极短的消息，让窗口提前开始，工作到一半时额度就会刷新。每次预热消耗极少额度。',
          'A subscription’s 5-hour window starts with its first use. Sending a tiny message before you start work (or right after a reset) starts the window early so it refreshes mid-session. Each warm-up uses a sliver of allowance.',
        )
      }}
    </p>
    <div class="wb-row">
      <label class="wb-check">
        <Checkbox :model-value="settings.warmup.claude" @update:model-value="settings.warmup.claude = $event === true" />
        Claude Code
      </label>
      <Button variant="ghost" size="sm" type="button" :disabled="!!warming" @click="warmNow('claude')">
        {{ warming === 'claude' ? tx('预热中…', 'Warming…') : tx('立即预热', 'Warm now') }}
      </Button>
      <span class="text-xs text-muted-foreground">{{ warmText('claude') }}</span>
    </div>
    <div class="wb-row">
      <label class="wb-check">
        <Checkbox :model-value="settings.warmup.codex" @update:model-value="settings.warmup.codex = $event === true" />
        Codex（ChatGPT）
      </label>
      <Button variant="ghost" size="sm" type="button" :disabled="!!warming" @click="warmNow('codex')">
        {{ warming === 'codex' ? tx('预热中…', 'Warming…') : tx('立即预热', 'Warm now') }}
      </Button>
      <span class="text-xs text-muted-foreground">{{ warmText('codex') }}</span>
    </div>
    <div class="wb-row items-end">
      <span class="text-sm">{{ tx('每天预热时间', 'Daily times') }}</span>
      <div v-for="(time, i) in settings.warmup.times || []" :key="i" class="flex items-center gap-1">
        <Input v-model="settings.warmup.times![i]" type="time" class="h-9 w-28" :aria-label="`${tx('预热时间', 'Warm-up time')} ${time}`" />
        <Button variant="ghost" size="icon-sm" type="button" :aria-label="tx('删除', 'Remove')" @click="settings.warmup.times!.splice(i, 1)">×</Button>
      </div>
      <Button variant="outline" size="sm" type="button" :disabled="(settings.warmup.times || []).length >= 6" @click="addWarmTime">
        {{ tx('添加时间', 'Add time') }}
      </Button>
    </div>
    <label class="wb-check">
      <Checkbox :model-value="settings.warmup.on_reset" @update:model-value="settings.warmup.on_reset = $event === true" />
      {{ tx('5 小时窗口重置后立即预热（需开启后台检查）', 'Warm up right after a 5-hour reset (needs background checks)') }}
    </label>
    <div class="mt-4">
      <Button type="button" :disabled="savingSettings" @click="saveSettings">{{ tx('保存预热设置', 'Save warm-up') }}</Button>
    </div>
  </section>
</template>
