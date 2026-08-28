<template>
  <div
    :class="[
      'group min-w-0 overflow-hidden rounded-lg border border-border bg-background transition-colors hover:border-primary/50',
      compact ? 'p-2.5' : 'p-4',
    ]"
  >
    <div class="flex min-w-0 items-center justify-between gap-3">
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

        <div class="min-w-0 flex-1 overflow-hidden">
          <AppTooltip :content="server.name" wrap class="block w-full min-w-0">
            <h4 :class="['min-w-0 truncate font-semibold', compact ? 'text-xs' : 'text-sm']">{{ server.name }}</h4>
          </AppTooltip>
          <div class="mt-1 flex min-w-0 flex-wrap items-center gap-2">
            <PlatformChips
              :enabled="platforms"
              :compact="compact"
              @toggle="$emit('toggle-platform', $event)"
            />
            <Badge
              v-if="testResult"
              variant="outline"
              :class="testResultClass"
            >
              <Check v-if="testResult.success" />
              {{ testResult.latency }}ms
            </Badge>
          </div>

          <AppTooltip v-if="!compact" :content="detailInfo" wrap class="mt-1 block w-full min-w-0">
            <div class="truncate font-mono text-xs text-muted-foreground">
              {{ detailInfo }}
            </div>
          </AppTooltip>

          <div v-if="!compact && server.tips" class="mt-1 line-clamp-2 break-words text-xs text-muted-foreground">
            {{ server.tips }}
          </div>
        </div>
      </div>

      <div
        :class="[
          'flex shrink-0 gap-1.5 transition-opacity',
          compact ? 'opacity-100' : 'opacity-0 group-hover:opacity-100',
        ]"
      >
        <AppTooltip content="测试连接">
        <Button
          variant="ghost"
          size="icon-sm"
          :disabled="isTesting"
          @click="$emit('test')"
        >
          <Loader2 v-if="isTesting" class="animate-spin" />
          <Zap v-else />
        </Button>
        </AppTooltip>
        <AppTooltip v-if="server.website" content="官网">
        <Button
          as="a"
          :href="server.website"
          target="_blank"
          variant="ghost"
          size="icon-sm"
        >
          <ExternalLink />
        </Button>
        </AppTooltip>
        <AppTooltip content="编辑">
        <Button
          variant="ghost"
          size="icon-sm"
          @click="$emit('edit')"
        >
          <Pencil />
        </Button>
        </AppTooltip>
        <AppTooltip content="删除">
        <Button
          variant="ghost"
          size="icon-sm"
          class="text-muted-foreground hover:text-destructive"
          @click="$emit('delete')"
        >
          <Trash2 />
        </Button>
        </AppTooltip>
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
import AppTooltip from '@/components/common/AppTooltip.vue'
import PlatformChips from '@/components/common/PlatformChips.vue'
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
  'toggle-platform': [platform: string]
}>()

const platforms = computed(() => props.server.enable_platform || [])
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
