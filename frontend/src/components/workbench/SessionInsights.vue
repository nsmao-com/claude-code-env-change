<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import InsightBars from './InsightBars.vue'
import { Loader2, RefreshCw } from '@lucide/vue'
import { ref, reactive, computed, watch, onScopeDispose } from 'vue'
import { useWorkbench } from '@/composables/useWorkbench'
import { callService } from '@/services/appBridge'
import type { InsightCount, SessionInsights } from '@/types/workbench'

const { tx } = useWorkbench()
const query = reactive({ provider: 'all', days: 30, project: '' })
const rangeValue = computed({
  get: () => String(query.days),
  set: (v: string) => {
    query.days = Number(v)
  },
})
const data = ref<SessionInsights | null>(null)
const loading = ref(false)
const error = ref('')
const metric = ref<'prompts' | 'tokens'>('prompts')
let generation = 0

async function load() {
  const current = ++generation
  loading.value = true
  error.value = ''
  try {
    const result = await callService<SessionInsights>('SessionService', 'GetSessionInsights', { ...query })
    if (current === generation) data.value = result
  } catch (e) {
    if (current === generation) error.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (current === generation) loading.value = false
  }
}

let timer: ReturnType<typeof setTimeout>
watch(
  () => [query.provider, query.days, query.project],
  () => {
    clearTimeout(timer)
    timer = setTimeout(load, 300)
  },
)
onScopeDispose(() => {
  clearTimeout(timer)
  generation++
})
void load()

