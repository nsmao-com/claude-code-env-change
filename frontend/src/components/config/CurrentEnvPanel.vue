<template>
  <section class="px-8 pt-3">
    <PageHeader title="环境">
      <template #title-extra>
        <Button variant="ghost" size="icon-xs" class="rounded-full ring-1 ring-black/[0.06] dark:ring-white/10" title="刷新" @click="refresh">
          <RefreshCw :class="['size-3.5', isLoading && 'animate-spin']" />
        </Button>
      </template>
      <template #actions>
        <ToolFilterChips />
        <div class="inline-flex h-9 items-center gap-2 rounded-full bg-card px-3 text-sm ring-1 ring-black/[0.06] dark:ring-white/10">
          <span class="size-2 rounded-full" :class="configuredCount ? 'bg-emerald-500' : 'bg-muted-foreground/40'" />
          已应用 {{ configuredCount }}/5
        </div>
        <Button class="h-9 rounded-full px-3.5" @click="$emit('add')">
          <Plus class="size-3.5" />
          新建
        </Button>
      </template>
    </PageHeader>

    <div class="mt-4 grid grid-cols-12 gap-3">
      <Card class="col-span-12 py-4 lg:col-span-7">
        <CardHeader class="px-5">
          <CardTitle class="text-sm font-medium text-muted-foreground">平台配置</CardTitle>
        </CardHeader>
        <CardContent class="px-5">
          <div class="flex h-32">
            <div class="flex w-8 shrink-0 flex-col justify-between pb-1 pr-1.5 text-right text-[10px] leading-none text-muted-foreground">
              <span>{{ axisMax }}</span>
              <span>{{ axisMid }}</span>
              <span>0</span>
            </div>
            <div class="flex min-w-0 flex-1 items-stretch gap-1.5">
              <button
                v-for="col in platformCols"
                :key="col.id"
                type="button"
                class="relative flex min-w-0 flex-1 flex-col rounded-xl px-1 pt-1 transition-colors"
                :class="col.highlighted ? 'bg-blue-50 dark:bg-blue-500/10' : 'hover:bg-muted/60'"
                @click="selectPlatform(col.id)"
              >
                <div class="px-0.5">
                  <div class="truncate text-[11px] text-muted-foreground">{{ col.label }}</div>
                  <div class="text-base font-semibold tracking-tight">{{ col.count }}</div>
                </div>
                <div class="relative mt-2 min-h-0 flex-1">
                  <motion.div
                    class="absolute inset-x-1.5 bottom-0 rounded-t-md"
                    :class="col.highlighted
                      ? 'bg-[#2F6BFF]'
                      : 'bg-[repeating-linear-gradient(135deg,#93c5fd_0px,#93c5fd_5px,#60a5fa_5px,#60a5fa_10px)] dark:bg-[repeating-linear-gradient(135deg,#1e3a8a_0px,#1e3a8a_5px,#2563eb_5px,#2563eb_10px)]'"
                    :initial="{ height: 0 }"
                    :animate="{ height: col.height + '%' }"
                    :transition="{ duration: 0.45, ease: [0.22, 1, 0.36, 1] }"
                  >
                    <span v-if="col.count !== 0" class="absolute inset-x-0 -top-px h-0.5 rounded-full bg-blue-800/40" />
                  </motion.div>
                </div>
              </button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card class="col-span-12 py-4 lg:col-span-5">
        <CardHeader class="px-5">
          <CardTitle class="text-sm font-medium text-muted-foreground">配置总量</CardTitle>
          <div class="mt-2 flex items-end gap-3">
            <span class="text-[2.5rem] leading-none font-semibold tracking-tight">{{ totalCount }}</span>
            <span class="mb-1 inline-flex items-center gap-1 text-xs font-medium text-emerald-600">
              <TrendingUp class="size-3.5" />
              {{ appliedRate }}%
            </span>
          </div>
        </CardHeader>
        <CardContent class="space-y-3 px-5 pt-3">
          <div v-for="row in volumeRows" :key="row.id">
            <div class="mb-1.5 flex items-center justify-between gap-3 text-sm">
              <span>{{ row.label }}</span>
              <span class="tabular-nums text-muted-foreground">{{ row.count }}</span>
            </div>
            <Progress :model-value="row.bar" class="h-1.5" :color="row.color" />
          </div>
        </CardContent>
      </Card>

      <Card class="col-span-12 py-4 sm:col-span-6 lg:col-span-3">
        <CardHeader class="px-5">
          <div class="flex items-start justify-between">
            <CardTitle class="text-sm font-medium text-muted-foreground">覆盖率</CardTitle>
            <span class="text-xs font-medium text-muted-foreground">{{ appliedRate }}%</span>
          </div>
        </CardHeader>
        <CardContent class="px-5">
          <svg viewBox="0 0 240 90" class="mt-1 h-16 w-full overflow-visible">
            <path :d="retentionArea" fill="#fda4af" fill-opacity="0.18"></path>
            <path :d="retentionLine" fill="none" stroke="#f43f5e" stroke-width="2.2" stroke-linejoin="miter"></path>
          </svg>
          <div class="mt-1 flex justify-between text-[11px] text-muted-foreground">
            <span v-for="col in platformCols" :key="col.id">{{ col.short }}</span>
          </div>
        </CardContent>
      </Card>

      <div class="col-span-12 flex flex-col gap-3 sm:col-span-6 lg:col-span-3">
        <Card class="py-3.5" size="sm">
          <CardHeader class="px-5">
            <div class="flex items-start justify-between">
              <CardTitle class="text-sm font-medium text-muted-foreground">配置</CardTitle>
              <span class="text-[11px] text-muted-foreground">{{ toolCountLabel }}</span>
            </div>
            <div class="mt-1 flex items-end justify-between gap-3">
              <span class="text-3xl font-semibold tracking-tight">{{ totalCount }}</span>
              <div class="mb-0.5 flex h-8 items-end gap-1">
                <span
                  v-for="(h, i) in countSpark"
                  :key="i"
                  class="w-2 rounded-full bg-emerald-400"
                  :style="{ height: `${h}%` }"
                />
              </div>
            </div>
          </CardHeader>
        </Card>
        <Card class="py-3.5" size="sm">
          <CardHeader class="px-5">
            <div class="flex items-start justify-between">
              <CardTitle class="text-sm font-medium text-muted-foreground">已应用</CardTitle>
              <span class="text-[11px] text-emerald-600">+{{ configuredCount }}</span>
            </div>
            <div class="mt-1 flex items-end justify-between gap-3">
              <span class="text-3xl font-semibold tracking-tight">{{ configuredCount }}</span>
              <div class="mb-0.5 flex h-8 items-end gap-1">
                <span
                  v-for="(h, i) in appliedSpark"
                  :key="i"
                  class="w-2 rounded-full bg-blue-500"
                  :style="{ height: `${h}%` }"
                />
              </div>
            </div>
          </CardHeader>
        </Card>
      </div>

      <Card class="col-span-12 overflow-hidden py-0 lg:col-span-6">
        <div class="relative flex h-full min-h-[148px] flex-col justify-end overflow-hidden bg-[linear-gradient(135deg,#f3ddc8_0%,#e7cbb8_22%,#c5d5ce_58%,#5aa7c2_100%)] px-6 py-5 dark:bg-[linear-gradient(135deg,#3a2f2a_0%,#2c3a3a_55%,#1f4a58_100%)]">
          <div class="pointer-events-none absolute inset-y-0 right-0 w-[42%] overflow-hidden">
            <div class="absolute -top-10 -right-6 size-36 rounded-[2rem] bg-white/25" />
            <div class="absolute top-5 right-6 size-24 rotate-[18deg] rounded-3xl bg-sky-300/40" />
            <div class="absolute right-2 bottom-2 size-14 rounded-full bg-orange-200/40" />
          </div>
          <div class="relative inline-flex items-center gap-1.5 rounded-full bg-white/40 px-2 py-0.5 text-[11px] font-medium text-foreground/70">
            <Sparkles class="size-3" />
            Insights
          </div>
          <div class="relative mt-2 text-4xl font-semibold tracking-tight">{{ appliedRate }}%</div>
          <p class="relative mt-2 max-w-sm text-sm leading-relaxed text-foreground/80">
            {{ insightCopy }}
          </p>
        </div>
      </Card>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { motion } from 'motion-v'
