<template>
  <motion.div
    :initial="enter.initial"
    :animate="enter.animate"
    :transition="enter.transition"
    :while-press="nested ? undefined : pressSpring"
    :while-hover="nested ? undefined : hoverLift"
    :class="[
      'flex cursor-pointer items-center gap-3 px-4 py-3 transition-colors hover:bg-muted/70',
      nested
        ? 'rounded-none bg-transparent'
        : 'rounded-2xl bg-card ring-1 ring-black/[0.04] dark:ring-white/10',
      !nested && isActive ? 'ring-black/10 dark:ring-white/25' : '',
      nested && isActive ? 'bg-muted/50' : '',
    ]"
    @click="$emit('click')"
  >
    <GripVertical class="size-3.5 shrink-0 text-muted-foreground/50" />
    <div class="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted text-sm">
      {{ config.icon || '⌘' }}
    </div>
    <div class="min-w-0 flex-1">
      <div class="flex items-center gap-2">
        <span class="truncate text-sm font-medium">{{ config.name }}</span>
        <span class="flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
          <BrandIcon :provider="config.provider || 'claude'" class="size-3" />
          {{ providerLabel }}
        </span>
        <Badge v-if="needsRoute" variant="outline" class="border-brand/30 bg-brand/10 text-brand">需路由</Badge>
        <Badge v-if="isActive" variant="secondary">当前</Badge>
      </div>
      <p v-if="config.description" class="truncate text-xs text-muted-foreground">{{ config.description }}</p>
    </div>
    <AppTooltip class="hidden w-40 shrink-0 lg:inline-flex" :content="modelValue" wrap :disabled="!modelValue">
      <span class="w-full truncate font-mono text-xs">{{ modelValue || '-' }}</span>
    </AppTooltip>
    <AppTooltip class="hidden w-52 shrink-0 xl:inline-flex" :content="baseUrlValue" wrap :disabled="!baseUrlValue">
      <span class="w-full truncate font-mono text-xs">{{ baseUrlValue || '-' }}</span>
    </AppTooltip>
    <Badge v-if="isUptimeEnabled" variant="outline">{{ uptimeBadgeText }}</Badge>
    <div class="flex shrink-0 items-center">
      <Button size="sm" @click.stop="$emit('apply')">应用</Button>
      <Button variant="ghost" size="icon-sm" title="测速" @click.stop="$emit('testLatency')">
        <Gauge />
      </Button>
      <Button variant="ghost" size="icon-sm" title="复制" @click.stop="$emit('duplicate')">
        <Copy />
      </Button>
      <Button variant="ghost" size="icon-sm" title="编辑" @click.stop="$emit('edit')">
        <Pencil />
      </Button>
      <Button variant="ghost" size="icon-sm" title="删除" @click.stop="$emit('delete')">
        <Trash2 class="text-destructive" />
      </Button>
    </div>
  </motion.div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { motion } from 'motion-v'
import { Copy, Gauge, GripVertical, Pencil, Trash2 } from '@lucide/vue'
import type { EnvConfig, UptimeCheck } from '@/types'
import { useUptimeStore } from '@/stores/uptimeStore'
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
  testLatency: []
}>()

const uptimeStore = useUptimeStore()

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
