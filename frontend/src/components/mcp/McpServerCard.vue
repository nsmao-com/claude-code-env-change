<template>
  <div class="p-4 rounded-lg border border-border bg-background hover:border-primary/50 transition-all group">
    <div class="flex items-start justify-between">
      <div class="flex items-center gap-3 min-w-0 flex-1">
        <!-- Icon -->
        <div class="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center flex-shrink-0">
          <i :class="['fas', typeIcon, 'text-primary']"></i>
        </div>

        <!-- Info -->
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2 mb-1">
            <h4 class="font-semibold text-sm">{{ server.name }}</h4>
            <!-- Platform badges -->
            <span
              v-if="hasClaude"
              class="text-[10px] px-1.5 py-0.5 rounded bg-green-500/10 text-green-500 border border-green-500/20"
            >
              Claude
            </span>
            <span
              v-if="hasCodex"
              class="text-[10px] px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-500 border border-blue-500/20"
            >
              Codex
            </span>
            <span
              v-if="!hasClaude && !hasCodex"
              class="text-[10px] px-1.5 py-0.5 rounded bg-muted text-muted-foreground border border-border"
            >
              未启用
            </span>
            <!-- Test result badge -->
            <span v-if="testResult" :class="['text-[10px] px-1.5 py-0.5 rounded', testResultClass]">
              <i v-if="testResult.success" class="fas fa-check mr-1"></i>
              <i v-else class="fas fa-times mr-1"></i>
              {{ testResult.latency }}ms
            </span>
          </div>

          <!-- Detail -->
          <div class="text-xs text-muted-foreground font-mono truncate">
            {{ detailInfo }}
          </div>

          <!-- Tips -->
          <div v-if="server.tips" class="text-xs text-muted-foreground mt-1">
            {{ server.tips }}
          </div>
        </div>
      </div>

      <!-- Actions -->
      <div class="opacity-0 group-hover:opacity-100 transition-opacity flex gap-1 flex-shrink-0 ml-2">
        <button
          class="w-7 h-7 rounded hover:bg-muted flex items-center justify-center text-muted-foreground hover:text-foreground"
          title="测试连接"
          :disabled="isTesting"
          @click="$emit('test')"
        >
          <i :class="['fas', isTesting ? 'fa-circle-notch fa-spin' : 'fa-bolt', 'text-xs']"></i>
        </button>
        <a
          v-if="server.website"
          :href="server.website"
          target="_blank"
          class="w-7 h-7 rounded hover:bg-muted flex items-center justify-center text-muted-foreground hover:text-foreground"
          title="官网"
        >
          <i class="fas fa-external-link-alt text-xs"></i>
        </a>
        <button
          class="w-7 h-7 rounded hover:bg-muted flex items-center justify-center text-muted-foreground hover:text-foreground"
          title="编辑"
          @click="$emit('edit')"
        >
          <i class="fas fa-pen text-xs"></i>
        </button>
        <button
          class="w-7 h-7 rounded hover:bg-destructive/10 flex items-center justify-center text-muted-foreground hover:text-destructive"
          title="删除"
          @click="$emit('delete')"
        >
          <i class="fas fa-trash text-xs"></i>
        </button>
      </div>
    </div>

    <!-- Placeholder Warning -->
    <div
      v-if="hasPlaceholder"
      class="mt-2 p-2 rounded bg-yellow-500/10 border border-yellow-500/20 text-xs text-yellow-600"
    >
      <i class="fas fa-exclamation-triangle mr-1"></i>
      存在未填写的占位符: {{ server.missing_placeholders.join(', ') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { MCPServer, MCPTestResult } from '@/types'

interface Props {
  server: MCPServer
  testResult?: MCPTestResult
  isTesting?: boolean
}

const props = defineProps<Props>()

defineEmits<{
  test: []
  edit: []
  delete: []
}>()

const typeIcon = computed(() => props.server.type === 'http' ? 'fa-globe' : 'fa-terminal')

const platforms = computed(() => props.server.enable_platform || [])
const hasClaude = computed(() => platforms.value.includes('claude-code'))
const hasCodex = computed(() => platforms.value.includes('codex'))
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
    ? 'bg-green-500/10 text-green-500 border border-green-500/20'
    : 'bg-red-500/10 text-red-500 border border-red-500/20'
})
</script>