import { Plus, RefreshCw, Sparkles, TrendingUp } from '@lucide/vue'
import type { Provider } from '@/types'
import { useConfigStore } from '@/stores/configStore'
import { toolLabel } from '@/lib/workspace'
import PageHeader from '@/components/layout/PageHeader.vue'
import ToolFilterChips from '@/components/layout/ToolFilterChips.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'

defineEmits<{
  add: []
}>()

const configStore = useConfigStore()
const isLoading = computed(() => configStore.isLoading)
const totalCount = computed(() => configStore.environments.length)

const configuredCount = computed(() => {
  return [
    configStore.currentEnvClaude,
    configStore.currentEnvCodex,
    configStore.currentEnvGemini,
    configStore.currentEnvOpenclaw,
    configStore.currentEnvGrok,
  ].filter(Boolean).length
})

const appliedRate = computed(() => Math.round((configuredCount.value / 5) * 100))

const platformCols = computed(() => {
  const rows = [
    { id: 'claude' as Provider, label: 'Claude', short: 'Claude', count: configStore.claudeEnvs.length },
    { id: 'codex' as Provider, label: 'Codex', short: 'Codex', count: configStore.codexEnvs.length },
    { id: 'gemini' as Provider, label: 'Gemini', short: 'Gemini', count: configStore.geminiEnvs.length },
    { id: 'openclaw' as Provider, label: 'OpenClaw', short: 'Claw', count: configStore.openclawEnvs.length },
    { id: 'grok' as Provider, label: 'Grok', short: 'Grok', count: configStore.grokEnvs.length },
  ]
  const max = Math.max(...rows.map(row => row.count), 1)
  const filter = configStore.currentFilter
  return rows.map(row => ({
    ...row,
    height: row.count <= 0 ? 4 : Math.max(14, Math.round((row.count / max) * 100)),
    highlighted: filter === 'all' ? row.count === max && row.count > 0 : filter === row.id,
  }))
})

