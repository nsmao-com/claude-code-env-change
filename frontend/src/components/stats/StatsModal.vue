<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded">
    <template #header>
      <div>
        <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">统计</h1>
        <p class="mt-2 text-sm text-muted-foreground">查看各平台请求量、Token 消耗与花费估算。</p>
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
      <Button variant="ghost" size="icon-sm" class="rounded-full" :disabled="loading" @click="refresh">
        <RefreshCw :class="['size-3.5', loading && 'animate-spin']" />
      </Button>
    </template>

    <div class="relative min-h-[200px]">
      <div v-if="loading" class="absolute inset-0 z-10 flex flex-col items-center justify-center rounded-2xl bg-background/90">
        <Loader2 class="size-6 animate-spin text-muted-foreground" />
        <span class="mt-2 text-sm text-muted-foreground">加载中...</span>
      </div>

      <div class="grid grid-cols-3 gap-3" :class="{ 'pointer-events-none opacity-30': loading }">
        <motion.div v-for="(card, i) in kpiItems" :key="card.label" :initial="{ opacity: 0, y: 12 }" :animate="{ opacity: 1, y: 0 }" :transition="{ delay: i * 0.05, duration: 0.28, ease: [0.22, 1, 0.36, 1] }">
          <Card size="sm">
            <CardHeader class="px-5">
              <div class="flex items-start justify-between">
                <CardDescription>{{ card.label }}</CardDescription>
                <component :is="card.icon" class="size-4 text-muted-foreground" />
              </div>
              <CardTitle class="text-3xl font-semibold tracking-tight">{{ card.value }}</CardTitle>
            </CardHeader>
          </Card>
        </motion.div>
      </div>

      <div class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2" :class="{ 'pointer-events-none opacity-30': loading }">
        <Card>
          <CardHeader class="px-5">
            <CardTitle>用量分布</CardTitle>
          </CardHeader>
          <CardContent class="px-5 pb-5">
            <div v-if="doughnutSlices.length" class="flex flex-col items-center gap-4">
              <div class="relative size-56">
                <Doughnut class="size-full" :data="doughnutData" :options="doughnutOptions" />
                <div class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
                  <span class="text-xs text-muted-foreground">总 Tokens</span>
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
                <EmptyTitle>暂无数据</EmptyTitle>
              </EmptyHeader>
            </Empty>
          </CardContent>
        </Card>

        <Card>
          <CardHeader class="px-5">
            <div class="flex items-center justify-between">
              <CardTitle>按模型</CardTitle>
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
                <span>请求 <span class="font-medium text-foreground">{{ formatNumber(item.stats.requests) }}</span></span>
                <span>Tokens <span class="font-medium text-foreground">{{ formatNumber(item.stats.tokens) }}</span></span>
              </div>
              <Progress :model-value="item.bar" class="mt-2.5 h-1.5" :color="item.color" />
            </div>
            <Empty v-if="pagedModels.length === 0" class="h-40 border-0">
              <EmptyHeader>
                <EmptyTitle>暂无模型数据</EmptyTitle>
              </EmptyHeader>
            </Empty>
          </CardContent>
        </Card>
      </div>

      <Card class="mt-4">
        <CardHeader class="px-5">
          <CardTitle>
            活动热力图
            <span class="text-xs font-normal text-muted-foreground">(最近 {{ heatmapWeeks }} 周)</span>
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
            <span class="flex aspect-square items-center">一</span>
            <span class="flex aspect-square items-center" />
            <span class="flex aspect-square items-center">三</span>
            <span class="flex aspect-square items-center" />
            <span class="flex aspect-square items-center">五</span>
            <span class="flex aspect-square items-center" />
          </div>
          <div class="flex min-w-0 flex-1 gap-0.5">
            <div v-for="(week, weekIdx) in heatmapGrid" :key="weekIdx" class="flex min-w-0 flex-1 flex-col gap-0.5">
              <div
                v-for="(day, dayIdx) in week"
                :key="dayIdx"
                class="aspect-square w-full rounded-[2px]"
                :class="day.date ? 'cursor-pointer hover:outline hover:outline-2 hover:outline-primary' : 'cursor-default'"
                :style="{ backgroundColor: day.date ? getHeatmapColor(day.requests) : 'transparent', outline: day.isToday ? '2px solid var(--primary)' : undefined }"
                :title="day.date ? `${day.date}: ${day.requests} 次请求, ${formatNumber(day.tokens)} tokens, $${formatCost(day.cost)}` : ''"
              />
            </div>
          </div>
        </div>
        <div class="mt-2.5 flex items-center justify-end gap-1.5">
          <span class="text-xs text-muted-foreground">少</span>
          <div class="flex gap-0.5">
            <div class="size-[11px] rounded-[2px]" :style="{ backgroundColor: getHeatmapColor(0) }" />
            <div class="size-[11px] rounded-[2px]" :style="{ backgroundColor: getHeatmapColor(3) }" />
            <div class="size-[11px] rounded-[2px]" :style="{ backgroundColor: getHeatmapColor(10) }" />
            <div class="size-[11px] rounded-[2px]" :style="{ backgroundColor: getHeatmapColor(25) }" />
            <div class="size-[11px] rounded-[2px]" :style="{ backgroundColor: getHeatmapColor(50) }" />
          </div>
          <span class="text-xs text-muted-foreground">多</span>
        </div>
        </CardContent>
      </Card>

      <div class="mt-4 rounded-2xl bg-muted/50 p-3">
        <div class="flex items-center gap-2 text-xs text-muted-foreground">
          <FolderOpen class="size-3.5" />
          <span>数据来源:</span>
          <code class="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px]">{{ logDirectory || '未检测到' }}</code>
        </div>
      </div>
    </div>

    <template v-if="!embedded" #footer>
      <Button variant="secondary" @click="isOpen = false">关闭</Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { motion } from 'motion-v'
