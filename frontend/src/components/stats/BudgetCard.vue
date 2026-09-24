<template>
  <Card>
    <CardHeader class="px-5">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <CardTitle>{{ t('budget.title') }}</CardTitle>
          <CardDescription class="mt-1 leading-relaxed">{{ t('budget.desc', { percent: settings.warn_percent }) }}</CardDescription>
        </div>
        <Button variant="outline" size="sm" class="shrink-0" @click="showEdit = true">
          <Wallet />
          {{ t('budget.edit') }}
        </Button>
      </div>
    </CardHeader>
    <CardContent class="px-5 pb-5">
      <p v-if="error" class="text-xs text-destructive">{{ error }}</p>
      <p v-else-if="statuses.length === 0" class="text-xs text-muted-foreground">{{ t('budget.empty') }}</p>
      <div v-else class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
        <div v-for="st in statuses" :key="st.rule.id" class="rounded-2xl bg-muted/50 px-4 py-3">
          <div class="flex items-center justify-between gap-2">
            <span class="truncate text-sm font-medium">{{ budgetScopeLabel(t, st.rule) }}</span>
            <Badge :variant="st.level === 'ok' ? 'secondary' : st.level === 'warn' ? 'outline' : 'destructive'" :class="st.level === 'warn' && 'border-amber-500/40 text-amber-600 dark:text-amber-400'">
              {{ t(`budget.level.${st.level}`) }}
            </Badge>
          </div>
          <p class="mt-1 text-xs text-muted-foreground tabular-nums">
            {{ t('budget.spentOf', { period: budgetPeriodLabel(t, st.rule.period), spent: formatUsd(st.spent), limit: formatUsd(st.rule.limit) }) }}
            · {{ Math.round(st.percent) }}%
          </p>
          <Progress :model-value="Math.min(100, st.percent)" class="mt-2 h-1.5" :color="levelColor(st.level)" />
        </div>
      </div>
    </CardContent>
    <BudgetEditModal v-model="showEdit" :settings="settings" @saved="reload" />
  </Card>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Wallet } from '@lucide/vue'
import type { BudgetSettings, BudgetStatus } from '@/types'
import { budgetService } from '@/services/budgetService'
import { budgetPeriodLabel, budgetScopeLabel, formatUsd } from '@/lib/budget'
import { useI18n } from '@/composables/useI18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import BudgetEditModal from './BudgetEditModal.vue'

const { t } = useI18n()

const settings = ref<BudgetSettings>({ rules: [], warn_percent: 80 })
const statuses = ref<BudgetStatus[]>([])
const error = ref('')
const showEdit = ref(false)

function levelColor(level: string) {
  if (level === 'over') return '#EF4444'
  if (level === 'warn') return '#F59E0B'
  return '#10B981'
}

async function reload() {
  error.value = ''
  try {
    settings.value = await budgetService.getSettings()
    statuses.value = await budgetService.status()
  } catch (e) {
    error.value = t('budget.loadFailed', { error: e instanceof Error ? e.message : String(e) })
  }
}

defineExpose({ reload })

onMounted(reload)
</script>
