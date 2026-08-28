<template>
  <motion.div
    :initial="enter.initial"
    :animate="enter.animate"
    :transition="enter.transition"
    :while-press="nested ? undefined : pressSpring"
    :while-hover="nested ? undefined : hoverLift"
    :class="[
      'flex min-w-0 w-full cursor-pointer items-center gap-3 overflow-hidden px-4 py-3 transition-colors hover:bg-muted/70',
      nested
        ? 'rounded-none bg-transparent'
        : 'rounded-2xl bg-card ring-1 ring-black/[0.04] dark:ring-white/10',
      !nested && isActive ? 'bg-primary/8 ring-2 ring-primary/35' : '',
      nested && isActive ? 'border-l-2 border-l-primary bg-primary/10' : '',
    ]"
    @click="$emit('click')"
  >
    <GripVertical class="size-3.5 shrink-0 text-muted-foreground/50" />
    <div
      :class="[
        'flex size-7 shrink-0 items-center justify-center rounded-md text-sm',
        isActive ? 'bg-primary text-primary-foreground' : 'bg-muted',
      ]"
    >
      {{ config.icon || '⌘' }}
    </div>
    <div class="min-w-0 flex-1 overflow-hidden">
      <div class="flex min-w-0 items-center gap-2">
        <AppTooltip :content="config.name" wrap class="min-w-0 flex-1">
          <span class="block w-full truncate text-sm font-medium">{{ config.name }}</span>
        </AppTooltip>
        <Badge v-if="needsRoute" variant="outline" class="shrink-0 border-brand/30 bg-brand/10 text-brand">需开启路由</Badge>
        <Badge v-if="isActive" class="shrink-0 gap-1">
          <Check class="size-3" />
          使用中
        </Badge>
      </div>
      <p class="mt-0.5 truncate text-xs text-muted-foreground">{{ config.description?.trim() || '暂无描述' }}</p>
    </div>
    <span class="flex w-[88px] shrink-0 items-center gap-1 text-xs text-muted-foreground">
      <BrandIcon :provider="config.provider || 'claude'" class="size-3 shrink-0" />
      <span class="truncate">{{ providerLabel }}</span>
    </span>
    <AppTooltip class="hidden w-36 shrink-0 sm:inline-flex" :content="modelValue" wrap :disabled="!modelValue">
      <span class="w-full truncate font-mono text-xs">{{ modelValue || '-' }}</span>
    </AppTooltip>
    <AppTooltip class="hidden w-44 shrink-0 xl:inline-flex" :content="baseUrlValue" wrap :disabled="!baseUrlValue">
      <span class="w-full truncate font-mono text-xs">{{ baseUrlValue || '-' }}</span>
    </AppTooltip>
    <Badge v-if="isUptimeEnabled" variant="outline">{{ uptimeBadgeText }}</Badge>
    <div class="flex shrink-0 items-center gap-1.5">
      <Button size="sm" @click.stop="$emit('apply')">应用</Button>
      <span v-if="latencyLabel" class="shrink-0 text-right text-[11px] tabular-nums text-muted-foreground">{{ latencyLabel }}</span>
      <AppTooltip content="测速">
        <Button variant="ghost" size="icon-sm" :disabled="testing" @pointerdown.stop @click.stop="onTestLatency">
          <Loader2 v-if="testing" class="animate-spin" />
          <Gauge v-else />
        </Button>
      </AppTooltip>
      <AppTooltip content="复制">
        <Button variant="ghost" size="icon-sm" @click.stop="$emit('duplicate')">
          <Copy />
        </Button>
      </AppTooltip>
      <AppTooltip content="编辑">
        <Button variant="ghost" size="icon-sm" @click.stop="$emit('edit')">
          <Pencil />
        </Button>
      </AppTooltip>
      <AppTooltip content="删除">
        <Button variant="ghost" size="icon-sm" @click.stop="$emit('delete')">
          <Trash2 class="text-destructive" />
        </Button>
      </AppTooltip>
    </div>
  </motion.div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { motion } from 'motion-v'
import { Check, Copy, Gauge, GripVertical, Loader2, Pencil, Trash2 } from '@lucide/vue'
import type { EnvConfig, UptimeCheck } from '@/types'
import { useUptimeStore } from '@/stores/uptimeStore'
import { useConfigLatency } from '@/composables/useConfigLatency'
import { hoverLift, listEnter, pressSpring } from '@/lib/motion'
import AppTooltip from '@/components/common/AppTooltip.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  config: EnvConfig
  isActive?: boolean
  index?: number
  nested?: boolean
}>()

const enter = computed(() => listEnter(props.index ?? 0))

defineEmits<{
  click: []
  apply: []
  duplicate: []
  edit: []
  delete: []
}>()

const uptimeStore = useUptimeStore()
const { testing, latencyLabel, test } = useConfigLatency()

function onTestLatency() {
  test(props.config)
}

const providerLabel = computed(() => {
  const labels: Record<string, string> = { claude: 'Claude', codex: 'Codex', gemini: 'Gemini', opencode: 'OpenCode', grok: 'Grok' }
  return labels[(props.config.provider || 'claude').toLowerCase()] || props.config.provider
})

const needsRoute = computed(() => !!props.config.upstream_format)

const modelValue = computed(() => {
  const provider = (props.config.provider || 'claude').toLowerCase()
  const vars = props.config.variables || {}
  if (provider === 'claude') return vars.ANTHROPIC_MODEL || ''
  if (provider === 'codex') return vars.model || ''
  if (provider === 'gemini') return vars.GEMINI_MODEL || ''
  if (provider === 'opencode') return vars.OPENCODE_MODEL || ''
  if (provider === 'grok') return vars.XAI_MODEL || ''
  return ''
})

const baseUrlValue = computed(() => {
  const provider = (props.config.provider || 'claude').toLowerCase()
  const vars = props.config.variables || {}
  if (provider === 'claude') return vars.ANTHROPIC_BASE_URL || vars.API_BASE_URL || ''
  if (provider === 'codex') return vars.base_url || ''
  if (provider === 'gemini') return vars.GOOGLE_GEMINI_BASE_URL || ''
  if (provider === 'opencode') return vars.OPENCODE_BASE_URL || ''
  if (provider === 'grok') return vars.XAI_BASE_URL || ''
  return ''
})

const isUptimeEnabled = computed(() => !!uptimeStore.settings.enabled)
const uptimeHistory = computed<UptimeCheck[]>(() => uptimeStore.getHistory(props.config.name))
const latestCheck = computed(() => uptimeHistory.value.at(-1) ?? null)
const uptimeBadgeText = computed(() => {
  if (!baseUrlValue.value?.trim()) return '-'
  const last = latestCheck.value
  if (!last) return '未测'
  return last.success ? `${last.latency_ms}ms` : '失败'
})
</script>
