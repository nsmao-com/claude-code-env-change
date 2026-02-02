<template>
  <AppModal v-model="isOpen" title="使用统计" size="xl">
    <!-- Platform Tabs with Glider -->
    <div class="relative mb-4 p-1 bg-secondary/50 border border-border/50 rounded-full inline-flex backdrop-blur-sm">
      <!-- Glider -->
      <div
        class="absolute top-1 bottom-1 bg-white rounded-full transition-all duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] shadow-[0_2px_8px_rgba(0,0,0,0.08)] border border-black/5 dark:border-white/10"
        :style="gliderStyle"
      ></div>

      <button
        v-for="p in platforms"
        :key="p.value"
        ref="tabRefs"
        :disabled="loading"
        :class="['relative z-10 px-5 py-1.5 rounded-full text-xs font-bold uppercase tracking-wide transition-colors duration-200 flex items-center gap-1.5', { 'text-foreground': platform === p.value, 'text-muted-foreground hover:text-foreground/80': platform !== p.value, 'opacity-50 cursor-not-allowed': loading }]"
        @click="setPlatform(p.value)"
      >
        <i :class="p.icon"></i>
        {{ p.label }}
      </button>
    </div>

    <!-- Content with Loading Overlay -->
    <div class="stats-content">
      <!-- Loading Overlay -->
      <div v-if="loading" class="loading-overlay">
        <div class="loading-spinner">
          <i class="fas fa-circle-notch fa-spin text-2xl text-primary"></i>
          <span class="text-sm text-muted-foreground mt-2">加载中...</span>
        </div>
      </div>

      <!-- Stats Summary Cards -->
      <div class="stats-summary" :class="{ 'opacity-30 pointer-events-none': loading }">
      <div class="stat-card">
        <div class="stat-label">总请求</div>
        <div class="stat-value">{{ formatNumber(stats?.total_requests || 0) }}</div>
        <div class="stat-hint">最近 {{ days }} 天</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">总 Tokens</div>
        <div class="stat-value">{{ formatNumber(totalTokens) }}</div>
        <div class="stat-hint">输入 + 输出</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">缓存读取</div>
        <div class="stat-value">{{ formatNumber(stats?.total_cache_read || 0) }}</div>
        <div class="stat-hint">节省成本</div>
      </div>
      <div class="stat-card highlight">
        <div class="stat-label">总花费</div>
        <div class="stat-value">${{ formatCost(stats?.total_cost || 0) }}</div>
        <div class="stat-hint">估算值</div>
      </div>
    </div>

    <!-- Time Range Selector -->
    <div class="flex items-center justify-between my-4">
      <div class="flex gap-2">
        <button
          v-for="d in [1, 7, 30]"
          :key="d"
          :class="['btn btn-compact', days === d ? 'btn-primary' : 'btn-secondary']"
          @click="setDays(d)"
        >
          {{ d === 1 ? '今天' : `${d}天` }}
        </button>
      </div>
      <button class="btn btn-ghost btn-compact" :disabled="loading" @click="refresh">
        <i :class="['fas fa-sync-alt', { 'fa-spin': loading }]"></i>
      </button>
    </div>

    <!-- Chart -->
    <div class="chart-container" v-if="stats?.series?.length">
      <Line :data="chartData" :options="chartOptions" />
    </div>
    <div v-else class="empty-state">
      <i class="fas fa-chart-line text-3xl mb-2 opacity-50"></i>
      <p class="text-sm text-muted-foreground">暂无数据</p>
    </div>

    <!-- Model Breakdown -->
    <div v-if="stats?.by_model && Object.keys(stats.by_model).length" class="mt-6">
      <h3 class="text-sm font-medium mb-3">按模型统计</h3>
      <div class="model-breakdown">
        <div v-for="(modelStats, model) in sortedModelStats" :key="model" class="model-item">
          <div class="model-info">
            <span class="model-name">{{ formatModelName(String(model)) }}</span>
            <span class="model-full-name">{{ model }}</span>
          </div>
          <div class="model-stats">
            <span class="model-stat">
              <i class="fas fa-exchange-alt"></i>
              {{ formatNumber(modelStats.requests) }} 次
            </span>
            <span class="model-stat">
              <i class="fas fa-coins"></i>
              {{ formatNumber(modelStats.tokens) }}
            </span>
            <span class="model-stat cost">
              ${{ formatCost(modelStats.cost) }}
            </span>
          </div>
          <div class="model-bar">
            <div
              class="model-bar-fill"
              :style="{ width: `${(modelStats.cost / maxModelCost) * 100}%` }"
            ></div>
          </div>
        </div>
      </div>
    </div>

    <!-- GitHub-style Heatmap -->
    <div class="mt-6">
      <h3 class="text-sm font-medium mb-3">活动热力图 <span class="text-muted-foreground text-xs font-normal">(最近 {{ heatmapWeeks }} 周)</span></h3>

      <!-- Month labels -->
      <div class="heatmap-months">
        <div class="heatmap-day-labels"></div>
        <div class="heatmap-month-row">
          <span
            v-for="(month, idx) in monthLabels"
            :key="idx"
            class="month-label"
            :style="{ left: month.left }"
          >
            {{ month.name }}
          </span>
        </div>
      </div>

      <div class="heatmap-container">
        <!-- Day of week labels -->
        <div class="heatmap-day-labels">
          <span></span>
          <span>一</span>
          <span></span>
          <span>三</span>
          <span></span>
          <span>五</span>
          <span></span>
        </div>

        <!-- Heatmap grid -->
        <div class="heatmap-grid">
          <div
            v-for="(week, weekIdx) in heatmapGrid"
            :key="weekIdx"
            class="heatmap-week"
          >
            <div
              v-for="(day, dayIdx) in week"
              :key="dayIdx"
              class="heatmap-cell"
              :class="{
                'empty': !day.date,
                'today': day.isToday
              }"
              :style="{ backgroundColor: day.date ? getHeatmapColor(day.requests) : 'transparent' }"
              :title="day.date ? `${day.date}: ${day.requests} 次请求, ${formatNumber(day.tokens)} tokens, $${formatCost(day.cost)}` : ''"
            ></div>
          </div>
        </div>
      </div>

      <!-- Legend -->
      <div class="heatmap-legend">
        <span class="text-xs text-muted-foreground">少</span>
        <div class="legend-scale">
          <div class="legend-item" :style="{ backgroundColor: getHeatmapColor(0) }"></div>
          <div class="legend-item" :style="{ backgroundColor: getHeatmapColor(3) }"></div>
          <div class="legend-item" :style="{ backgroundColor: getHeatmapColor(10) }"></div>
          <div class="legend-item" :style="{ backgroundColor: getHeatmapColor(25) }"></div>
          <div class="legend-item" :style="{ backgroundColor: getHeatmapColor(50) }"></div>
        </div>
        <span class="text-xs text-muted-foreground">多</span>
      </div>
    </div>

    <!-- Log Directory Info -->
    <div class="mt-6 p-3 rounded-lg bg-muted/50">
      <div class="flex items-center gap-2 text-xs text-muted-foreground">
        <i class="fas fa-folder-open"></i>
        <span>数据来源:</span>
        <code class="font-mono text-[10px] bg-muted px-1.5 py-0.5 rounded">{{ logDirectory || '未检测到' }}</code>
      </div>
    </div>
    </div>

    <template #footer>
      <button class="btn btn-secondary" @click="isOpen = false">关闭</button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { Line } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import AppModal from '@/components/common/AppModal.vue'
