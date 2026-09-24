<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded">
    <template #header>
      <div>
        <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">{{ t('nav.stats') }}</h1>
        <p class="mt-2 text-sm text-muted-foreground">{{ t('stats.panelHint') }}</p>
      </div>
    </template>
    <template #actions>
      <ToolFilterChips />
      <Select :model-value="String(days)" @update:model-value="onDays">
        <SelectTrigger class="h-9 gap-2 rounded-full border-0 bg-card px-3 shadow-none ring-1 ring-black/[0.06] dark:ring-white/10">
          <Calendar class="size-3.5 text-muted-foreground" />
          <SelectValue />
        </SelectTrigger>
        <SelectContent align="end">
          <SelectItem v-for="item in dayTabs" :key="item.value" :value="item.value">
            {{ item.label }}
          </SelectItem>
        </SelectContent>
      </Select>
      <span v-if="loading" class="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Loader2 class="size-3.5 animate-spin" />
        {{ t('stats.loading') }}
      </span>
      <Button variant="ghost" size="icon-sm" class="rounded-full" :disabled="loading" @click="refresh">
        <RefreshCw :class="['size-3.5', loading && 'animate-spin']" />
      </Button>
    </template>

    <div class="relative min-h-[200px]">
      <div
        v-if="loading"
        class="sticky top-0 z-20 mb-3 flex items-center gap-2 rounded-xl border bg-background/95 px-3 py-2 text-sm text-muted-foreground shadow-sm backdrop-blur"
      >
        <Loader2 class="size-4 animate-spin" />
        {{ t('stats.loadingData') }}
      </div>

      <p v-if="isGatewayPlatform" class="mb-3 rounded-xl bg-muted/50 px-3 py-2 text-xs leading-relaxed text-muted-foreground">
        {{ t('stats.gatewayHint') }}
      </p>

      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <motion.div v-for="(card, i) in kpiItems" :key="card.label" :initial="{ opacity: 0, y: 12 }" :animate="{ opacity: 1, y: 0 }" :transition="{ delay: i * 0.05, duration: 0.28, ease: [0.22, 1, 0.36, 1] }">
          <Card size="sm">
            <CardHeader class="px-5">
              <div class="flex items-start justify-between gap-2">
                <CardDescription>{{ card.label }}</CardDescription>
                <component :is="card.icon" class="size-4 text-muted-foreground" />
              </div>
              <CardTitle class="text-3xl font-semibold tracking-tight">{{ card.value }}</CardTitle>
            </CardHeader>
          </Card>
        </motion.div>
      </div>

      <BudgetCard class="mt-4" />

      <div class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader class="px-5">
            <CardTitle>{{ t('stats.distribution') }}</CardTitle>
          </CardHeader>
          <CardContent class="px-5 pb-5">
            <div v-if="doughnutSlices.length" class="flex flex-col items-center gap-4">
              <div class="relative size-56">
                <Doughnut class="size-full" :data="doughnutData" :options="doughnutOptions" />
                <div class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
                  <span class="text-xs text-muted-foreground">{{ t('stats.totalTokens') }}</span>
                  <span class="text-2xl font-semibold">{{ formatNumber(totalTokens) }}</span>
                </div>
              </div>
              <div class="flex w-full flex-wrap justify-center gap-x-4 gap-y-1.5 text-xs">
                <span v-for="slice in doughnutSlices" :key="slice.label" class="flex items-center gap-1.5 text-muted-foreground">
                  <span class="size-2 rounded-full" :style="{ backgroundColor: slice.color }" />
                  {{ slice.label }} ({{ slice.percent }})
                </span>
              </div>
            </div>
            <Empty v-else class="h-48 border-0">
              <EmptyHeader>
                <ChartLine class="size-8 text-muted-foreground" />
                <EmptyTitle>{{ t('stats.noData') }}</EmptyTitle>
              </EmptyHeader>
            </Empty>
          </CardContent>
        </Card>

        <Card>
          <CardHeader class="px-5">
            <div class="flex items-center justify-between gap-2">
              <CardTitle>{{ t('stats.byModel') }}</CardTitle>
              <div v-if="modelPages > 1" class="flex items-center gap-1 text-xs text-muted-foreground">
                <Button variant="ghost" size="icon-xs" :disabled="modelPage <= 0" @click="modelPage -= 1">
                  <ChevronLeft />
                </Button>
                {{ modelPage + 1 }} / {{ modelPages }}
                <Button variant="ghost" size="icon-xs" :disabled="modelPage + 1 >= modelPages" @click="modelPage += 1">
                  <ChevronRight />
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent class="space-y-3 px-5 pb-5">
            <div
              v-for="item in pagedModels"
              :key="item.name"
              class="rounded-2xl bg-muted/50 px-4 py-3"
            >
              <div class="flex items-center justify-between gap-3">
                <span class="truncate text-sm font-medium">{{ formatModelName(item.name) }}</span>
                <span class="text-xs text-muted-foreground">{{ item.percent }}</span>
              </div>
              <div class="mt-2 flex gap-6 text-xs text-muted-foreground">
                <span>{{ t('stats.requests') }} <span class="font-medium text-foreground">{{ formatNumber(item.stats.requests) }}</span></span>
                <span>Tokens <span class="font-medium text-foreground">{{ formatNumber(item.stats.tokens) }}</span></span>
              </div>
              <Progress :model-value="item.bar" class="mt-2.5 h-1.5" :color="item.color" />
            </div>
            <Empty v-if="pagedModels.length === 0" class="h-40 border-0">
              <EmptyHeader>
                <EmptyTitle>{{ t('stats.noModelData') }}</EmptyTitle>
              </EmptyHeader>
            </Empty>
          </CardContent>
        </Card>
      </div>

      <div class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader class="px-5">
            <CardTitle>{{ t('stats.requestTrend') }}</CardTitle>
            <CardDescription>{{ t('stats.requestTrendDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="px-5 pb-5">
            <div v-if="lineLabels.length" class="h-56">
              <Line class="size-full" :data="requestLineData" :options="lineOptions" />
            </div>
            <Empty v-else class="h-48 border-0">
              <EmptyHeader>
                <EmptyTitle>{{ t('stats.noTrend') }}</EmptyTitle>
              </EmptyHeader>
            </Empty>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="px-5">
            <CardTitle>{{ t('stats.costTrend') }}</CardTitle>
            <CardDescription>{{ t('stats.costTrendDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="px-5 pb-5">
            <div v-if="lineLabels.length" class="h-56">
              <Line class="size-full" :data="costLineData" :options="lineOptions" />
            </div>
            <Empty v-else class="h-48 border-0">
              <EmptyHeader>
                <EmptyTitle>{{ t('stats.noCost') }}</EmptyTitle>
              </EmptyHeader>
            </Empty>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="px-5">
            <CardTitle>{{ t('stats.ioTokens') }}</CardTitle>
            <CardDescription>{{ t('stats.ioTokensDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="px-5 pb-5">
            <div v-if="lineLabels.length" class="h-56">
              <Bar class="size-full" :data="tokenBarData" :options="stackedBarOptions" />
            </div>
            <Empty v-else class="h-48 border-0">
              <EmptyHeader>
                <EmptyTitle>{{ t('stats.noTokens') }}</EmptyTitle>
              </EmptyHeader>
            </Empty>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="px-5">
            <CardTitle>{{ t('stats.modelCost') }}</CardTitle>
            <CardDescription>{{ t('stats.modelCostDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="px-5 pb-5">
            <div v-if="modelCostBars.labels.length" class="h-56">
              <Bar class="size-full" :data="modelCostBars" :options="barOptions" />
            </div>
            <Empty v-else class="h-48 border-0">
              <EmptyHeader>
                <EmptyTitle>{{ t('stats.noModelCost') }}</EmptyTitle>
              </EmptyHeader>
            </Empty>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="px-5">
            <CardTitle>{{ t('stats.envUsage') }}</CardTitle>
            <CardDescription>{{ t('stats.envUsageDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="px-5 pb-5">
            <div v-if="envBarData.labels.length" class="h-56">
              <Bar class="size-full" :data="envBarData" :options="barOptions" />
            </div>
            <Empty v-else class="h-48 border-0">
              <EmptyHeader>
                <EmptyTitle>{{ t('stats.noEnvUsage') }}</EmptyTitle>
              </EmptyHeader>
            </Empty>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="px-5">
            <CardTitle>{{ t('stats.cacheHit') }}</CardTitle>
            <CardDescription>Cache Read vs Cache Write</CardDescription>
          </CardHeader>
          <CardContent class="px-5 pb-5">
            <div v-if="cacheDoughnut.labels.length" class="mx-auto h-56 max-w-xs">
              <Doughnut class="size-full" :data="cacheDoughnut" :options="doughnutOptions" />
            </div>
            <Empty v-else class="h-48 border-0">
              <EmptyHeader>
                <EmptyTitle>{{ t('stats.noCache') }}</EmptyTitle>
              </EmptyHeader>
            </Empty>
          </CardContent>
        </Card>
      </div>

      <Card class="mt-4">
        <CardHeader class="px-5">
          <CardTitle>
            {{ t('stats.heatmap') }}
            <span class="text-xs font-normal text-muted-foreground">{{ t('stats.heatmapRange', { weeks: heatmapWeeks }) }}</span>
          </CardTitle>
        </CardHeader>
        <CardContent class="px-5 pb-5">
        <div class="mb-2 ml-6 flex">
          <div class="relative h-4 flex-1 text-[9px] text-muted-foreground">
            <span
              v-for="(month, idx) in monthLabels"
              :key="idx"
              class="absolute top-0 whitespace-nowrap"
              :style="{ left: month.left, transform: 'translateX(-50%)' }"
            >
              {{ month.name }}
            </span>
          </div>
        </div>
        <div class="flex w-full gap-1">
          <div class="flex w-5 shrink-0 flex-col gap-0.5 pt-0 text-[9px] text-muted-foreground">
            <span class="flex aspect-square items-center" />
            <span class="flex aspect-square items-center">{{ t('stats.mon') }}</span>
            <span class="flex aspect-square items-center" />
            <span class="flex aspect-square items-center">{{ t('stats.wed') }}</span>
            <span class="flex aspect-square items-center" />
            <span class="flex aspect-square items-center">{{ t('stats.fri') }}</span>
            <span class="flex aspect-square items-center" />
          </div>
          <div class="flex min-w-0 flex-1 gap-0.5">
            <div v-for="(week, weekIdx) in heatmapGrid" :key="weekIdx" class="flex min-w-0 flex-1 flex-col gap-0.5">
              <AppTooltip
                v-for="(day, dayIdx) in week"
                :key="dayIdx"
                :content="day.date ? t('stats.heatmapTip', { date: day.date, requests: day.requests, tokens: formatNumber(day.tokens), cost: formatCost(day.cost) }) : ''"
                :disabled="!day.date"
                wrap
                class="block w-full"
              >
                <div
                  class="aspect-square w-full rounded-[2px]"
                  :class="day.date ? 'cursor-pointer hover:outline hover:outline-2 hover:outline-primary' : 'cursor-default'"
                  :style="{ backgroundColor: day.date ? getHeatmapColor(day.requests) : 'transparent', outline: day.isToday ? '2px solid var(--primary)' : undefined }"
                />
              </AppTooltip>
            </div>
          </div>
        </div>
        <div class="mt-2.5 flex items-center justify-end gap-1.5">
          <span class="text-xs text-muted-foreground">{{ t('stats.less') }}</span>
          <div class="flex gap-0.5">
            <div class="size-[11px] rounded-[2px]" :style="{ backgroundColor: getHeatmapColor(0) }" />
            <div class="size-[11px] rounded-[2px]" :style="{ backgroundColor: getHeatmapColor(3) }" />
            <div class="size-[11px] rounded-[2px]" :style="{ backgroundColor: getHeatmapColor(10) }" />
            <div class="size-[11px] rounded-[2px]" :style="{ backgroundColor: getHeatmapColor(25) }" />
            <div class="size-[11px] rounded-[2px]" :style="{ backgroundColor: getHeatmapColor(50) }" />
          </div>
          <span class="text-xs text-muted-foreground">{{ t('stats.more') }}</span>
        </div>
        </CardContent>
      </Card>

      <div class="mt-4 rounded-2xl bg-muted/50 p-3">
        <div class="flex items-center gap-2 text-xs text-muted-foreground">
          <FolderOpen class="size-3.5" />
          <span>{{ t('stats.source') }}</span>
          <code class="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px]">{{ logDirectory || t('stats.notDetected') }}</code>
        </div>
      </div>
    </div>

    <template v-if="!embedded" #footer>
      <Button variant="secondary" @click="isOpen = false">{{ t('stats.close') }}</Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { ref, computed, watch } from 'vue'
import { motion } from 'motion-v'
import { Activity, Calendar, ChartLine, ChevronLeft, ChevronRight, Coins, Database, FolderOpen, Loader2, RefreshCw } from '@lucide/vue'
import { Bar, Doughnut, Line } from 'vue-chartjs'
import {
  Chart as ChartJS,
  ArcElement,
  BarElement,
  CategoryScale,
  Filler,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Tooltip,
} from 'chart.js'
import AppModal from '@/components/common/AppModal.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import ToolFilterChips from '@/components/layout/ToolFilterChips.vue'
import BudgetCard from './BudgetCard.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Progress } from '@/components/ui/progress'
import { getStatsOverview, getUsageStats, getHeatmapData, getLogDirectory, getEnvUsageSummary, type UsageStats, type HeatmapData, type ModelStats, type StatsPlatform, type EnvUsageSummary } from '@/services/logService'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'

const { t, locale } = useI18n()

ChartJS.register(ArcElement, BarElement, CategoryScale, Filler, Legend, LinearScale, LineElement, PointElement, Tooltip)

const CHART_COLORS = ['#8B7CF6', '#5B9CF6', '#F472B6', '#F5C16C', '#2DD4BF', '#C4B5FD', '#94A3B8']
const dayTabs = computed(() => [
  { value: '1', label: t('stats.today') },
  { value: '7', label: t('stats.last7') },
  { value: '30', label: t('stats.last30') },
  { value: '0', label: t('stats.all') },
])

interface Props {
  modelValue: boolean
  embedded?: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const configStore = useConfigStore()
const { error: toastError } = useToast()
const loading = ref(false)
const days = ref(7)
const platform = ref<StatsPlatform>('all')
// 这几个平台没有本地用量日志，统计的是经过本地网关的请求
const GATEWAY_PLATFORMS: StatsPlatform[] = ['claude_desktop', 'opencode', 'grok']
const STATS_PLATFORMS: StatsPlatform[] = ['claude', 'codex', 'antigravity', ...GATEWAY_PLATFORMS]
const isGatewayPlatform = computed(() => GATEWAY_PLATFORMS.includes(platform.value))
const stats = ref<UsageStats | null>(null)
const heatmap = ref<HeatmapData[]>([])
const envSummary = ref<Record<string, EnvUsageSummary>>({})
const logDirectory = ref('')
const heatmapWeeks = 26



const totalTokens = computed(() => {
  if (!stats.value) return 0
  return (stats.value.total_input_tokens || 0) + (stats.value.total_output_tokens || 0)
})

const kpiItems = computed(() => {
  const requests = stats.value?.total_requests || 0
  const cost = stats.value?.total_cost || 0
  const cacheRead = stats.value?.total_cache_read || 0
  const cacheWrite = stats.value?.total_cache_write || 0
  const input = stats.value?.total_input_tokens || 0
  const output = stats.value?.total_output_tokens || 0
  return [
    { label: t('stats.kpi.requests'), value: formatNumber(requests), icon: Activity },
    { label: t('stats.kpi.tokens'), value: formatNumber(totalTokens.value), icon: ChartLine },
    { label: t('stats.kpi.cost'), value: `$${formatCost(cost)}`, icon: Coins },
    { label: t('stats.kpi.avgCost'), value: `$${formatCost(requests ? cost / requests : 0)}`, icon: Coins },
    { label: t('stats.kpi.input'), value: formatNumber(input), icon: ChartLine },
    { label: t('stats.kpi.output'), value: formatNumber(output), icon: ChartLine },
    { label: 'Cache Read', value: formatNumber(cacheRead), icon: Database },
    { label: 'Cache Write', value: formatNumber(cacheWrite), icon: Database },
  ]
})

const sortedModelStats = computed(() => {
  if (!stats.value?.by_model) return {}
  const entries = Object.entries(stats.value.by_model) as [string, ModelStats][]
  const filtered = entries.filter(([model]) =>
    model && !model.includes('synthetic') && !model.startsWith('<')
  )
  filtered.sort((a, b) => b[1].cost - a[1].cost)
  return Object.fromEntries(filtered)
})

const modelEntries = computed(() => {
  return Object.entries(sortedModelStats.value).map(([name, modelStats], i) => {
    const tokens = modelStats.tokens || 0
    const percentNum = totalTokens.value > 0 ? (tokens / totalTokens.value) * 100 : 0
    return {
      name,
      stats: modelStats,
      color: CHART_COLORS[i % CHART_COLORS.length],
      percent: `${percentNum.toFixed(1)}%`,
      bar: Math.min(100, Math.max(4, percentNum)),
    }
  })
})

const doughnutSlices = computed(() => {
  const entries = modelEntries.value
  const top = entries.slice(0, 6)
  const rest = entries.slice(6)
  const restTokens = rest.reduce((sum, item) => sum + item.stats.tokens, 0)
  const slices = top.map((item) => ({
    label: formatModelName(item.name),
    value: item.stats.tokens,
    color: item.color,
    percent: item.percent,
  }))
  if (restTokens > 0) {
    const percent = totalTokens.value > 0 ? ((restTokens / totalTokens.value) * 100).toFixed(1) : '0.0'
    slices.push({ label: t('stats.other'), value: restTokens, color: CHART_COLORS[6], percent: `${percent}%` })
  }
  return slices
})

const doughnutData = computed(() => ({
  labels: doughnutSlices.value.map(s => s.label),
  datasets: [{
    data: doughnutSlices.value.map(s => s.value),
    backgroundColor: doughnutSlices.value.map(s => s.color),
    borderWidth: 0,
    hoverOffset: 10,
  }],
}))

const lineLabels = computed(() => (stats.value?.series || []).map(item => item.hour.slice(5)))

const requestLineData = computed(() => ({
  labels: lineLabels.value,
  datasets: [{
    label: t('stats.requests'),
    data: (stats.value?.series || []).map(item => item.requests),
    borderColor: CHART_COLORS[0],
    backgroundColor: 'rgba(139, 124, 246, 0.15)',
    fill: true,
    tension: 0.35,
    pointRadius: 0,
    pointHoverRadius: 4,
    pointHitRadius: 16,
  }],
}))

const costLineData = computed(() => ({
  labels: lineLabels.value,
  datasets: [{
    label: t('stats.cost'),
    data: (stats.value?.series || []).map(item => Number(item.cost.toFixed(4))),
    borderColor: CHART_COLORS[3],
    backgroundColor: 'rgba(245, 193, 108, 0.18)',
    fill: true,
    tension: 0.35,
    pointRadius: 0,
    pointHoverRadius: 4,
    pointHitRadius: 16,
  }],
}))

const tokenBarData = computed(() => ({
  labels: lineLabels.value,
  datasets: [
    {
      label: t('stats.input'),
      data: (stats.value?.series || []).map(item => item.input_tokens),
      backgroundColor: CHART_COLORS[1],
      stack: 'tokens',
    },
    {
      label: t('stats.output'),
      data: (stats.value?.series || []).map(item => item.output_tokens),
      backgroundColor: CHART_COLORS[2],
      stack: 'tokens',
    },
  ],
}))

const modelCostBars = computed(() => {
  const top = modelEntries.value.slice(0, 8)
  return {
    labels: top.map(item => formatModelName(item.name)),
    datasets: [{
      label: t('stats.cost'),
      data: top.map(item => Number(item.stats.cost.toFixed(4))),
      backgroundColor: top.map(item => item.color),
    }],
  }
})

const envBarData = computed(() => {
  const entries = Object.entries(envSummary.value || {}).sort((a, b) => b[1].requests - a[1].requests).slice(0, 8)
  return {
    labels: entries.map(([name]) => name),
    datasets: [{
      label: t('stats.requests'),
      data: entries.map(([, item]) => item.requests),
      backgroundColor: CHART_COLORS[0],
    }],
  }
})

const cacheDoughnut = computed(() => {
  const read = stats.value?.total_cache_read || 0
  const write = stats.value?.total_cache_write || 0
  if (!read && !write) return { labels: [] as string[], datasets: [] }
  return {
    labels: ['Cache Read', 'Cache Write'],
    datasets: [{
      data: [read, write],
      backgroundColor: [CHART_COLORS[5], CHART_COLORS[3]],
      borderWidth: 0,
      hoverOffset: 10,
    }],
  }
})

const axisTick = { color: 'oklch(0.55 0 0)' }
const axisGrid = { color: 'oklch(0.9 0 0 / 0.4)' }
const indexHover = {
  mode: 'index' as const,
  intersect: false,
}

const lineOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: indexHover,
  plugins: {
    legend: { display: false },
    tooltip: { ...indexHover },
  },
  scales: {
    x: { ticks: { maxTicksLimit: 8, ...axisTick }, grid: { display: false } },
    y: { ticks: axisTick, grid: axisGrid },
  },
}

const barOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: indexHover,
  plugins: {
    legend: { display: false },
    tooltip: { ...indexHover },
  },
  scales: {
    x: { ticks: axisTick, grid: { display: false } },
    y: { ticks: axisTick, grid: axisGrid },
  },
}

const stackedBarOptions = {
  ...barOptions,
  scales: {
    x: { stacked: true, ticks: { maxTicksLimit: 8, ...axisTick }, grid: { display: false } },
    y: { stacked: true, ticks: axisTick, grid: axisGrid },
  },
}

const doughnutOptions = {
  responsive: true,
  maintainAspectRatio: false,
  cutout: '72%',
  interaction: { mode: 'nearest' as const, intersect: true },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (ctx: { label?: string; parsed: number }) => `${ctx.label}: ${formatNumber(ctx.parsed)}`,
      },
    },
  },
}

const modelPage = ref(0)
const modelPageSize = 3
const modelPages = computed(() => Math.max(1, Math.ceil(modelEntries.value.length / modelPageSize)))
const pagedModels = computed(() => {
  const start = modelPage.value * modelPageSize
  return modelEntries.value.slice(start, start + modelPageSize)
})

watch(modelPages, (pages) => {
  if (modelPage.value >= pages) modelPage.value = Math.max(0, pages - 1)
})

interface HeatmapCell {
  date: string
  requests: number
  tokens: number
  cost: number
  isToday: boolean
}

const heatmapGrid = computed(() => {
  const grid: HeatmapCell[][] = []
  const today = new Date()
  const heatmapMap = new Map<string, HeatmapData>()
  heatmap.value.forEach(d => heatmapMap.set(d.date, d))
  const startDate = new Date(today)
  startDate.setDate(startDate.getDate() - (heatmapWeeks * 7) - today.getDay() + 1)
  const currentDate = new Date(startDate)
  for (let week = 0; week < heatmapWeeks; week++) {
    const weekCells: HeatmapCell[] = []
    for (let day = 0; day < 7; day++) {
      const dateStr = currentDate.toISOString().split('T')[0]
      const data = heatmapMap.get(dateStr)
      const isToday = dateStr === today.toISOString().split('T')[0]
      if (currentDate <= today) {
        weekCells.push({
          date: dateStr,
          requests: data?.requests || 0,
          tokens: data?.tokens || 0,
          cost: data?.cost || 0,
          isToday
        })
      } else {
        weekCells.push({ date: '', requests: 0, tokens: 0, cost: 0, isToday: false })
      }
      currentDate.setDate(currentDate.getDate() + 1)
    }
    grid.push(weekCells)
  }
  return grid
})

const monthLabels = computed(() => {
  const labels: { name: string; left: string }[] = []
  const today = new Date()
  const startDate = new Date(today)
  startDate.setDate(startDate.getDate() - (heatmapWeeks * 7) - today.getDay() + 1)
  let lastMonth = -1
  const currentDate = new Date(startDate)
  for (let week = 0; week < heatmapWeeks; week++) {
    const month = currentDate.getMonth()
    if (month !== lastMonth) {
      const leftPercent = (week / heatmapWeeks) * 100
      const name = new Date(2000, month, 1).toLocaleDateString(locale.value === 'en' ? 'en-US' : 'zh-CN', { month: 'short' })
      labels.push({ name, left: `${leftPercent}%` })
      lastMonth = month
    }
    currentDate.setDate(currentDate.getDate() + 7)
  }
  return labels
})

function formatNumber(num: number): string {
  if (num >= 1_000_000) return (num / 1_000_000).toFixed(1) + 'M'
  if (num >= 1_000) return (num / 1_000).toFixed(1) + 'K'
  return num.toLocaleString()
}

function formatCost(cost: number): string {
  if (cost >= 1) return cost.toFixed(2)
  if (cost >= 0.01) return cost.toFixed(3)
  return cost.toFixed(4)
}

// 按 Claude 模型 ID 的命名规则解析：claude-opus-4-8、claude-sonnet-4-5-20250929、
// claude-fable-5-1，以及旧式的 claude-3-5-sonnet-20241022。末尾的 8 位日期不是版本号。
function formatClaudeModelName(model: string): string | null {
  const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1)
  const current = model.match(/claude-(opus|sonnet|haiku|fable|mythos)-(\d+)(?:-(\d{1,2}))?(?!\d)/)
  if (current) return `${cap(current[1])} ${current[2]}${current[3] ? `.${current[3]}` : ''}`
  const legacy = model.match(/claude-(\d+)(?:-(\d+))?-(opus|sonnet|haiku)/)
  if (legacy) return `${cap(legacy[3])} ${legacy[1]}${legacy[2] ? `.${legacy[2]}` : ''}`
  return null
}

function formatModelName(model: string): string {
  const claude = formatClaudeModelName(model)
  if (claude) return claude
  if (model.includes('gpt-4o-mini')) return 'GPT-4o Mini'
  if (model.includes('gpt-4o')) return 'GPT-4o'
  if (model.includes('gpt-4-turbo')) return 'GPT-4 Turbo'
  if (model.includes('gpt-4')) return 'GPT-4'
  if (model.includes('gpt-5.2-codex')) return 'GPT-5.2 Codex'
  if (model.includes('gpt-5.2')) return 'GPT-5.2'
  if (model.includes('gpt-5.1-codex-mini')) return 'GPT-5.1 Codex Mini'
  if (model.includes('gpt-5.1-codex-max')) return 'GPT-5.1 Codex Max'
  if (model.includes('gpt-5.1-codex')) return 'GPT-5.1 Codex'
  if (model.includes('gpt-5.1')) return 'GPT-5.1'
  if (model.includes('gpt-5-codex')) return 'GPT-5 Codex'
  if (model.includes('gpt-5')) return 'GPT-5'
  if (model.includes('codex-1')) return 'Codex-1'
  if (model.includes('gemini-2.0')) return 'Gemini 2.0'
  if (model.includes('gemini-1.5-pro')) return 'Gemini 1.5 Pro'
  if (model.includes('gemini-1.5-flash')) return 'Gemini 1.5 Flash'
  return model.split('-').slice(0, 2).join(' ')
}

function getHeatmapColor(requests: number): string {
  if (requests === 0) return 'var(--heatmap-0, #ebedf0)'
  if (requests < 5) return 'var(--heatmap-1, #9be9a8)'
  if (requests < 15) return 'var(--heatmap-2, #40c463)'
  if (requests < 30) return 'var(--heatmap-3, #30a14e)'
  return 'var(--heatmap-4, #216e39)'
}

function compactSeries(series: NonNullable<UsageStats['series']>, maxPoints = 56) {
  if (!series.length || series.length <= maxPoints) return series
  const byDay = new Map<string, (typeof series)[number]>()
  for (const item of series) {
    const key = item.hour.slice(0, 10)
    const prev = byDay.get(key)
    if (!prev) {
      byDay.set(key, { ...item, hour: key })
      continue
    }
    prev.requests += item.requests
    prev.input_tokens += item.input_tokens
    prev.output_tokens += item.output_tokens
    prev.cost += item.cost
  }
  const daily = Array.from(byDay.values())
  if (daily.length <= maxPoints) return daily
  const step = Math.ceil(daily.length / maxPoints)
  return daily.filter((_, index) => index % step === 0)
}

function onDays(value: unknown) {
  if (typeof value !== 'string') return
  const d = Number(value)
  if (Number.isNaN(d)) return
  setDays(d)
}

async function setDays(d: number) {
  days.value = d
  await loadData()
}

async function refresh() {
  await loadData()
}

async function loadData() {
  loading.value = true
  try {
    const data = await getStatsOverview(days.value, heatmapWeeks * 7, platform.value)
    const nextStats = data.stats
    if (nextStats?.series) nextStats.series = compactSeries(nextStats.series)
    stats.value = nextStats
    heatmap.value = data.heatmap || []
    logDirectory.value = data.log_directory || ''
    envSummary.value = data.env_summary != null
      ? data.env_summary
      : await getEnvUsageSummary(days.value).catch(() => ({}))
  } catch {
    try {
      const [statsData, heatmapData] = await Promise.all([
        getUsageStats(days.value, platform.value),
        getHeatmapData(heatmapWeeks * 7, platform.value),
      ])
      if (statsData?.series) statsData.series = compactSeries(statsData.series)
      stats.value = statsData
      heatmap.value = heatmapData || []
      logDirectory.value = await getLogDirectory().catch(() => '')
      envSummary.value = await getEnvUsageSummary(days.value).catch(() => ({}))
    } catch (err) {
      toastError(t('stats.loadFailed', { error: err instanceof Error ? err.message : String(err) }))
    }
  } finally {
    loading.value = false
  }
}

watch(isOpen, (open) => {
  if (open) {
    modelPage.value = 0
    loadData()
  }}, { immediate: true })

watch(() => configStore.currentFilter, (tool) => {
  const next: StatsPlatform = STATS_PLATFORMS.find(item => item === tool) ?? 'all'
  if (platform.value === next) return
  platform.value = next
  if (isOpen.value) loadData()
}, { immediate: true })
</script>
