<template>
  <span
    v-if="statusText"
    :class="['mcp-status-badge', badgeClass]"
  >
    <i v-if="isLoading" class="fas fa-circle-notch fa-spin"></i>
    <template v-else>
      <span v-if="availableCount > 0" class="text-green-500">{{ availableCount }} 可用</span>
      <span v-if="failedCount > 0" class="text-red-500">{{ failedCount > 0 && availableCount > 0 ? ' / ' : '' }}{{ failedCount }} 失败</span>
    </template>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useMcpStore } from '@/stores/mcpStore'

const mcpStore = useMcpStore()

const isLoading = computed(() => mcpStore.isTestingAll)
const statusText = computed(() => mcpStore.testStatusText)
const availableCount = computed(() => mcpStore.availableCount)
const failedCount = computed(() => mcpStore.failedCount)

const badgeClass = computed(() => {
  if (isLoading.value) return 'bg-muted'
  if (failedCount.value > 0 && availableCount.value === 0) return 'bg-red-500/10'
  if (failedCount.value > 0) return 'bg-yellow-500/10'
  if (availableCount.value > 0) return 'bg-green-500/10'
  return 'bg-muted'
})
</script>