import { getUsageStats, getHeatmapData, getLogDirectory, type UsageStats, type HeatmapData, type ModelStats, type StatsPlatform } from '@/services/logService'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)

interface Props {
  modelValue: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const loading = ref(false)
const days = ref(7)
const platform = ref<StatsPlatform>('all')
const stats = ref<UsageStats | null>(null)
const heatmap = ref<HeatmapData[]>([])
const logDirectory = ref('')
const heatmapWeeks = 26 // ~6 months

const platforms = [
  { value: 'all' as StatsPlatform, label: '全部', icon: 'fas fa-layer-group' },
  { value: 'claude' as StatsPlatform, label: 'Claude', icon: 'fas fa-robot' },
  { value: 'gemini' as StatsPlatform, label: 'Gemini', icon: 'fas fa-gem' },
  { value: 'codex' as StatsPlatform, label: 'Codex', icon: 'fas fa-code' }
]

// Glider for tabs
const tabRefs = ref<HTMLElement[]>([])
const gliderStyle = ref({ left: '4px', width: '0px' })

function updateGlider() {
  nextTick(() => {
    const activeIndex = platforms.findIndex(p => p.value === platform.value)
    if (activeIndex !== -1 && tabRefs.value[activeIndex]) {
      const el = tabRefs.value[activeIndex]
      gliderStyle.value = {
        left: `${el.offsetLeft}px`,
        width: `${el.offsetWidth}px`
      }
    }
  })
}

const totalTokens = computed(() => {
  if (!stats.value) return 0
  return (stats.value.total_input_tokens || 0) + (stats.value.total_output_tokens || 0)
})

// Sort models by cost (descending), filter out synthetic entries
const sortedModelStats = computed(() => {
  if (!stats.value?.by_model) return {}
  const entries = Object.entries(stats.value.by_model) as [string, ModelStats][]
  // Filter out synthetic and empty model names
  const filtered = entries.filter(([model]) =>
    model && !model.includes('synthetic') && !model.startsWith('<')
  )
  filtered.sort((a, b) => b[1].cost - a[1].cost)
  return Object.fromEntries(filtered)
})

const maxModelCost = computed(() => {
  if (!stats.value?.by_model) return 1
  const costs = Object.values(stats.value.by_model).map(s => s.cost)
  return Math.max(...costs, 0.001)
})

// GitHub-style heatmap grid generation
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

