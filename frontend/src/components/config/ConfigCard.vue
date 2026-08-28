<template>
  <motion.div
    class="min-w-0"
    :initial="enter.initial"
    :animate="enter.animate"
    :transition="enter.transition"
    :while-press="pressSpring"
    :while-hover="hoverLift"
  >
  <Card
    :class="['cursor-pointer transition-colors hover:bg-muted/70', isActive ? 'border-primary bg-primary/8 ring-2 ring-primary/35' : '']"
    @click="$emit('click')"
  >
    <CardHeader class="gap-2">
      <div class="flex min-w-0 items-start justify-between gap-2">
        <div class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
          <div
            :class="[
              'flex size-8 shrink-0 items-center justify-center rounded-md text-sm',
              isActive ? 'bg-primary text-primary-foreground' : 'bg-muted',
            ]"
          >
            {{ config.icon || '⌘' }}
          </div>
          <div class="min-w-0 flex-1 overflow-hidden">
            <AppTooltip :content="config.name" wrap class="block w-full min-w-0">
              <CardTitle class="text-sm">{{ config.name }}</CardTitle>
            </AppTooltip>
            <CardDescription class="flex min-w-0 items-center gap-1 truncate">
              <BrandIcon :provider="config.provider || 'claude'" class="size-3 shrink-0" />
              <span class="truncate">{{ providerLabel }}</span>
            </CardDescription>
          </div>
        </div>
        <div class="flex shrink-0 items-center gap-1.5">
          <Badge v-if="needsRoute" variant="outline" class="border-brand/30 bg-brand/10 text-brand">需开启路由</Badge>
          <Badge v-if="isActive" class="gap-1">
            <Check class="size-3" />
            使用中
          </Badge>
        </div>
      </div>
    </CardHeader>
    <CardContent class="space-y-3">
      <p class="line-clamp-2 break-words text-xs text-muted-foreground">{{ config.description?.trim() || '暂无描述' }}</p>
      <div class="space-y-1 text-xs">
        <div class="flex justify-between gap-3">
          <span class="shrink-0 text-muted-foreground">模型</span>
          <span class="min-w-0 truncate font-mono">{{ modelValue || '-' }}</span>
        </div>
        <div class="flex justify-between gap-3">
          <span class="shrink-0 text-muted-foreground">地址</span>
          <AppTooltip :content="baseUrlValue" wrap :disabled="!baseUrlValue">
            <span class="min-w-0 truncate font-mono">{{ baseUrlValue || '-' }}</span>
          </AppTooltip>
        </div>
        <div class="flex justify-between gap-3">
          <span class="shrink-0 text-muted-foreground">延迟</span>
          <span class="min-w-0 truncate tabular-nums">{{ latencyLabel || '未测' }}</span>
        </div>
      </div>
      <div v-if="isUptimeEnabled" class="flex items-center justify-between gap-2">
        <Badge variant="outline">{{ uptimeBadgeText }}</Badge>
      </div>
    </CardContent>
    <CardFooter class="gap-1.5">
      <Button size="sm" class="flex-1" @click.stop="$emit('apply')">应用</Button>
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
    </CardFooter>
  </Card>
  </motion.div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { motion } from 'motion-v'
import { Check, Copy, Gauge, Loader2, Pencil, Trash2 } from '@lucide/vue'
import type { EnvConfig, UptimeCheck } from '@/types'
import { useUptimeStore } from '@/stores/uptimeStore'
import { useConfigLatency } from '@/composables/useConfigLatency'
import { hoverLift, listEnter, pressSpring } from '@/lib/motion'
import AppTooltip from '@/components/common/AppTooltip.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'

const props = defineProps<{
  config: EnvConfig
  isActive?: boolean
  index?: number
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
  if (!baseUrlValue.value?.trim()) return '无地址'
  const last = latestCheck.value
  if (!last) return '未测'
  return last.success ? `${last.latency_ms}ms` : '失败'
})
</script>
