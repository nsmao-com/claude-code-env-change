<template>
  <div
    :class="['config-item group', { active: isActive }]"
    @click="$emit('click')"
  >
    <!-- Active Badge -->
    <div v-if="isActive" class="absolute -top-1 -right-1 z-10">
      <div class="bg-foreground text-background text-[9px] font-bold uppercase tracking-wider px-2 py-1 rounded-full shadow-lg flex items-center gap-1">
        <i class="fas fa-check text-[8px]"></i>
        Active
      </div>
    </div>

    <!-- Header: Icon + Name + Provider Badge -->
    <div class="flex items-center justify-between mb-4">
      <div class="flex items-center gap-3 min-w-0 flex-1">
        <div class="w-10 h-10 rounded flex items-center justify-center flex-shrink-0 text-xl bg-background">
          {{ config.icon || '📦' }}
        </div>
        <div class="min-w-0 flex-1">
          <h4 class="font-bold text-[15px] truncate leading-tight mb-1 font-mono tracking-tight" :title="config.name">{{ config.name }}</h4>
          <span class="text-[10px] px-2 py-0.5 rounded-full font-bold uppercase tracking-wider text-foreground bg-transparent border border-border">
            {{ providerLabel }}
          </span>
        </div>
      </div>
    </div>

    <!-- Description -->
    <p v-if="config.description" class="text-xs text-muted-foreground mb-4 line-clamp-2 h-8 leading-relaxed font-mono" :title="config.description">
      {{ config.description }}
    </p>
    <div v-else class="h-8 mb-4"></div>

    <!-- Details Grid -->
    <div class="grid grid-cols-2 gap-3 mb-4">
      <div class="p-2 rounded bg-muted/50">
        <div class="text-[9px] text-muted-foreground uppercase mb-1 font-bold tracking-widest">Model</div>
        <div class="text-xs font-mono truncate" :title="modelValue">{{ modelValue || '-' }}</div>
      </div>
      <div class="p-2 rounded bg-muted/50">
        <div class="text-[9px] text-muted-foreground uppercase mb-1 font-bold tracking-widest">Base URL</div>
        <div class="text-xs font-mono truncate" :title="baseUrlValue">{{ baseUrlValue || '-' }}</div>
      </div>
    </div>

    <!-- Uptime -->
    <div class="mb-4">
      <div class="flex items-center justify-between mb-2">
        <div class="text-[9px] text-muted-foreground uppercase font-bold tracking-widest">Uptime</div>
        <AppTooltip :content="uptimeBadgeTooltip">
          <span
            :class="[
              'text-[10px] font-mono px-2 py-0.5 rounded-full border border-border',
              uptimeBadgeClass
            ]"
          >
            {{ uptimeBadgeText }}
          </span>
        </AppTooltip>
      </div>
      <div class="flex items-center gap-2">
        <AppTooltip
          v-for="dot in uptimeDots"
          :key="dot.key"
          :content="dot.title"
        >
          <span
            :class="['w-2.5 h-2.5 rounded-md border border-border transition-transform hover:scale-110', dot.class]"
            tabindex="0"
          ></span>
        </AppTooltip>
      </div>
    </div>

    <!-- Action Bar -->
    <div class="flex items-center gap-2 pt-3 border-t border-dashed border-border mt-auto">
       <button
         class="flex-1 h-8 rounded border border-border hover:bg-foreground hover:text-background transition-all text-xs font-bold uppercase tracking-wider flex items-center justify-center gap-2"
        @click.stop="$emit('apply')"
      >
        <i class="fas fa-play text-[10px]"></i> Apply
      </button>

      <div class="flex gap-1">
        <button
          class="w-8 h-8 rounded border border-transparent hover:border-border flex items-center justify-center text-muted-foreground hover:text-foreground transition-all"
          title="Test Latency"
          @click.stop="$emit('testLatency')"
        >
          <i class="fas fa-tachometer-alt text-xs"></i>
        </button>
        <button
          class="w-8 h-8 rounded border border-transparent hover:border-border flex items-center justify-center text-muted-foreground hover:text-foreground transition-all"
          title="Duplicate"
          @click.stop="$emit('duplicate')"
        >
          <i class="fas fa-copy text-xs"></i>
        </button>
        <button
          class="w-8 h-8 rounded border border-transparent hover:border-border flex items-center justify-center text-muted-foreground hover:text-foreground transition-all"
          title="Edit"
          @click.stop="$emit('edit')"
        >
          <i class="fas fa-pen text-xs"></i>
        </button>
         <button
          class="w-8 h-8 rounded border border-transparent hover:border-destructive hover:bg-destructive hover:text-destructive-foreground flex items-center justify-center text-muted-foreground transition-all"
          title="Delete"
          @click.stop="$emit('delete')"
        >
          <i class="fas fa-trash text-xs"></i>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { EnvConfig, UptimeCheck } from '@/types'