  // Create lookup map
  heatmap.value.forEach(d => heatmapMap.set(d.date, d))

  // Calculate start date (beginning of week, heatmapWeeks weeks ago)
  const startDate = new Date(today)
  startDate.setDate(startDate.getDate() - (heatmapWeeks * 7) - today.getDay() + 1)

  // Generate grid week by week
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
        weekCells.push({
          date: '',
          requests: 0,
          tokens: 0,
          cost: 0,
          isToday: false
        })
      }
      currentDate.setDate(currentDate.getDate() + 1)
    }
    grid.push(weekCells)
  }

  return grid
})

// Month labels for heatmap
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
      // Calculate position as percentage
      const leftPercent = (week / heatmapWeeks) * 100
      labels.push({ name: monthNames[month], left: `${leftPercent}%` })
      lastMonth = month
    }
    currentDate.setDate(currentDate.getDate() + 7)
  }

  return labels
})

const chartData = computed(() => {
  const series = stats.value?.series || []
  return {
    labels: series.map(s => s.hour.slice(-5)),
    datasets: [
      {
        label: '花费 ($)',
        data: series.map(s => Number(s.cost.toFixed(4))),
        borderColor: '#f97316',
        backgroundColor: 'rgba(249, 115, 22, 0.1)',
        tension: 0.3,
        fill: false,
        yAxisID: 'yCost'
      },
      {
        label: '输入 Tokens',
        data: series.map(s => s.input_tokens),
        borderColor: '#34d399',
        backgroundColor: 'rgba(52, 211, 153, 0.2)',
        tension: 0.3,
        fill: true
      },
      {
        label: '输出 Tokens',
        data: series.map(s => s.output_tokens),
        borderColor: '#60a5fa',
        backgroundColor: 'rgba(96, 165, 250, 0.2)',
        tension: 0.3,
        fill: true
      }
    ]
  }
})

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    mode: 'index' as const,
    intersect: false
  },
  plugins: {
    legend: {
      labels: {
        color: 'var(--foreground)',
        font: { size: 11 }
      }
    }
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { color: 'var(--muted-foreground)', font: { size: 10 } }
    },
    y: {
      beginAtZero: true,
      grid: { color: 'var(--border)' },
      ticks: { color: 'var(--muted-foreground)', font: { size: 10 } }
    },
    yCost: {
      position: 'right' as const,
      beginAtZero: true,
      grid: { drawOnChartArea: false },
      ticks: {
        color: '#f97316',
        font: { size: 10 },
        callback: (value: number | string) => `$${Number(value).toFixed(3)}`
      }
    }
  }
}))

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
  // Extract friendly name from model ID
  // Claude models
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
  // GPT models
  if (model.includes('gpt-4o-mini')) return 'GPT-4o Mini'
  if (model.includes('gpt-4o')) return 'GPT-4o'
  if (model.includes('gpt-4-turbo')) return 'GPT-4 Turbo'
  if (model.includes('gpt-4')) return 'GPT-4'
  // Codex models
  if (model.includes('gpt-5.2-codex')) return 'GPT-5.2 Codex'
  if (model.includes('gpt-5.2')) return 'GPT-5.2'
  if (model.includes('gpt-5.1-codex-mini')) return 'GPT-5.1 Codex Mini'
  if (model.includes('gpt-5.1-codex-max')) return 'GPT-5.1 Codex Max'
  if (model.includes('gpt-5.1-codex')) return 'GPT-5.1 Codex'
  if (model.includes('gpt-5.1')) return 'GPT-5.1'
  if (model.includes('gpt-5-codex')) return 'GPT-5 Codex'
  if (model.includes('gpt-5')) return 'GPT-5'
  if (model.includes('codex-1')) return 'Codex-1'
  // Gemini models
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

async function setDays(d: number) {
  days.value = d
  await loadData()
}

async function setPlatform(p: StatsPlatform) {
  platform.value = p
  updateGlider()
  await loadData()
}

async function refresh() {
  await loadData()
}

