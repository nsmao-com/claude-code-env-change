<template>
  <motion.div
    :initial="enter.initial"
    :animate="enter.animate"
    :transition="enter.transition"
    :while-press="pressSpring"
    :while-hover="hoverLift"
  >
  <Card
    :class="['cursor-pointer transition-colors hover:bg-muted/70', isActive ? 'ring-black/10 dark:ring-white/20' : '']"
    @click="$emit('click')"
  >
    <CardHeader class="gap-2">
      <div class="flex items-start justify-between gap-2">
        <div class="flex min-w-0 items-center gap-2">
          <div class="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted text-sm">
            {{ config.icon || '⌘' }}
          </div>
          <div class="min-w-0">
            <CardTitle class="truncate text-sm">{{ config.name }}</CardTitle>
            <CardDescription class="flex items-center gap-1">
              <BrandIcon :provider="config.provider || 'claude'" class="size-3" />
              {{ providerLabel }}
            </CardDescription>
          </div>
        </div>
        <div class="flex shrink-0 items-center gap-1.5">
          <Badge v-if="needsRoute" variant="outline" class="border-brand/30 bg-brand/10 text-brand">需路由</Badge>
          <Badge v-if="isActive" variant="secondary">当前</Badge>
        </div>
      </div>
    </CardHeader>
    <CardContent class="space-y-3">
      <p v-if="config.description" class="line-clamp-2 text-xs text-muted-foreground">{{ config.description }}</p>
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
      </div>
      <div v-if="isUptimeEnabled" class="flex items-center justify-between">
        <Badge variant="outline">{{ uptimeBadgeText }}</Badge>
      </div>
    </CardContent>
    <CardFooter class="gap-1">
      <Button size="sm" class="flex-1" @click.stop="$emit('apply')">应用</Button>
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
    </CardFooter>
  </Card>
  </motion.div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { motion } from 'motion-v'
import { Copy, Gauge, Pencil, Trash2 } from '@lucide/vue'
import type { EnvConfig, UptimeCheck } from '@/types'
import { useUptimeStore } from '@/stores/uptimeStore'
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
  if (!baseUrlValue.value?.trim()) return '无地址'
  const last = latestCheck.value
  if (!last) return '未测'
  return last.success ? `${last.latency_ms}ms` : '失败'
})
</script>