function compact(n: number) {
  if (n >= 1e9) return `${(n / 1e9).toFixed(1)}B`
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(1)}K`
  return String(Math.round(n))
}

const totalTokens = computed(() =>
  data.value ? data.value.input_tokens + data.value.output_tokens + data.value.cache_write_tokens : 0,
)
const kpis = computed(() => {
  const d = data.value
  if (!d) return []
  return [
    { label: tx('会话', 'Sessions'), value: compact(d.sessions), sub: tx(`扫描 ${d.scanned} 个日志文件`, `${d.scanned} log files scanned`) },
    { label: tx('提问', 'Prompts'), value: compact(d.prompts), sub: tx(`回复 ${compact(d.replies)} 条`, `${compact(d.replies)} replies`) },
    { label: tx('工具调用', 'Tool calls'), value: compact(d.tool_calls), sub: d.prompts ? tx(`平均每次提问 ${(d.tool_calls / d.prompts).toFixed(1)} 次`, `${(d.tool_calls / d.prompts).toFixed(1)} per prompt`) : '' },
    { label: 'Token', value: compact(totalTokens.value), sub: tx(`另有缓存读取 ${compact(d.cache_read_tokens)}`, `+${compact(d.cache_read_tokens)} cache reads`) },
    { label: tx('活跃时长', 'Active time'), value: d.active_minutes >= 60 ? tx(`${(d.active_minutes / 60).toFixed(1)} 小时`, `${(d.active_minutes / 60).toFixed(1)} h`) : tx(`${d.active_minutes} 分钟`, `${d.active_minutes} min`), sub: tx('相邻活动间隔不超过 5 分钟计入', 'Gaps ≤ 5 min counted') },
    { label: tx('每会话 Token', 'Tokens per session'), value: compact(d.median_tokens), sub: tx(`中位数 · P90 ${compact(d.p90_tokens)}`, `median · P90 ${compact(d.p90_tokens)}`) },
  ]
})

const days = computed(() => data.value?.days || [])
const dayMax = computed(() => Math.max(1, ...days.value.map((d) => (metric.value === 'tokens' ? d.tokens : d.prompts))))
const hourMax = computed(() => Math.max(1, ...(data.value?.hours || [0])))
const weekdayMax = computed(() => Math.max(1, ...(data.value?.weekdays || [0])))
const weekdayNames = computed(() =>
  tx('一,二,三,四,五,六,日', 'Mon,Tue,Wed,Thu,Fri,Sat,Sun').split(','),
)
const lengthTotal = computed(() => (data.value?.lengths || []).reduce((s, i) => s + i.count, 0) || 1)

const tokenFormat = (i: InsightCount) => compact(i.tokens || 0)
const projectFormat = (i: InsightCount) => tx(`${i.count} 个会话 · ${compact(i.tokens || 0)}`, `${i.count} sessions · ${compact(i.tokens || 0)}`)
function shortPath(p: string) {
  const parts = p.split(/[\\/]/).filter(Boolean)
  return parts.length > 2 ? `…/${parts.slice(-2).join('/')}` : p
}
const projectItems = computed(() => (data.value?.projects || []).map((p) => ({ ...p, name: shortPath(p.name) })))
</script>

<template>
  <div class="wb-card">
    <div class="wb-row justify-between !mb-0">
      <div class="flex flex-wrap items-center gap-3">
        <Select v-model="query.provider">
          <SelectTrigger class="h-9 w-40 min-w-0" :aria-label="tx('工具', 'Tool')">
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper" align="start">
            <SelectItem value="all">{{ tx('全部工具', 'All tools') }}</SelectItem>
            <SelectItem value="claude">Claude Code</SelectItem>
            <SelectItem value="codex">Codex</SelectItem>
          </SelectContent>
        </Select>
        <SegmentedPills
          v-model="rangeValue"
          layout-id="insight-range"
          dense
          :items="[
            { value: '7', label: tx('7 天', '7 days') },
            { value: '30', label: tx('30 天', '30 days') },
            { value: '90', label: tx('90 天', '90 days') },
            { value: '0', label: tx('全部', 'All') },
          ]"
        />
        <Input
          v-model="query.project"
          type="search"
          class="h-9 w-56"
          :placeholder="tx('按项目路径筛选…', 'Filter by project path…')"
          :aria-label="tx('项目路径', 'Project path')"
        />
      </div>
      <Button variant="outline" size="sm" type="button" :disabled="loading" @click="load">
        <Loader2 v-if="loading" class="animate-spin" />
        <RefreshCw v-else />
        {{ tx('刷新', 'Refresh') }}
      </Button>
    </div>
    <p class="wb-hint mt-3 !mb-0">
      {{
        tx(
          '统计本机 Claude Code 与 Codex 会话日志里的提问、工具调用、Skills、MCP 与 Token（Claude 子代理计入父会话）。只读本地文件，不上传任何内容。',
          'Counts prompts, tool calls, skills, MCP and tokens from local Claude Code and Codex session logs (Claude subagents count toward their parent). Reads local files only; nothing is uploaded.',
        )
      }}
    </p>
  </div>

  <p v-if="error" role="alert" class="wb-error">{{ error }}</p>
  <div v-if="loading && !data" class="wb-grid" aria-busy="true">
    <div v-for="i in 6" :key="i" class="h-24 animate-pulse rounded-xl bg-muted" />
  </div>
  <template v-else-if="data">
    <div v-if="!data.sessions" class="wb-empty">
      {{
        query.project
          ? tx('没有匹配该项目路径的会话，换个关键词试试。', 'No sessions match this project path. Try another keyword.')
          : tx('所选时间范围内没有会话记录。在 Claude Code 或 Codex 中工作后再来看看。', 'No sessions in this range. Come back after working in Claude Code or Codex.')
      }}
    </div>
    <template v-else>
      <div class="wb-grid mb-4">
        <div v-for="k in kpis" :key="k.label" class="rounded-xl border border-border bg-card p-4">
          <p class="text-xs text-muted-foreground">{{ k.label }}</p>
          <p class="mt-1 text-2xl font-semibold tabular-nums">{{ k.value }}</p>
          <p class="mt-1 text-[11px] text-muted-foreground">{{ k.sub }}</p>
        </div>
      </div>

      <section class="wb-card">
        <div class="wb-row justify-between">
          <h2 class="!mb-0">{{ tx('每日活动', 'Daily activity') }}</h2>
          <SegmentedPills
            v-model="metric"
            layout-id="insight-metric"
            dense
            :items="[
              { value: 'prompts', label: tx('提问', 'Prompts') },
              { value: 'tokens', label: 'Token' },
            ]"
          />
        </div>
        <div class="overflow-x-auto pb-1">
          <div class="flex h-36 min-w-full items-end gap-1" role="img" :aria-label="tx('每日活动柱状图', 'Daily activity chart')">
            <div
              v-for="d in days"
              :key="d.date"
              class="group relative flex h-full min-w-[10px] flex-1 flex-col justify-end"
              :title="`${d.date} · ${tx('会话', 'sessions')} ${d.sessions} · ${tx('提问', 'prompts')} ${d.prompts} · Token ${compact(d.tokens)}`"
            >
              <div
                class="w-full rounded-t-sm bg-brand/80 transition-colors group-hover:bg-brand"
                :style="{ height: `${Math.max(2, ((metric === 'tokens' ? d.tokens : d.prompts) / dayMax) * 100)}%` }"
              />
            </div>
          </div>
        </div>
        <div class="mt-2 flex justify-between text-[11px] text-muted-foreground">
          <span>{{ days[0]?.date }}</span>
          <span>{{ days[days.length - 1]?.date }}</span>
        </div>
      </section>

      <div class="wb-grid mb-4">
        <section class="rounded-xl border border-border p-4">
          <h3 class="text-sm">{{ tx('一天中的提问时段', 'Prompts by hour') }}</h3>
          <div class="flex h-24 items-end gap-0.5">
            <div
              v-for="(n, h) in data.hours"
              :key="h"
              class="flex h-full flex-1 flex-col justify-end"
              :title="`${h}:00 · ${n}`"
            >
              <div class="w-full rounded-t-sm bg-foreground/70" :style="{ height: `${n ? Math.max(4, (n / hourMax) * 100) : 0}%` }" />
            </div>
          </div>
          <div class="mt-1 flex justify-between text-[10px] text-muted-foreground">
            <span>0</span><span>6</span><span>12</span><span>18</span><span>23</span>
          </div>
        </section>
        <section class="rounded-xl border border-border p-4">
          <h3 class="text-sm">{{ tx('星期分布', 'By weekday') }}</h3>
          <div class="flex h-24 items-end gap-2">
            <div v-for="(n, i) in data.weekdays" :key="i" class="flex h-full flex-1 flex-col justify-end" :title="`${weekdayNames[i]} · ${n}`">
              <div class="w-full rounded-t-sm bg-foreground/70" :style="{ height: `${n ? Math.max(4, (n / weekdayMax) * 100) : 0}%` }" />
            </div>
          </div>
          <div class="mt-1 flex gap-2 text-center text-[10px] text-muted-foreground">
            <span v-for="name in weekdayNames" :key="name" class="flex-1">{{ name }}</span>
          </div>
        </section>
        <section class="rounded-xl border border-border p-4">
          <h3 class="text-sm">{{ tx('会话长度（每会话提问数）', 'Session length (prompts)') }}</h3>
          <ul class="space-y-2">
            <li v-for="b in data.lengths || []" :key="b.name" class="flex items-center gap-2 text-xs">
              <span class="w-12 shrink-0 tabular-nums text-muted-foreground">{{ b.name }}</span>
              <div class="h-1.5 flex-1 rounded-full bg-muted">
                <div class="h-full rounded-full bg-brand/80" :style="{ width: `${(b.count / lengthTotal) * 100}%` }" />
              </div>
              <span class="w-8 shrink-0 text-right tabular-nums">{{ b.count }}</span>
            </li>
          </ul>
        </section>
      </div>

      <div class="wb-grid mb-4">
        <InsightBars :title="tx('最常用的工具', 'Most used tools')" :items="data.tools" :empty="tx('没有工具调用', 'No tool calls')" />
        <InsightBars :title="tx('Skills 使用', 'Skills used')" :items="data.skills" :empty="tx('这段时间没有调用 Skill', 'No skills used in this range')" />
        <InsightBars :title="tx('MCP 服务器', 'MCP servers')" :items="data.mcp" :empty="tx('没有 MCP 调用', 'No MCP calls')" />
        <InsightBars :title="tx('模型（按 Token）', 'Models (by tokens)')" :items="data.models" by="tokens" :format="tokenFormat" :empty="tx('没有模型用量', 'No model usage')" />
        <InsightBars :title="tx('项目', 'Projects')" :items="projectItems" :format="projectFormat" :empty="tx('没有项目信息', 'No project info')" />
      </div>

      <section class="wb-card">
        <h2>{{ tx('Token 最多的会话', 'Sessions with the most tokens') }}</h2>
        <div class="wb-table">
          <table>
            <thead>
              <tr>
                <th>{{ tx('会话', 'Session') }}</th>
                <th>{{ tx('工具', 'Tool') }}</th>
                <th class="text-right">Token</th>
                <th class="text-right">{{ tx('提问', 'Prompts') }}</th>
                <th class="text-right">{{ tx('工具调用', 'Tool calls') }}</th>
                <th class="text-right">{{ tx('活跃', 'Active') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="s in data.top || []" :key="`${s.provider}:${s.session_id}`">
                <td class="max-w-[360px]">
                  <p class="truncate font-medium" :title="s.title">{{ s.title || s.session_id }}</p>
                  <p class="truncate text-[11px] text-muted-foreground" :title="s.project">{{ s.project }}</p>
                </td>
                <td>{{ s.provider === 'claude' ? 'Claude Code' : 'Codex' }}</td>
                <td class="text-right tabular-nums">{{ compact(s.tokens) }}</td>
                <td class="text-right tabular-nums">{{ s.prompts }}</td>
                <td class="text-right tabular-nums">{{ s.tool_calls }}</td>
                <td class="text-right tabular-nums">{{ s.active_minutes }} {{ tx('分', 'min') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
      <p v-for="w in data.warnings || []" :key="w" class="wb-error">{{ w }}</p>
    </template>
  </template>
</template>
