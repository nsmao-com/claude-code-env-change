<template>
  <div
    :class="['config-item config-row group flex items-center gap-4', { active: isActive }]"
    @click="$emit('click')"
  >
    <!-- Active Badge -->
    <div v-if="isActive" class="absolute -top-1 -right-1 z-10">
      <div class="bg-foreground text-background text-[9px] font-bold uppercase tracking-wider px-2 py-1 rounded-full shadow-lg flex items-center gap-1">
        <i class="fas fa-check text-[8px]"></i>
        Active
      </div>
    </div>

    <!-- Drag hint -->
    <div class="text-muted-foreground/60 w-6 flex-none flex items-center justify-center">
      <i class="fas fa-grip-vertical text-xs"></i>
    </div>

    <!-- Icon -->
    <div class="w-10 h-10 rounded flex items-center justify-center flex-shrink-0 text-xl bg-background">
      {{ config.icon || '📦' }}
    </div>

    <!-- Name / Desc -->
    <div class="min-w-0 flex-1">
      <div class="flex items-center gap-2 min-w-0">
        <h4 class="font-bold text-[14px] truncate leading-tight font-mono tracking-tight" :title="config.name">
          {{ config.name }}
        </h4>
        <span class="text-[10px] px-2 py-0.5 rounded-full font-bold uppercase tracking-wider text-foreground bg-transparent border border-border flex-none">
          {{ providerLabel }}
        </span>
      </div>
      <p v-if="config.description" class="text-[11px] text-muted-foreground truncate mt-0.5" :title="config.description">
        {{ config.description }}
      </p>
      <p v-else class="text-[11px] text-muted-foreground/60 mt-0.5">（无描述）</p>
    </div>

    <!-- Model -->
    <div class="hidden lg:block w-[180px] flex-none">
      <div class="text-[9px] text-muted-foreground uppercase mb-1 font-bold tracking-widest">Model</div>
      <div class="text-xs font-mono truncate" :title="modelValue">{{ modelValue || '-' }}</div>
    </div>

    <!-- Base URL -->
    <div class="hidden xl:block w-[260px] flex-none">
      <div class="text-[9px] text-muted-foreground uppercase mb-1 font-bold tracking-widest">Base URL</div>
      <div class="text-xs font-mono truncate" :title="baseUrlValue">{{ baseUrlValue || '-' }}</div>
    </div>

    <!-- Badges -->
    <div class="flex items-center gap-2 flex-none">
      <AppTooltip :content="uptimeTooltip">
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

    <!-- Actions -->
    <div class="flex items-center gap-1 flex-none">
      <button
        class="w-8 h-8 rounded border border-transparent hover:border-border flex items-center justify-center text-muted-foreground hover:text-foreground transition-all"
        title="Apply"
        @click.stop="$emit('apply')"
      >
        <i class="fas fa-play text-xs"></i>
      </button>
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
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { EnvConfig, UptimeCheck } from '@/types'
import AppTooltip from '@/components/common/AppTooltip.vue'
import { useUptimeStore } from '@/stores/uptimeStore'

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

  if (provider === 'claude') return vars.ANTHROPIC_MODEL || ''
  if (provider === 'codex') return vars.model || ''
  if (provider === 'gemini') return vars.GEMINI_MODEL || ''
  return ''
})

const baseUrlValue = computed(() => {
  const provider = (props.config.provider || 'claude').toLowerCase()
  const vars = props.config.variables || {}

  if (provider === 'claude') return vars.ANTHROPIC_BASE_URL || vars.API_BASE_URL || ''
  if (provider === 'codex') return vars.base_url || ''
  if (provider === 'gemini') return vars.GOOGLE_GEMINI_BASE_URL || ''
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

const uptimeTooltip = computed(() => {
  if (!isUptimeEnabled.value) return '监控未启用'
  if (!hasUptimeURL.value) return '未配置 Base URL\n无法检测'
  const last = latestCheck.value
  if (!last) return '暂无检测记录'
  if (last.success) return `成功\nHTTP ${last.status_code}\n延迟 ${last.latency_ms}ms\n${formatAgo(last.at)}`
  return `失败\n${last.error || '检测失败'}\n${formatAgo(last.at)}`
})
</script>