import { useUptimeStore } from '@/stores/uptimeStore'
import AppTooltip from '@/components/common/AppTooltip.vue'

interface Props {
  config: EnvConfig
  isActive?: boolean
}

const props = defineProps<Props>()
const uptimeStore = useUptimeStore()

defineEmits<{
  click: []
  apply: []
  duplicate: []
  edit: []
  delete: []
  testLatency: []
}>()

const providerLabel = computed(() => {
  const labels: Record<string, string> = {
    claude: 'Claude',
    codex: 'Codex',
    gemini: 'Gemini'
  }
  const provider = (props.config.provider || 'claude').toLowerCase()
  return labels[provider] || provider
})

const modelValue = computed(() => {
  const provider = (props.config.provider || 'claude').toLowerCase()
  const vars = props.config.variables || {}

  if (provider === 'claude') {
    return vars.ANTHROPIC_MODEL || ''
  } else if (provider === 'codex') {
    return vars.model || ''
  } else if (provider === 'gemini') {
    return vars.GEMINI_MODEL || ''
  }
  return ''
})

const baseUrlValue = computed(() => {
  const provider = (props.config.provider || 'claude').toLowerCase()
  const vars = props.config.variables || {}

  if (provider === 'claude') {
    return vars.ANTHROPIC_BASE_URL || vars.API_BASE_URL || ''
  } else if (provider === 'codex') {
    return vars.base_url || ''
  } else if (provider === 'gemini') {
    return vars.GOOGLE_GEMINI_BASE_URL || ''
  }
  return ''
})

const isUptimeEnabled = computed(() => !!uptimeStore.settings.enabled)
const hasUptimeURL = computed(() => !!baseUrlValue.value?.trim())
const uptimeHistory = computed<UptimeCheck[]>(() => uptimeStore.getHistory(props.config.name))
const latestCheck = computed<UptimeCheck | null>(() => {
  const list = uptimeHistory.value
  return list.length ? list[list.length - 1] : null
})

function formatAgo(atSeconds: number): string {
  const now = Math.floor(Date.now() / 1000)
  const diff = Math.max(0, now - atSeconds)
  if (diff < 60) return `${diff}秒前`
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
  return `${Math.floor(diff / 3600)}小时前`
}

const uptimeBadgeText = computed(() => {
  if (!isUptimeEnabled.value) return 'OFF'
  if (!hasUptimeURL.value) return 'NO URL'
  const last = latestCheck.value
  if (!last) return '—'
  if (last.success) return `${last.latency_ms}ms`
  return 'FAIL'
})

const uptimeBadgeClass = computed(() => {
  if (!isUptimeEnabled.value) return 'text-muted-foreground bg-muted/40'
  if (!hasUptimeURL.value) return 'text-muted-foreground bg-muted/40'
  const last = latestCheck.value
  if (!last) return 'text-muted-foreground bg-muted/40'
  return last.success ? 'text-green-600 bg-green-500/10' : 'text-red-600 bg-red-500/10'
})

const uptimeBadgeTooltip = computed(() => {
  if (!isUptimeEnabled.value) return '监控未启用'
  if (!hasUptimeURL.value) return '未配置 Base URL\n无法检测'
  const last = latestCheck.value
  if (!last) return '暂无检测记录'
  if (last.success) return `成功\nHTTP ${last.status_code}\n延迟 ${last.latency_ms}ms\n${formatAgo(last.at)}`
  return `失败\n${last.error || '检测失败'}\n${formatAgo(last.at)}`
})

const uptimeDots = computed(() => {
  const keep = 10
  const history = uptimeHistory.value || []
  const trimmed = history.slice(Math.max(0, history.length - keep))
  const padded: Array<UptimeCheck | null> = [
    ...Array.from({ length: Math.max(0, keep - trimmed.length) }).map(() => null),
    ...trimmed
  ]

  return padded.map((c, idx) => {
    if (!isUptimeEnabled.value) {
      return { key: `off-${idx}`, class: 'bg-muted/30', title: '监控未启用' }
    }
    if (!hasUptimeURL.value) {
      return { key: `nourl-${idx}`, class: 'bg-muted/30', title: '未配置 Base URL' }
    }
    if (!c) {
      return { key: `empty-${idx}`, class: 'bg-muted/30', title: '无记录' }
    }
    const title = c.success
      ? `成功\nHTTP ${c.status_code}\n延迟 ${c.latency_ms}ms\n${formatAgo(c.at)}`
      : `失败\n${c.error || '失败'}\n${formatAgo(c.at)}`
    return {
      key: `check-${c.at}-${idx}`,
      class: c.success ? 'bg-green-500/60' : 'bg-red-500/60',
      title
    }
  })
})
</script>