async function loadData() {
  loading.value = true
  try {
    const [statsData, heatmapData] = await Promise.all([
      getUsageStats(days.value, platform.value),
      getHeatmapData(heatmapWeeks * 7, platform.value)
    ])
    stats.value = statsData
    heatmap.value = heatmapData

    try {
      logDirectory.value = await getLogDirectory()
    } catch {
      logDirectory.value = '需要重新编译后端'
    }
  } catch (e) {
    console.error('Failed to load stats:', e)
  } finally {
    loading.value = false
  }
}

watch(isOpen, (open) => {
  if (open) {
    loadData()
    // Update glider after modal opens
    setTimeout(updateGlider, 100)
  } else {
    // 关闭时重置平台
    platform.value = 'all'
  }
})
</script>

<style scoped>
.stats-content {
  position: relative;
  min-height: 200px;
}

.stats-summary {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}

@media (max-width: 640px) {
  .stats-summary {
    grid-template-columns: repeat(2, 1fr);
  }
}

.stat-card {
  background: var(--muted);
  border-radius: 12px;
  padding: 16px;
  text-align: center;
}

.stat-card.highlight {
  background: linear-gradient(135deg, rgba(249, 115, 22, 0.15), rgba(249, 115, 22, 0.05));
  border: 1px solid rgba(249, 115, 22, 0.3);
}

.stat-label {
  font-size: 12px;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.stat-value {
  font-size: 24px;
  font-weight: 700;
  margin: 4px 0;
}

.stat-hint {
  font-size: 11px;
  color: var(--muted-foreground);
}

.chart-container {
  height: 250px;
  margin-top: 16px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: var(--muted-foreground);
}

/* Model Breakdown Styles */
.model-breakdown {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.model-item {
  background: var(--muted);
  border-radius: 10px;
  padding: 12px 14px;
  position: relative;
  overflow: hidden;
}

.model-info {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 6px;
}

.model-name {
  font-weight: 600;
  font-size: 14px;
}

.model-full-name {
  font-family: ui-monospace, monospace;
  font-size: 10px;
  color: var(--muted-foreground);
  opacity: 0.7;
}

.model-stats {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: var(--muted-foreground);
}

.model-stat {
  display: flex;
  align-items: center;
  gap: 4px;
}

.model-stat i {
  font-size: 10px;
  opacity: 0.7;
}

.model-stat.cost {
  color: #f97316;
  font-weight: 600;
}

.model-bar {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: var(--border);
}

.model-bar-fill {
  height: 100%;
  background: linear-gradient(90deg, #f97316, #fb923c);
  border-radius: 0 2px 2px 0;
  transition: width 0.3s ease;
}

/* GitHub-style Heatmap */
.heatmap-months {
  display: flex;
  margin-bottom: 8px;
  margin-left: 24px;
}

.heatmap-month-row {
  display: flex;
  flex: 1;
  font-size: 9px;
  color: var(--muted-foreground);
  position: relative;
  height: 16px;
}

.month-label {
  white-space: nowrap;
  position: absolute;
  top: 0;
  transform: translateX(-50%);
}

.heatmap-container {
  display: flex;
  gap: 4px;
  width: 100%;
}

.heatmap-day-labels {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 9px;
  color: var(--muted-foreground);
  width: 20px;
  flex-shrink: 0;
  padding-top: 0;
}

.heatmap-day-labels span {
  aspect-ratio: 1;
  display: flex;
  align-items: center;
}

.heatmap-grid {
  display: flex;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.heatmap-week {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.heatmap-cell {
  aspect-ratio: 1;
  width: 100%;
  border-radius: 2px;
  cursor: pointer;
  transition: transform 0.1s, outline 0.1s;
  outline: 1px solid transparent;
}

.heatmap-cell:not(.empty):hover {
  transform: scale(1.3);
  outline: 2px solid var(--primary);
  z-index: 10;
  position: relative;
}

.heatmap-cell.empty {
  cursor: default;
}

.heatmap-cell.today {
  outline: 2px solid var(--primary);
}

.heatmap-legend {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 10px;
  justify-content: flex-end;
}

.legend-scale {
  display: flex;
  gap: 2px;
}

.legend-item {
  width: 11px;
  height: 11px;
  border-radius: 2px;
}

/* Loading Overlay */
.loading-overlay {
  position: absolute;
  inset: 0;
  background: var(--background);
  background: color-mix(in srgb, var(--background) 92%, transparent);
  backdrop-filter: blur(2px);
  z-index: 999;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
}

.loading-spinner {
  display: flex;
  flex-direction: column;
  align-items: center;
}

/* Dark mode glider fix */
:global(.dark) .bg-white {
  background: var(--background) !important;
}
</style>
