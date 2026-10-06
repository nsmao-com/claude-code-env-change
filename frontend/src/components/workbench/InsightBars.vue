<script setup lang="ts">
import { computed } from 'vue'
import type { InsightCount } from '@/types/workbench'

const props = withDefaults(
  defineProps<{
    title: string
    items: InsightCount[] | null | undefined
    by?: 'count' | 'tokens'
    empty: string
    format?: (item: InsightCount) => string
  }>(),
  { by: 'count', format: undefined },
)

const rows = computed(() => {
  const list = props.items || []
  const max = Math.max(1, ...list.map((i) => (props.by === 'tokens' ? i.tokens || 0 : i.count)))
  return list.map((i) => {
    const value = props.by === 'tokens' ? i.tokens || 0 : i.count
    return { item: i, pct: Math.max(2, Math.round((value / max) * 100)) }
  })
})
</script>

<template>
  <section class="rounded-xl border border-border p-4">
    <h3 class="text-sm">{{ title }}</h3>
    <p v-if="!rows.length" class="text-xs text-muted-foreground">{{ empty }}</p>
    <ol v-else class="space-y-2">
      <li v-for="row in rows" :key="row.item.name" class="text-xs">
        <div class="flex items-baseline justify-between gap-3">
          <span class="min-w-0 truncate" :title="row.item.name">{{ row.item.name }}</span>
          <span class="shrink-0 tabular-nums text-muted-foreground">{{ format ? format(row.item) : row.item.count }}</span>
        </div>
        <div class="mt-1 h-1.5 rounded-full bg-muted">
          <div class="h-full rounded-full bg-brand/80" :style="{ width: `${row.pct}%` }" />
        </div>
      </li>
    </ol>
  </section>
</template>
