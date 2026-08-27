<template>
  <div
    :class="[
      'group rounded-lg border border-border bg-background transition-colors hover:border-primary/50',
      compact ? 'p-2.5' : 'p-4',
    ]"
  >
    <div class="flex items-center justify-between gap-3">
      <div class="flex min-w-0 flex-1 items-center gap-3">
        <div
          :class="[
            'flex shrink-0 items-center justify-center rounded-lg bg-primary/10',
            compact ? 'size-8' : 'size-10',
          ]"
        >
          <Globe v-if="server.type === 'http'" :class="['text-primary', compact ? 'size-3.5' : 'size-5']" />
          <Terminal v-else :class="['text-primary', compact ? 'size-3.5' : 'size-5']" />
        </div>

        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <h4 :class="['font-semibold', compact ? 'text-xs' : 'text-sm']">{{ server.name }}</h4>
            <Badge
              v-for="p in enabledPlatforms"
              :key="p.key"
              variant="outline"
              :class="['gap-1 text-[10px]', p.class]"
            >
              <BrandIcon :provider="p.brand" class="size-3" />
              {{ p.label }}
            </Badge>
            <Badge
              v-if="enabledPlatforms.length === 0"
              variant="outline"
              class="text-[10px] text-muted-foreground"
            >
              未启用
            </Badge>
            <Badge
              v-if="testResult"
              variant="outline"
              :class="testResultClass"
            >
              <Check v-if="testResult.success" />
              {{ testResult.latency }}ms
            </Badge>
          </div>

          <div v-if="!compact" class="mt-1 truncate font-mono text-xs text-muted-foreground">
            {{ detailInfo }}
          </div>

          <div v-if="!compact && server.tips" class="mt-1 text-xs text-muted-foreground">
            {{ server.tips }}
          </div>
        </div>
      </div>

      <div
        :class="[
          'flex shrink-0 gap-1 transition-opacity',
          compact ? 'opacity-100' : 'opacity-0 group-hover:opacity-100',
        ]"
      >
        <Button
          variant="ghost"
          size="icon-sm"
          title="测试连接"
          :disabled="isTesting"
          @click="$emit('test')"
        >
          <Loader2 v-if="isTesting" class="animate-spin" />
          <Zap v-else />
        </Button>
        <Button
          v-if="server.website"
          as="a"
          :href="server.website"
          target="_blank"
          variant="ghost"
          size="icon-sm"
          title="官网"
        >
          <ExternalLink />
        </Button>
        <Button
          variant="ghost"
          size="icon-sm"
          title="编辑"
          @click="$emit('edit')"
        >
          <Pencil />
        </Button>
        <Button
          variant="ghost"
          size="icon-sm"
          class="text-muted-foreground hover:text-destructive"
          title="删除"
          @click="$emit('delete')"
        >
          <Trash2 />
        </Button>
      </div>
    </div>

    <div
      v-if="!compact && hasPlaceholder"
      class="mt-2 flex items-start gap-1.5 rounded-lg border border-yellow-500/20 bg-yellow-500/10 p-2 text-xs text-yellow-600"
    >
      <TriangleAlert class="mt-0.5 size-3.5 shrink-0" />
      存在未填写的占位符: {{ server.missing_placeholders.join(', ') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Check, ExternalLink, Globe, Loader2, Pencil, Terminal, Trash2, TriangleAlert, Zap } from '@lucide/vue'
import type { MCPServer, MCPTestResult } from '@/types'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

interface Props {
  server: MCPServer
  testResult?: MCPTestResult
  isTesting?: boolean
  compact?: boolean
}

const props = defineProps<Props>()

defineEmits<{
  test: []
  edit: []
  delete: []
}>()

const platforms = computed(() => props.server.enable_platform || [])

const PLATFORM_ITEMS = [
  { key: 'claude-code', brand: 'claude', label: 'Claude', class: 'border-green-500/20 bg-green-500/10 text-green-500' },
  { key: 'codex', brand: 'codex', label: 'Codex', class: 'border-brand/20 bg-brand/10 text-brand' },
  { key: 'gemini', brand: 'gemini', label: 'Gemini', class: 'border-blue-500/20 bg-blue-500/10 text-blue-600 dark:text-blue-400' },
  { key: 'opencode', brand: 'opencode', label: 'OpenCode', class: 'border-foreground/15 bg-foreground/5 text-foreground/80' },
  { key: 'grok', brand: 'grok', label: 'Grok', class: 'border-stone-500/25 bg-stone-500/10 text-stone-600 dark:text-stone-400' },
]

const enabledPlatforms = computed(() => PLATFORM_ITEMS.filter(p => platforms.value.includes(p.key)))
const hasPlaceholder = computed(() =>
  props.server.missing_placeholders && props.server.missing_placeholders.length > 0
)

const detailInfo = computed(() => {
  if (props.server.type === 'http') {
    return props.server.url || '-'
  }
  return `${props.server.command || ''} ${(props.server.args || []).join(' ')}`
})

const testResultClass = computed(() => {
  if (!props.testResult) return ''
  return props.testResult.success
    ? 'border-green-500/20 bg-green-500/10 text-[10px] text-green-500'
    : 'border-red-500/20 bg-red-500/10 text-[10px] text-red-500'
})
</script>
