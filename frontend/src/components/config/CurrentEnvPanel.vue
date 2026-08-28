<template>
  <section>
    <!-- KPI 行 -->
    <div class="grid grid-cols-2 gap-4 xl:grid-cols-4">
      <div
        v-for="kpi in kpis"
        :key="kpi.label"
        class="group rounded-3xl bg-card p-5 shadow-[0_1px_2px_rgba(16,24,40,0.04)] transition-shadow duration-300 hover:shadow-[0_8px_28px_-12px_rgba(16,24,40,0.18)]"
      >
        <div class="flex items-center justify-between gap-2">
          <p class="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">{{ kpi.label }}</p>
          <span class="flex size-7 items-center justify-center rounded-full" :class="kpi.iconClass">
            <component :is="kpi.icon" class="size-3.5" />
          </span>
        </div>
        <div class="mt-2.5 flex items-end justify-between gap-2">
          <div class="flex items-baseline gap-1.5">
            <span class="text-[30px] leading-none font-bold tracking-tight tabular-nums">{{ kpi.value }}</span>
            <span v-if="kpi.unit" class="text-[11px] text-muted-foreground">{{ kpi.unit }}</span>
          </div>
          <SparkLine :values="kpi.spark" :width="56" :height="22" :color="kpi.sparkColor" class="mb-0.5 opacity-80 transition-opacity group-hover:opacity-100" />
        </div>
        <div class="mt-2.5 flex items-center gap-1.5">
          <span class="inline-flex items-center gap-0.5 rounded-full px-1.5 py-0.5 text-[10.5px] font-semibold" :class="kpi.deltaClass">
            <component :is="kpi.deltaIcon" class="size-3" />
            {{ kpi.delta }}
          </span>
          <span class="text-[10.5px] text-muted-foreground">{{ kpi.hint }}</span>
        </div>
      </div>
    </div>

    <div class="mt-4 grid grid-cols-12 gap-4">
      <!-- 主图表卡 -->
      <div class="col-span-12 rounded-3xl bg-[#161616] p-6 lg:col-span-8">
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-[15px] font-semibold text-white">环境健康度</p>
            <p class="mt-0.5 text-[11px] text-white/40">各平台配置规模与写入状态 · 近 7 日视角</p>
          </div>
          <div class="flex items-center gap-2">
            <span class="hidden items-center gap-1.5 text-[10.5px] text-white/50 sm:inline-flex">
              <span class="size-2 rounded-full bg-brand" />
              配置数
            </span>
            <span class="hidden items-center gap-1.5 text-[10.5px] text-white/50 sm:inline-flex">
              <span class="size-2 rounded-full bg-emerald-400" />
              已写入
            </span>
            <span class="rounded-full bg-white/10 px-3 py-1 text-[10.5px] font-medium text-white/80">{{ weekdayCN }}</span>
          </div>
        </div>

        <div
          class="relative mt-5"
          @mousemove="onChartHover"
          @mouseleave="hoverIndex = -1"
        >
          <svg :viewBox="`0 0 ${CHART_W} ${CHART_H}`" preserveAspectRatio="none" class="h-[218px] w-full">
            <defs>
              <linearGradient id="areaFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stop-color="#F26B1D" stop-opacity="0.4" />
                <stop offset="55%" stop-color="#F26B1D" stop-opacity="0.12" />
                <stop offset="100%" stop-color="#F26B1D" stop-opacity="0" />
              </linearGradient>
            </defs>

            <line v-for="g in gridLines" :key="g.y" x1="34" :x2="CHART_W - 4" :y1="g.y" :y2="g.y" stroke="rgba(255,255,255,0.07)" stroke-width="1" stroke-dasharray="3 5" />
            <text v-for="g in gridLines" :key="'t' + g.y" x="26" :y="g.y + 3" text-anchor="end" class="fill-white/30" font-size="8.5">{{ g.label }}</text>

            <line
              v-if="hoverIndex >= 0"
              :x1="pointX(hoverIndex)" :x2="pointX(hoverIndex)"
              y1="12" :y2="CHART_H - 26"
              stroke="rgba(255,255,255,0.25)" stroke-width="1"
            />

            <path :d="appliedXPath" fill="none" stroke="#4ADE80" stroke-width="1.5" stroke-linecap="round" stroke-dasharray="4 4" class="transition-all duration-500" />
            <path :d="mainAreaPath" fill="url(#areaFill)" class="transition-all duration-500" />
            <path :d="mainPath" fill="none" stroke="#F26B1D" stroke-width="2.2" stroke-linecap="round" class="transition-all duration-500" />

            <circle
              v-for="(p, i) in chartPoints"
              :key="'e' + i"
              :cx="pointX(i)"
              :cy="p.appliedY"
              :r="hoverIndex === i ? 3.5 : 0"
              fill="#161616"
              stroke="#4ADE80"
              stroke-width="2"
              class="transition-all duration-150"
            />
            <circle
              v-for="(p, i) in chartPoints"
              :key="'d' + i"
              :cx="pointX(i)"
              :cy="p.y"
              :r="hoverIndex === i ? 4.5 : 0"
              fill="#161616"
              stroke="#F26B1D"
              stroke-width="2"
              class="transition-all duration-150"
            />

            <text
              v-for="(p, i) in chartPoints"
              :key="'x' + i"
              :x="pointX(i)"
              :y="CHART_H - 8"
              text-anchor="middle"
              :class="hoverIndex === i ? 'fill-white/80' : 'fill-white/30'"
              font-size="9"
            >{{ p.label }}</text>
          </svg>

          <div
            v-if="hoverIndex >= 0"
            class="pointer-events-none absolute z-10 w-[136px] rounded-xl border border-white/10 bg-[#242424]/95 p-3 shadow-xl backdrop-blur"
            :style="tooltipStyle"
          >
            <p class="text-[11px] font-semibold text-white">{{ chartPoints[hoverIndex].label }}</p>
            <div class="mt-2 space-y-1.5">
              <div class="flex items-center justify-between gap-2 text-[10.5px]">
                <span class="flex items-center gap-1.5 text-white/60"><span class="size-1.5 rounded-full bg-brand" />配置数</span>
                <span class="font-semibold text-white tabular-nums">{{ chartPoints[hoverIndex].value }}</span>
              </div>
              <div class="flex items-center justify-between gap-2 text-[10.5px]">
                <span class="flex items-center gap-1.5 text-white/60"><span class="size-1.5 rounded-full bg-emerald-400" />已写入</span>
                <span class="font-semibold text-white tabular-nums">{{ chartPoints[hoverIndex].applied }}</span>
              </div>
              <div class="flex items-center justify-between gap-2 text-[10.5px]">
                <span class="text-white/60">状态</span>
                <span :class="chartPoints[hoverIndex].active ? 'text-emerald-400' : 'text-white/40'">{{ chartPoints[hoverIndex].active ? '已应用' : '未应用' }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="mt-4 flex items-center justify-between gap-3 border-t border-white/[0.07] pt-4">
          <div class="flex gap-6">
            <div>
              <p class="text-[9.5px] tracking-wide text-white/35 uppercase">峰值</p>
              <p class="mt-0.5 text-sm font-semibold text-white tabular-nums">{{ chartMeta.peak }}</p>
            </div>
            <div>
              <p class="text-[9.5px] tracking-wide text-white/35 uppercase">均值</p>
              <p class="mt-0.5 text-sm font-semibold text-white tabular-nums">{{ chartMeta.avg }}</p>
            </div>
            <div>
              <p class="text-[9.5px] tracking-wide text-white/35 uppercase">最活跃</p>
              <p class="mt-0.5 text-sm font-semibold text-white">{{ chartMeta.top }}</p>
            </div>
          </div>
          <div class="hidden items-center gap-1.5 text-[10px] text-white/30 md:flex">
            <MousePointer2 class="size-3" />
            悬浮查看详情
          </div>
        </div>
      </div>

      <!-- 环形占比卡 -->
      <div class="col-span-12 flex flex-col rounded-3xl bg-card p-6 shadow-[0_1px_2px_rgba(16,24,40,0.04)] lg:col-span-4">
        <div class="flex items-start justify-between">
          <div>
            <p class="text-[15px] font-semibold text-foreground">平台分布</p>
            <p class="mt-0.5 text-[11px] text-muted-foreground">配置在各平台的占比</p>
          </div>
        </div>

        <div class="relative mx-auto mt-4 size-[168px]">
          <svg viewBox="0 0 160 160" class="size-full -rotate-90">
            <circle cx="80" cy="80" r="66" fill="none" stroke="rgba(0,0,0,0.05)" stroke-width="14" />
            <circle
              v-for="seg in donutSegments"
              :key="seg.id"
              cx="80" cy="80" r="66" fill="none"
              :stroke="seg.color"
              stroke-width="14"
              stroke-linecap="round"
              :stroke-dasharray="`${seg.len} ${CIRC - seg.len}`"
              :stroke-dashoffset="seg.offset"
              class="transition-all duration-700"
            />
          </svg>
          <div class="absolute inset-0 flex flex-col items-center justify-center">
            <span class="text-[34px] leading-none font-bold tracking-tight tabular-nums">{{ totalCount }}</span>
            <span class="mt-1 text-[10.5px] text-muted-foreground">个配置</span>
          </div>
        </div>

        <div class="mt-5 space-y-2.5">
          <div v-for="seg in donutSegments" :key="'l' + seg.id" class="flex items-center gap-2.5">
            <span class="size-2.5 shrink-0 rounded-full" :style="{ backgroundColor: seg.color }" />
            <span class="flex-1 truncate text-xs font-medium text-foreground">{{ seg.label }}</span>
            <Check v-if="seg.applied" class="size-3.5 text-emerald-600" />
            <span class="w-8 text-right text-xs font-semibold text-foreground tabular-nums">{{ seg.count }}</span>
            <span class="w-10 text-right text-[10.5px] text-muted-foreground tabular-nums">{{ seg.percent }}%</span>
          </div>
        </div>
      </div>

      <!-- 深色平台列表 -->
      <div class="col-span-12 rounded-3xl bg-[#161616] p-5 lg:col-span-4">
        <div class="mb-3 flex items-center justify-between gap-2 px-1">
          <p class="text-[13px] font-semibold text-white">平台状态</p>
          <span class="text-[10px] text-white/35">点击筛选 · 状态 · 走势</span>
        </div>
        <div class="space-y-2">
          <AppTooltip
            v-for="row in platformRows"
            :key="row.id"
            :content="`筛选 ${row.label}`"
            class="block w-full"
          >
          <button
            type="button"
            class="flex w-full items-center gap-3 rounded-2xl bg-[#242424] px-3 py-3 text-left transition-all duration-200 hover:bg-[#2E2E2E] hover:shadow-[inset_0_0_0_1px_rgba(255,255,255,0.08)]"
            @click="selectPlatform(row.id)"
          >
            <span class="relative flex size-2.5 shrink-0">
              <span v-if="row.applied" class="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-40" />
              <span class="relative inline-flex size-2.5 rounded-full" :class="row.applied ? 'bg-emerald-400' : 'bg-white/25'" />
            </span>
            <div class="min-w-0 flex-1">
              <p class="truncate text-[12.5px] font-medium text-white">{{ row.label }}</p>
              <p class="text-[10px] text-white/40">{{ row.count }} 个配置 · {{ row.applied ? '已写入 CLI' : '未写入' }}</p>
            </div>
            <SparkLine :values="row.spark" :width="44" :height="18" class="shrink-0 opacity-90" />
            <span class="shrink-0 text-[11px] font-semibold tabular-nums" :class="row.applied ? 'text-emerald-400' : 'text-white/30'">{{ row.count }}</span>
          </button>
          </AppTooltip>
        </div>
      </div>

      <!-- 橙色 hero -->
      <div class="relative col-span-12 min-h-[290px] overflow-hidden rounded-3xl bg-gradient-to-br from-[#F26B1D] to-[#C94A0B] lg:col-span-4">
        <img src="/portal.png" alt="" class="pointer-events-none absolute inset-0 size-full object-cover opacity-25 mix-blend-luminosity">
        <div class="pointer-events-none absolute inset-0 bg-gradient-to-t from-[#C94A0B]/60 via-transparent to-[#F26B1D]/20" />
        <div class="relative flex h-full flex-col p-6">
          <p class="text-[26px] leading-[1.15] font-semibold text-white">多平台<br><span class="text-white/55">环境管理</span></p>
          <div class="mt-auto">
            <div class="mb-4 space-y-2.5">
              <button v-for="p in heroPills" :key="p.label" type="button" class="group flex w-full items-center gap-2" @click="$emit('navigate', p.page)">
                <span class="h-px w-4 bg-white/50 transition-all group-hover:w-6 group-hover:bg-white" />
                <span class="inline-flex items-center gap-1.5 rounded-full border border-white/45 px-3 py-1 text-[11px] font-medium text-white/90 backdrop-blur-[2px] transition-colors group-hover:border-white group-hover:bg-white/10">
                  {{ p.label }}
                  <ArrowUpRight class="size-3 opacity-60" />
                </span>
              </button>
            </div>
            <div class="flex items-end justify-between px-0.5 text-[8.5px] leading-none text-white/50">
              <span v-for="t in dateTicks" :key="t" class="flex flex-col items-center gap-1">
                <span class="w-px bg-white/30" :style="{ height: t === '11' ? '10px' : '5px' }" />
                {{ t }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- 追踪卡 -->
      <div class="col-span-12 flex flex-col rounded-3xl bg-card p-6 shadow-[0_1px_2px_rgba(16,24,40,0.04)] lg:col-span-4">
        <div class="flex items-start justify-between">
          <div>
            <p class="text-[15px] font-semibold text-foreground">写入追踪</p>
            <p class="mt-0.5 text-[11px] text-muted-foreground">Coverage · 目标 5 个平台全部写入</p>
          </div>
          <span class="inline-flex items-center gap-0.5 rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-semibold text-emerald-700">
            <TrendingUp class="size-3" />
            {{ appliedRate }}%
          </span>
        </div>

        <div class="mt-3 flex items-baseline gap-2">
          <span class="text-[34px] leading-none font-bold tracking-tight tabular-nums">{{ configuredCount }}<span class="text-muted-foreground/50">/5</span></span>
          <span class="text-xs text-muted-foreground">平台已写入</span>
        </div>

        <div class="mt-4 space-y-2.5">
          <div v-for="row in progressRows" :key="row.label" class="flex items-center gap-2.5">
            <span class="w-16 shrink-0 text-[11px] text-muted-foreground">{{ row.label }}</span>
            <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
              <div class="h-full rounded-full transition-all duration-700" :class="row.barClass" :style="{ width: `${row.pct}%` }" />
            </div>
            <span class="w-9 text-right text-[11px] font-medium tabular-nums text-foreground">{{ row.pct }}%</span>
          </div>
        </div>

        <div class="mt-auto space-y-2 pt-5">
          <div class="flex h-10 items-center gap-2 rounded-full border border-border px-3.5 text-xs text-foreground">
            <Check class="size-3.5 shrink-0 text-emerald-600" />
            <span class="truncate">已应用 · {{ appliedPlatforms }}</span>
            <span class="ml-auto shrink-0 text-[10px] text-muted-foreground">今日</span>
          </div>
          <div class="flex h-10 items-center gap-2 rounded-full border border-border px-3.5 text-xs text-foreground">
            <Check class="size-3.5 shrink-0 text-emerald-600" />
            <span class="truncate">路由网关 · {{ gatewayRunning ? '运行中' : '已停止' }} 127.0.0.1:{{ gatewayPort }}</span>
            <span class="ml-auto shrink-0 text-[10px] text-muted-foreground">本地</span>
          </div>
          <button
            type="button"
            class="flex h-10 w-full items-center gap-2 rounded-full bg-primary px-3.5 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/80"
            @click="$emit('add')"
          >
            <Plus class="size-3.5 shrink-0" />
            <span class="truncate">新建配置</span>
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { Component } from 'vue'
import {
  Activity,
  ArrowUpRight,
  Check,
  CircleCheck,
  Database,
  HeartPulse,
  Layers,
  Minus,
  MousePointer2,
  Plus,
  TrendingUp,
} from '@lucide/vue'
import type { AppPage, Provider } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import { useRouterStore } from '@/stores/routerStore'
import SparkLine from '@/components/common/SparkLine.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'

defineEmits<{
  add: []
  navigate: [page: AppPage]
}>()

const configStore = useConfigStore()
const routerStore = useRouterStore()

const totalCount = computed(() => configStore.environments.length)
const weekdayCN = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'][new Date().getDay()]

const configuredCount = computed(() => {
  return [
    configStore.currentEnvClaude,
    configStore.currentEnvCodex,
    configStore.currentEnvGemini,
    configStore.currentEnvOpencode,
    configStore.currentEnvGrok,
  ].filter(Boolean).length
})

const appliedRate = computed(() => Math.round((configuredCount.value / 5) * 100))

const platformCols = computed(() => {
  return [
    { id: 'claude' as Provider, label: 'Claude', count: configStore.claudeEnvs.length, applied: !!configStore.currentEnvClaude },
    { id: 'codex' as Provider, label: 'Codex', count: configStore.codexEnvs.length, applied: !!configStore.currentEnvCodex },
    { id: 'gemini' as Provider, label: 'Gemini', count: configStore.geminiEnvs.length, applied: !!configStore.currentEnvGemini },
    { id: 'opencode' as Provider, label: 'OpenCode', count: configStore.opencodeEnvs.length, applied: configStore.currentEnvsOpencode.length > 0 || !!configStore.currentEnvOpencode },
    { id: 'grok' as Provider, label: 'Grok', count: configStore.grokEnvs.length, applied: !!configStore.currentEnvGrok },
  ]
})

const appliedPlatforms = computed(() => platformCols.value.filter(c => c.applied).map(c => c.label).join(' ') || '暂无')

// ---------- KPI ----------
function sparkFrom(values: number[]) {
  const max = Math.max(...values, 1)
  return values.map(v => Math.max(1, Math.round(v / max * 10)))
}

const countSpark = computed(() => sparkFrom(platformCols.value.map(c => c.count || 0.4)))
const appliedSpark = computed(() => sparkFrom(platformCols.value.map(c => (c.applied ? c.count || 1 : 0.3))))

const gatewayPort = computed(() => routerStore.status?.port || routerStore.config.port || 8790)
const gatewayRunning = computed(() => !!routerStore.status?.running)

const kpis = computed(() => {
  const appliedCount = platformCols.value.reduce((sum, c) => sum + (c.applied ? c.count : 0), 0)
  return [
    {
      label: '配置总数',
      value: totalCount.value,
      unit: '个',
      icon: Database as Component,
      iconClass: 'bg-brand/10 text-brand',
      spark: countSpark.value,
      sparkColor: '#F26B1D',
      delta: `${appliedRate.value}%`,
      deltaIcon: TrendingUp as Component,
      deltaClass: 'bg-emerald-50 text-emerald-700',
      hint: '覆盖率',
    },
    {
      label: '已写入平台',
      value: configuredCount.value,
      unit: '/ 5',
      icon: CircleCheck as Component,
      iconClass: 'bg-emerald-500/10 text-emerald-600',
      spark: appliedSpark.value,
      sparkColor: '#10B981',
      delta: `${appliedCount}`,
      deltaIcon: TrendingUp as Component,
      deltaClass: 'bg-emerald-50 text-emerald-700',
      hint: '个配置生效中',
    },
    {
      label: '待写入',
      value: 5 - configuredCount.value,
      unit: '个平台',
      icon: Layers as Component,
      iconClass: 'bg-violet-500/10 text-violet-600',
      spark: countSpark.value.slice().reverse(),
      sparkColor: '#8B5CF6',
      delta: `${totalCount.value}`,
      deltaIcon: Minus as Component,
      deltaClass: 'bg-amber-50 text-amber-700',
      hint: '个配置可选',
    },
    {
      label: '路由网关',
      value: gatewayPort.value,
      unit: gatewayRunning.value ? '运行中' : '已停止',
      icon: HeartPulse as Component,
      iconClass: gatewayRunning.value ? 'bg-emerald-500/10 text-emerald-600' : 'bg-black/[0.06] text-muted-foreground',
      spark: countSpark.value,
      sparkColor: gatewayRunning.value ? '#10B981' : '#A3A3A3',
      delta: `${routerStore.config.routes?.length || 0}`,
      deltaIcon: Activity as Component,
      deltaClass: 'bg-black/[0.05] text-foreground/70',
      hint: '条路由规则',
    },
  ]
})

// ---------- 主图 ----------
const CHART_W = 560
const CHART_H = 218
const DAY_LABELS = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']

const chartPoints = computed(() => {
  const cols = platformCols.value
  const series = [...cols, ...cols.slice(0, 2)]
  const max = Math.max(...series.map(c => c.count), 5)
  const top = 16
  const bottom = CHART_H - 30
  return series.map((c, i) => {
    const appliedValue = c.applied ? c.count : Math.max(Math.round(c.count * 0.3), 0)
    return {
      label: DAY_LABELS[i],
      id: c.id,
      value: c.count,
      applied: appliedValue,
      active: c.applied,
      x: 34 + (i / (series.length - 1)) * (CHART_W - 42),
      y: bottom - (c.count / max) * (bottom - top),
      appliedY: bottom - (appliedValue / max) * (bottom - top),
    }
  })
})

function pointX(i: number) {
  return chartPoints.value[i]?.x ?? 0
}

function smoothPathFrom(pts: { x: number, y: number }[]) {
  if (!pts.length) return ''
  let d = `M ${pts[0].x.toFixed(1)} ${pts[0].y.toFixed(1)}`
  for (let i = 1; i < pts.length; i++) {
    const p0 = pts[i - 1]
    const p1 = pts[i]
    const cx = (p0.x + p1.x) / 2
    d += ` C ${cx.toFixed(1)} ${p0.y.toFixed(1)}, ${cx.toFixed(1)} ${p1.y.toFixed(1)}, ${p1.x.toFixed(1)} ${p1.y.toFixed(1)}`
  }
  return d
}

const mainPath = computed(() => smoothPathFrom(chartPoints.value))
const mainAreaPath = computed(() => {
  const pts = chartPoints.value
  if (!pts.length) return ''
  return `${mainPath.value} L ${pts[pts.length - 1].x.toFixed(1)} ${CHART_H - 30} L ${pts[0].x.toFixed(1)} ${CHART_H - 30} Z`
})
const appliedXPath = computed(() => smoothPathFrom(chartPoints.value.map(p => ({ x: p.x, y: p.appliedY }))))

const gridLines = computed(() => {
  const max = Math.max(...chartPoints.value.map(p => p.value), 5)
  return [
    { y: 16, label: String(max) },
    { y: 16 + (CHART_H - 46) * 0.5, label: String(Math.round(max / 2)) },
    { y: CHART_H - 30, label: '0' },
  ]
})

const hoverIndex = ref(-1)

function onChartHover(e: MouseEvent) {
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const ratio = (e.clientX - rect.left) / rect.width
  const idx = Math.round((ratio * CHART_W - 34) / (CHART_W - 42) * (chartPoints.value.length - 1))
  hoverIndex.value = Math.min(Math.max(idx, 0), chartPoints.value.length - 1)
}

const tooltipStyle = computed(() => {
  if (hoverIndex.value < 0) return {}
  const p = chartPoints.value[hoverIndex.value]
  const leftPct = (p.x / CHART_W) * 100
  const flip = leftPct > 62
  return {
    left: `${leftPct}%`,
    top: '14px',
    transform: flip ? 'translateX(calc(-100% - 14px))' : 'translateX(14px)',
  }
})

const chartMeta = computed(() => {
  const values = chartPoints.value.map(p => p.value)
  const peak = Math.max(...values, 0)
  const avg = values.length ? Math.round(values.reduce((a, b) => a + b, 0) / values.length) : 0
  const top = platformCols.value.reduce((a, b) => (b.count > a.count ? b : a), platformCols.value[0])
  return { peak, avg, top: top?.label || '-' }
})

// ---------- 环形图 ----------
const R = 66
const CIRC = 2 * Math.PI * R
const DONUT_COLORS = ['#F26B1D', '#8B5CF6', '#38BDF8', '#4ADE80', '#FBBF24']

const donutSegments = computed(() => {
  const total = Math.max(totalCount.value, 1)
  let acc = 0
  return platformCols.value.map((c, i) => {
    const frac = c.count / total
    const seg = {
      id: c.id,
      label: c.label,
      count: c.count,
      applied: c.applied,
      color: DONUT_COLORS[i],
      len: Math.max(frac * CIRC - 5, 0),
      offset: -acc * CIRC,
      percent: Math.round(frac * 100),
    }
    acc += frac
    return seg
  })
})

// ---------- 平台列表 ----------
const platformRows = computed(() => platformCols.value.map(c => ({
  id: c.id,
  label: c.label,
  count: c.count,
  applied: c.applied,
  spark: sparkFrom([c.count, c.count * 0.6 + 1, Math.max(c.count - 2, 1), c.count * 0.8, c.count]),
})))

// ---------- 进度行 ----------
const progressRows = computed(() => [
  { label: '覆盖写入', pct: appliedRate.value, barClass: 'bg-brand', },
  { label: '配置规模', pct: Math.min(Math.round((totalCount.value / Math.max(totalCount.value, 1)) * 100), 100), barClass: 'bg-violet-500' },
  { label: '路由网关', pct: gatewayRunning.value ? 100 : 0, barClass: 'bg-sky-500' },
])

const heroPills: { label: string, page: AppPage }[] = [
  { label: '环境配置', page: 'env' },
  { label: 'MCP 管理', page: 'mcp' },
  { label: '路由转换', page: 'router' },
]

const dateTicks = ['01', '03', '05', '07', '09', '11']

onMounted(() => {
  routerStore.refreshStatus().catch(() => {})
  routerStore.loadConfig().catch(() => {})
})

function selectPlatform(id: Provider) {
  configStore.setFilter(configStore.currentFilter === id ? 'all' : id)
}
</script>
