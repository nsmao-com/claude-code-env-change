<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import ModelCombobox from './ModelCombobox.vue'
import { Loader2, RefreshCw } from '@lucide/vue'
import { ref, reactive, computed } from 'vue'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { useConfigStore } from '@/stores/configStore'
import { useConfirm } from '@/composables/useConfirm'
import { callService } from '@/services/appBridge'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'

interface ExtraAgentStatus {
  id: string
  name: string
  installed: boolean
  config_path: string
  connected: boolean
  provider?: string
  environment?: string
  model?: string
  note?: string
}

const { tx, busy, error, run } = useWorkbench()
const config = useConfigStore()
const confirm = useConfirm()
const agents = ref<ExtraAgentStatus[]>([])
const loading = ref(false)
const editing = ref('')
const form = reactive({ env: '', model: '' })
const models = ref<{ id: string; name: string }[]>([])

const envs = computed(() => config.environments.filter((e) => !e.official_login))
const selectedEnv = computed(() => envs.value.find((e) => `${e.provider}/${e.name}` === form.env))

async function load() {
  loading.value = true
  try {
    agents.value = (await workbench<ExtraAgentStatus[]>('ListExtraAgents')) || []
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function openForm(a: ExtraAgentStatus) {
  if (editing.value === a.id) {
    editing.value = ''
    return
  }
  editing.value = a.id
  models.value = []
  form.env = a.connected && a.provider ? `${a.provider}/${a.environment}` : envs.value[0] ? `${envs.value[0].provider}/${envs.value[0].name}` : ''
  form.model = a.model || ''
}

function envBase(): string {
  const e = selectedEnv.value
  if (!e) return ''
  const v = e.variables || {}
  return v.ANTHROPIC_BASE_URL || v.base_url || v.GOOGLE_GEMINI_BASE_URL || v.OPENCODE_BASE_URL || v.XAI_BASE_URL || ''
}

async function discover() {
  const e = selectedEnv.value
  if (!e) return
  await run(async () => {
    const base = envBase()
    const m = /^account:\/\/(codex|copilot|claude)/i.exec(base)
    if (m) {
      const ids = await callService<string[]>('RouterService', 'ListAccountModels', m[1].toLowerCase())
      models.value = (ids || []).map((id) => ({ id, name: id }))
    } else {
      models.value = (await workbench<{ id: string; name: string }[]>('DiscoverModels', e.provider, e.name)) || []
    }
  })
}

async function connect(a: ExtraAgentStatus) {
  const e = selectedEnv.value
  if (!e || !form.model.trim()) return
  await run(async () => {
    await workbench('ConnectExtraAgent', a.id, e.provider, e.name, form.model.trim())
    editing.value = ''
    await load()
  }, tx(`${a.name} 已接入本机网关，重新打开它即可生效`, `${a.name} now uses the local gateway; restart it to apply`))
}

async function disconnect(a: ExtraAgentStatus) {
  const ok = await confirm.show(
    tx('断开接入', 'Disconnect'),
    tx(
      `将从 ${a.name} 的配置中删除 AI ENV 写入的条目，并恢复接入前的默认模型；对应的网关路由也会删除。`,
      `AI ENV's entries are removed from ${a.name}'s config and its previous default model is restored; the gateway route is removed too.`,
    ),
    'warning',
  )
  if (!ok) return
  await run(async () => {
    await workbench('DisconnectExtraAgent', a.id)
    await load()
  }, tx(`已断开 ${a.name}`, `${a.name} disconnected`))
}

// ===== RTK =====
interface RTKStatus {
  installed: boolean
  path?: string
  version?: string
  can_install: boolean
  install_hint: string
  agents: { id: string; name: string; enabled: boolean }[]
  homepage: string
}
const rtk = ref<RTKStatus | null>(null)
const rtkBusy = ref(false)
async function loadRTK() {
  try {
    rtk.value = await workbench<RTKStatus>('GetRTKStatus')
  } catch {
    rtk.value = null
  }
}
async function installRTK() {
  const ok = await confirm.show(
    tx('安装 rtk', 'Install rtk'),
    tx(`将运行系统包管理器安装 rtk（${rtk.value?.install_hint || ''}），可能需要几分钟。`, `rtk will be installed with the system package manager (${rtk.value?.install_hint || ''}); this can take a few minutes.`),
    'info',
  )
  if (!ok) return
  rtkBusy.value = true
  try {
    rtk.value = await workbench<RTKStatus>('InstallRTK')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
    await loadRTK()
  } finally {
    rtkBusy.value = false
  }
}
async function toggleRTK(agent: string, on: boolean) {
  rtkBusy.value = true
  try {
    rtk.value = await workbench<RTKStatus>('SetRTKAgent', agent, on)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
    await loadRTK()
  } finally {
    rtkBusy.value = false
  }
}
function openRTKSite() {
  if (rtk.value?.homepage) BrowserOpenURL(rtk.value.homepage)
}

void config.loadConfig().catch(() => {})
void load()
void loadRTK()
</script>

<template>
  <p v-if="error" role="alert" class="wb-error">{{ error }}</p>
  <section class="wb-card">
    <div class="wb-row justify-between !mb-1">
      <h2 class="!mb-0">{{ tx('更多 Agent', 'More agents') }}</h2>
      <Button variant="outline" size="sm" type="button" :disabled="loading" @click="load">
        <Loader2 v-if="loading" class="animate-spin" />
        <RefreshCw v-else />
        {{ tx('重新检测', 'Rescan') }}
      </Button>
    </div>
    <p class="wb-hint">
      {{
        tx(
          '把下列命令行 Agent 接到本机网关：选一个已有环境作为上游，协议由网关自动转换（如用 Claude 中转驱动 Crush）。写入前会保存历史快照与 .bak 备份；断开时恢复原来的默认模型。接入后会开启“网关随应用启动”。',
          'Point these CLI agents at the local gateway: pick an existing environment as the upstream and the gateway converts protocols (e.g. drive Crush with a Claude relay). A history snapshot and .bak are saved before writing; disconnecting restores the previous default model. Connecting turns on “start gateway with the app”.',
        )
      }}
    </p>
    <div v-if="loading && !agents.length" class="space-y-2" aria-busy="true">
      <div v-for="i in 3" :key="i" class="h-14 animate-pulse rounded-lg bg-muted" />
    </div>
    <ul v-else class="divide-y divide-border rounded-xl border border-border">
      <li v-for="a in agents" :key="a.id" class="px-4 py-3">
        <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
          <div class="min-w-0 flex-1">
            <p class="flex items-center gap-2 text-sm font-medium">
              {{ a.name }}
              <Badge v-if="a.connected" class="border-transparent bg-green-500/10 text-green-700 dark:text-green-400">{{ tx('已接入', 'Connected') }}</Badge>
              <Badge v-else-if="!a.installed" variant="secondary">{{ tx('未检测到', 'Not found') }}</Badge>
            </p>
            <p v-if="a.connected" class="mt-0.5 text-xs text-muted-foreground">
              {{ a.environment ? `${a.provider} · ${a.environment} · ` : '' }}{{ a.model }}
            </p>
            <p class="mt-0.5 truncate font-mono text-[11px] text-muted-foreground" :title="a.config_path">{{ a.config_path }}</p>
            <p v-if="a.note" class="mt-1 whitespace-pre-line text-xs text-muted-foreground" :class="a.note.includes('⚠') ? 'text-amber-600 dark:text-amber-400' : ''">{{ a.note }}</p>
          </div>
          <Button variant="outline" size="sm" type="button" :disabled="busy" @click="openForm(a)">
            {{ a.connected ? tx('更换模型', 'Change') : tx('接入', 'Connect') }}
          </Button>
          <Button v-if="a.connected" variant="ghost" size="sm" type="button" class="text-destructive" :disabled="busy" @click="disconnect(a)">
            {{ tx('断开', 'Disconnect') }}
          </Button>
        </div>
        <form v-if="editing === a.id" class="mt-3 rounded-lg bg-muted/50 p-3" @submit.prevent="connect(a)">
          <div class="wb-grid">
            <label>
              {{ tx('上游环境', 'Upstream environment') }}
              <Select v-model="form.env" :disabled="busy">
                <SelectTrigger class="h-9 w-full min-w-0"><SelectValue :placeholder="tx('选择环境', 'Select environment')" /></SelectTrigger>
                <SelectContent position="popper" align="start">
                  <SelectItem v-for="e in envs" :key="`${e.provider}/${e.name}`" :value="`${e.provider}/${e.name}`">
                    {{ e.provider }} · {{ e.name }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label>
              {{ tx('模型', 'Model') }}
              <ModelCombobox v-model="form.model" :models="models" :disabled="busy" />
            </label>
          </div>
          <div class="wb-row mt-3 !mb-0">
            <Button variant="outline" size="sm" type="button" :disabled="busy || !form.env" @click="discover">
              {{ tx('获取模型列表', 'Load models') }}
            </Button>
            <span v-if="models.length" class="text-xs text-muted-foreground">{{ models.length }} {{ tx('个模型', 'models') }}</span>
            <Button size="sm" type="submit" class="ml-auto" :disabled="busy || !form.env || !form.model.trim()">
              {{ tx('保存并接入', 'Save & connect') }}
            </Button>
          </div>
          <p v-if="!a.installed" class="mt-2 text-xs text-muted-foreground">
            {{ tx('没有检测到这个 Agent，仍可先写入配置，安装后直接生效。', 'This agent was not found; the config can still be written and will apply once it is installed.') }}
          </p>
        </form>
      </li>
    </ul>
  </section>

  <section class="wb-card">
    <h2>{{ tx('RTK：精简命令输出', 'RTK: leaner command output') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          'rtk 会在 Agent 执行 git、ls、测试等命令时把输出精简后再交给模型，通常能明显减少 Token。为某个 Agent 开启会运行 rtk 自带的安装器写入钩子；关闭时只删除 rtk 写入的钩子与 RTK.md，其它配置不动。改动后需要重新打开对应 Agent。',
          'rtk trims the output of commands an agent runs (git, ls, tests…) before the model sees it, usually saving many tokens. Turning it on runs rtk’s own installer to add its hook; turning it off removes only rtk’s hook and RTK.md. Restart the agent afterwards.',
        )
      }}
    </p>
    <div v-if="rtk" class="space-y-3">
      <div class="wb-row !mb-0">
        <span class="text-sm">
          {{ rtk.installed ? tx(`已安装 ${rtk.version || ''}`, `Installed ${rtk.version || ''}`) : tx('未安装 rtk', 'rtk is not installed') }}
        </span>
        <Button v-if="!rtk.installed && rtk.can_install" size="sm" type="button" :disabled="rtkBusy" @click="installRTK">
          <Loader2 v-if="rtkBusy" class="animate-spin" />
          {{ tx('安装 rtk', 'Install rtk') }}
        </Button>
        <Button variant="ghost" size="sm" type="button" @click="openRTKSite">{{ tx('官网', 'Website') }}</Button>
      </div>
      <div v-for="a in rtk.agents" :key="a.id" class="flex items-center gap-3">
        <Switch
          :checked="a.enabled"
          :disabled="rtkBusy || (!rtk.installed && !a.enabled)"
          :aria-label="a.name"
          @update:checked="(v: boolean) => toggleRTK(a.id, v)"
        />
        <span class="text-sm">{{ a.name }}</span>
        <span class="text-xs text-muted-foreground">{{ a.enabled ? tx('已启用', 'On') : tx('未启用', 'Off') }}</span>
      </div>
    </div>
  </section>
</template>