const axisMax = computed(() => Math.max(...platformCols.value.map(col => col.count), 1))
const axisMid = computed(() => Math.round(axisMax.value / 2))

const volumeRows = computed(() => {
  const max = Math.max(...platformCols.value.map(col => col.count), 1)
  const colors = ['#22c55e', '#3b82f6', '#f472b6', '#94a3b8', '#f59e0b']
  return platformCols.value.map((col, i) => ({
    id: col.id,
    label: col.label,
    count: col.count,
    bar: Math.round((col.count / max) * 100),
    color: colors[i],
  }))
})

const retentionLine = computed(() => stepPath(platformCols.value.map(col => col.count), false))
const retentionArea = computed(() => stepPath(platformCols.value.map(col => col.count), true))

function stepPath(values: number[], closed: boolean) {
  const max = Math.max(...values, 1)
  const n = Math.max(values.length, 1)
  const left = 8
  const right = 232
  const top = 10
  const bottom = 80
  const step = (right - left) / n
  let d = `M ${left} ${bottom}`
  values.forEach((value, i) => {
    const y = bottom - (value / max) * (bottom - top)
    const x0 = left + i * step
    const x1 = left + (i + 1) * step
    d += ` H ${x0} V ${y} H ${x1}`
  })
  if (closed) d += ` V ${bottom} H ${left} Z`
  return d
}

function sparkFrom(values: number[]) {
  const max = Math.max(...values, 1)
  const out: number[] = []
  for (let i = 0; i < 10; i++) {
    const v = values[i % values.length] || 0
    out.push(Math.max(22, Math.round((v / max) * 100)))
  }
  return out
}

const countSpark = computed(() => sparkFrom(platformCols.value.map(col => col.count)))
const appliedSpark = computed(() => sparkFrom(volumeRows.value.map(row => (row.count > 0 ? row.count : 0.2))))

const toolCount = computed(() => {
  const tool = configStore.currentFilter
  if (tool === 'all') return totalCount.value
  return configStore.environments.filter(env => env.provider === tool).length
})

const toolCountLabel = computed(() => {
  const tool = configStore.currentFilter
  if (tool === 'all') return '5 个平台'
  return toolLabel(tool)
})

const insightCopy = computed(() => {
  if (totalCount.value === 0) return '还没有环境配置。先新建一条，再点应用写入对应 CLI。'
  if (configuredCount.value === 0) return '配置已经建好，但还没有应用到 CLI。打开下方列表，点应用即可写入。'
  if (configuredCount.value < 5) {
    return `已写入 ${configuredCount.value} 个平台。其余平台可在列表里一键应用，预计补齐后覆盖率到 100%。`
  }
  return '五个平台都已写入 CLI。改配置后重新点应用，就会覆盖当前环境。'
})

function selectPlatform(id: Provider) {
  configStore.setFilter(configStore.currentFilter === id ? 'all' : id)
}

async function refresh() {
  await configStore.loadConfig()
}
</script>
