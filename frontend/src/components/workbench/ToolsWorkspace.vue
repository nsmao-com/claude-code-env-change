<script setup lang="ts">
import { ref, computed } from 'vue'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { callApp, callService } from '@/services/appBridge'
import { useConfigStore } from '@/stores/configStore'
import { useConfirm } from '@/composables/useConfirm'
import DiffPreview from './DiffPreview.vue'
import type { DiagnosticItem, HistorySummary, HistoryPreview } from '@/types/workbench'
const { tx, busy, error, run } = useWorkbench(),
  config = useConfigStore(),
  confirm = useConfirm()
const selected = ref(''),
  network = ref(false),
  diagnostics = ref<DiagnosticItem[]>([]),
  history = ref<HistorySummary[]>([])
const preview = ref<HistoryPreview | null>(null),
  historyID = ref(''),
  files = ref<string[]>([])
const health = ref<
  {
    route: string
    host: string
    requests: number
    failures: number
    consecutive: number
    latency: number
    cool_until: number
    probing: boolean
  }[]
>([])
const healthTime = ref(Date.now())
async function loadHealth() {
  health.value = await callService('RouterService', 'GetUpstreamHealth')
  healthTime.value = Date.now()
}
const env = computed(() => config.environments.find((e) => `${e.provider}/${e.name}` === selected.value))
async function load() {
  history.value = await callApp<HistorySummary[]>('ListConfigHistory')
  await loadHealth()
}
async function diagnose() {
  if (!env.value) return
  if (
    network.value &&
    !(await confirm.show(
      tx('联网诊断', 'Network diagnostics'),
      tx(
        '将启动选中的 MCP 服务并发送最小模型请求，可能产生费用。',
        'Starts selected MCP servers and sends a minimal model request, which may incur cost.',
      ),
    ))
  )
    return
  await run(async () => {
    diagnostics.value = await workbench('RunDiagnostics', env.value!.provider, env.value!.name, network.value)
  })
}
async function view(h: HistorySummary) {
  await run(async () => {
    preview.value = await callApp('PreviewConfigHistory', h.id)
    historyID.value = h.id
    files.value = preview.value!.changes.filter((c) => c.changed).map((c) => c.path)
  })
}
async function restore() {
  if (
    !preview.value ||
    !(await confirm.show(
      tx('恢复选中文件', 'Restore selected files'),
      tx(
        '将用预览中的历史版本覆盖所选配置，当前内容会先备份。',
        'Overwrite selected configuration files with the previewed version. Current content is backed up first.',
      ),
      'warning',
    ))
  )
    return
  await run(
    async () => {
      await callApp('RestoreConfigHistoryFiles', historyID.value, preview.value!.token, files.value)
      preview.value = null
      await config.loadConfig()
      await load()
    },
    tx('已恢复配置', 'Configuration restored'),
  )
}
void run(async () => {
  await config.loadConfig()
  selected.value = config.environments[0]
    ? `${config.environments[0].provider}/${config.environments[0].name}`
    : ''
  await load()
})
</script>
<template>
  <p v-if="error" class="wb-error" role="alert">{{ error }}</p>
  <section class="wb-card">
    <h2>{{ tx('统一诊断', 'Diagnostics') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '检查配置格式、CLI 安装、配置漂移、网络及 MCP 握手。',
          'Check configuration syntax, CLI installation, configuration drift, network and MCP handshake.',
        )
      }}
    </p>
    <label
      >{{ tx('环境', 'Environment')
      }}<select v-model="selected">
        <option value="" disabled>{{ tx('请选择环境', 'Select an environment') }}</option>
        <option
          v-for="e in config.environments"
          :key="`${e.provider}/${e.name}`"
          :value="`${e.provider}/${e.name}`"
        >
          {{ e.provider }} · {{ e.name }}
        </option>
      </select></label
    >
    <div class="wb-row mt-4">
      <label class="wb-check"
        ><input type="checkbox" v-model="network" />{{
          tx('包含联网与模型试请求', 'Include network and model request')
        }}</label
      ><button class="primary" :disabled="busy || !env" @click="diagnose">
        {{ busy ? tx('处理中…', 'Working…') : tx('开始诊断', 'Run diagnostics') }}
      </button>
    </div>
    <div class="wb-table" v-if="diagnostics.length">
      <table>
        <thead>
          <tr>
            <th>{{ tx('项目', 'Check') }}</th>
            <th>{{ tx('状态', 'Status') }}</th>
            <th>{{ tx('结果与建议', 'Result & action') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(item, i) in diagnostics" :key="i">
            <td>{{ item.name }}</td>
            <td :class="item.status === 'error' ? 'text-destructive' : ''">{{ item.status }}</td>
            <td>
              <p class="break-all">{{ item.message }}</p>
              <p class="mt-1 text-xs text-muted-foreground">{{ item.action }}</p>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
  <section class="wb-card">
    <div class="wb-row justify-between">
      <h2>{{ tx('配置历史', 'Configuration history') }}</h2>
      <button :disabled="busy" @click="run(load)">{{ tx('刷新', 'Refresh') }}</button>
    </div>
    <p class="wb-hint">
      {{
        tx(
          '保留最近 100 份写入前快照。恢复前查看脱敏差异，支持按文件选择。',
          'Keeps the latest 100 snapshots taken before writes. Review redacted changes and select files to restore.',
        )
      }}
    </p>
    <div v-if="!history.length" class="wb-empty">
      {{
        tx(
          '暂无历史。修改配置后会自动生成快照。',
          'No history yet. Configuration changes create snapshots automatically.',
        )
      }}
    </div>
    <div class="wb-list max-h-80 overflow-auto">
      <button
        v-for="h in history"
        :key="h.id"
        :disabled="busy"
        :class="{ active: historyID === h.id && preview }"
        @click="view(h)"
      >
        <strong>{{ new Date(h.at).toLocaleString() }} · {{ h.reason }}</strong
        ><span class="truncate text-xs opacity-70">{{ h.paths.join(' · ') }}</span>
      </button>
    </div>
    <div v-if="preview" class="mt-5">
      <DiffPreview :changes="preview.changes" v-model="files" /><button
        class="primary mt-4"
        :disabled="busy || !files.length"
        @click="restore"
      >
        {{ tx('恢复所选文件', 'Restore selected files') }} ({{ files.length }})
      </button>
    </div>
  </section>
  <section class="wb-card">
    <div class="wb-row justify-between">
      <h2>{{ tx('网关上游健康', 'Gateway upstream health') }}</h2>
      <button :disabled="busy" @click="run(loadHealth)">{{ tx('刷新健康状态', 'Refresh health') }}</button>
    </div>
    <p class="wb-hint">
      {{
        tx(
          '显示本次运行的请求结果。冷却结束后，下一个请求负责探测恢复；刷新不会主动发送模型请求。',
          'Request results for this run. The next request probes recovery after cooldown; refreshing does not send model requests.',
        )
      }}
    </p>
    <div v-if="!health.length" class="wb-empty">
      {{
        tx(
          '暂无上游请求，使用网关后显示健康指标。',
          'No upstream requests yet. Metrics appear after gateway traffic.',
        )
      }}
    </div>
    <div v-else class="wb-table">
      <table>
        <thead>
          <tr>
            <th>{{ tx('路由 / 上游', 'Route / upstream') }}</th>
            <th>{{ tx('状态', 'Status') }}</th>
            <th>{{ tx('请求 / 失败 / 连续失败', 'Requests / failures / consecutive failures') }}</th>
            <th>{{ tx('最近响应头耗时', 'Last response headers latency') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(h, i) in health" :key="i">
            <td>
              {{ h.route }}
              <p class="text-xs text-muted-foreground">{{ h.host }}</p>
            </td>
            <td>
              {{
                h.probing
                  ? tx('恢复探测中', 'Probing')
                  : h.cool_until > healthTime
                    ? tx('冷却至 ', 'Cooling until ') + new Date(h.cool_until).toLocaleTimeString()
                    : h.cool_until
                      ? tx('等待恢复探测', 'Awaiting recovery probe')
                      : tx('可用', 'Available')
              }}
            </td>
            <td>{{ h.requests }} / {{ h.failures }} / {{ h.consecutive }}</td>
            <td>{{ h.latency }} ms</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