import { Activity, Calendar, ChartLine, ChevronLeft, ChevronRight, Coins, FolderOpen, Loader2, RefreshCw } from '@lucide/vue'
import { Doughnut } from 'vue-chartjs'
import {
  Chart as ChartJS,
  ArcElement,
  Tooltip,
  Legend
} from 'chart.js'
import AppModal from '@/components/common/AppModal.vue'
import ToolFilterChips from '@/components/layout/ToolFilterChips.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Progress } from '@/components/ui/progress'
import { getStatsOverview, getUsageStats, getHeatmapData, getLogDirectory, type UsageStats, type HeatmapData, type ModelStats, type StatsPlatform } from '@/services/logService'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'

ChartJS.register(ArcElement, Tooltip, Legend)

const CHART_COLORS = ['#8B7CF6', '#5B9CF6', '#F472B6', '#F5C16C', '#2DD4BF', '#C4B5FD', '#94A3B8']
const dayTabs = [
  { value: '1', label: '今天' },
  { value: '7', label: '近 7 天' },
  { value: '30', label: '近 30 天' },
  { value: '0', label: '全部' },
]

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
const stats = ref<UsageStats | null>(null)
const heatmap = ref<HeatmapData[]>([])
const logDirectory = ref('')
const heatmapWeeks = 26



const totalTokens = computed(() => {
  if (!stats.value) return 0
  return (stats.value.total_input_tokens || 0) + (stats.value.total_output_tokens || 0)
})

const kpiItems = computed(() => [
  { label: '总请求', value: formatNumber(stats.value?.total_requests || 0), icon: Activity },
  { label: '总 Tokens', value: formatNumber(totalTokens.value), icon: ChartLine },
  { label: '总花费', value: `$${formatCost(stats.value?.total_cost || 0)}`, icon: Coins },
])

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
    slices.push({ label: '其他', value: restTokens, color: CHART_COLORS[6], percent: `${percent}%` })
  }
  return slices
})

const doughnutData = computed(() => ({
  labels: doughnutSlices.value.map(s => s.label),
  datasets: [{
    data: doughnutSlices.value.map(s => s.value),
    backgroundColor: doughnutSlices.value.map(s => s.color),
    borderWidth: 0,
    hoverOffset: 4,
  }],
}))

const doughnutOptions = {
  responsive: true,
  maintainAspectRatio: false,
  cutout: '72%',
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
      const monthNames = ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月']
      const leftPercent = (week / heatmapWeeks) * 100
      labels.push({ name: monthNames[month], left: `${leftPercent}%` })
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

function formatModelName(model: string): string {
  if (model.includes('opus-4-5')) return 'Opus 4.5'
  if (model.includes('opus-4-1')) return 'Opus 4.1'
  if (model.includes('opus-4')) return 'Opus 4'
  if (model.includes('opus')) return 'Opus'
  if (model.includes('sonnet-4-5')) return 'Sonnet 4.5'
  if (model.includes('sonnet-4')) return 'Sonnet 4'
  if (model.includes('3-7-sonnet')) return 'Sonnet 3.7'
  if (model.includes('3-5-sonnet')) return 'Sonnet 3.5'
  if (model.includes('3-5-haiku')) return 'Haiku 3.5'
  if (model.includes('haiku')) return 'Haiku'
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
    stats.value = data.stats
    heatmap.value = data.heatmap || []
    logDirectory.value = data.log_directory || ''
  } catch {
    // 后端未重新编译（没有合并接口）时，回退到原有两个接口
    try {
      const [statsData, heatmapData] = await Promise.all([
        getUsageStats(days.value, platform.value),
        getHeatmapData(heatmapWeeks * 7, platform.value),
      ])
      stats.value = statsData
      heatmap.value = heatmapData
      logDirectory.value = await getLogDirectory().catch(() => '')
    } catch (err) {
      console.error('统计数据加载失败', err)
      toastError(`统计数据加载失败: ${err instanceof Error ? err.message : String(err)}`)
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
  const next: StatsPlatform = tool === 'claude' || tool === 'codex' || tool === 'gemini' ? tool : 'all'
  if (platform.value === next) return
  platform.value = next
  if (isOpen.value) loadData()
}, { immediate: true })
</script>
