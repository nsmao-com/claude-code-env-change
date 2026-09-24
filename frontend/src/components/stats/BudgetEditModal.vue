<template>
  <AppModal v-model="isOpen" :title="t('budget.editTitle')" size="lg" :close-on-overlay="false">
    <div class="space-y-4">
      <div class="grid max-w-xs gap-1.5">
        <Label>{{ t('budget.warnPercent') }}</Label>
        <Input v-model="warnPercent" type="number" min="1" max="99" />
        <p class="text-[11px] text-muted-foreground">{{ t('budget.warnPercentHint') }}</p>
      </div>

      <div class="space-y-2">
        <p v-if="rows.length === 0" class="rounded-xl border border-dashed px-3 py-6 text-center text-xs text-muted-foreground">{{ t('budget.noRules') }}</p>
        <div v-for="(row, i) in rows" :key="row.key" class="grid grid-cols-2 items-end gap-2 rounded-xl border p-3 sm:grid-cols-[repeat(4,minmax(0,1fr))_auto]">
          <div class="grid min-w-0 gap-1">
            <Label class="text-xs">{{ t('budget.scope') }}</Label>
            <Select v-model="row.scope">
              <SelectTrigger class="w-full min-w-0 overflow-hidden"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{{ t('budget.scopeAll') }}</SelectItem>
                <SelectItem value="provider">{{ t('budget.scopeProvider') }}</SelectItem>
                <SelectItem value="env">{{ t('budget.scopeEnv') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="grid min-w-0 gap-1">
            <Label class="text-xs">{{ row.scope === 'env' ? t('budget.env') : t('budget.provider') }}</Label>
            <Select v-if="row.scope === 'provider'" v-model="row.provider">
              <SelectTrigger class="w-full min-w-0 overflow-hidden"><SelectValue :placeholder="t('budget.provider')" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="p in BUDGET_PROVIDERS" :key="p.id" :value="p.id">{{ p.label }}</SelectItem>
              </SelectContent>
            </Select>
            <Select v-else-if="row.scope === 'env'" :model-value="envValue(row)" @update:model-value="v => setEnv(row, v)">
              <SelectTrigger class="w-full min-w-0 overflow-hidden"><SelectValue :placeholder="t('budget.pickEnv')" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="env in envOptions" :key="env.value" :value="env.value">{{ env.label }}</SelectItem>
              </SelectContent>
            </Select>
            <div v-else class="flex h-9 items-center text-xs text-muted-foreground">—</div>
          </div>
          <div class="grid min-w-0 gap-1">
            <Label class="text-xs">{{ t('budget.period') }}</Label>
            <Select v-model="row.period">
              <SelectTrigger class="w-full min-w-0 overflow-hidden"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="day">{{ t('budget.periodDay') }}</SelectItem>
                <SelectItem value="month">{{ t('budget.periodMonth') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="grid min-w-0 gap-1">
            <Label class="text-xs">{{ t('budget.limit') }}</Label>
            <Input v-model="row.limit" type="number" min="0.01" step="0.01" placeholder="10" />
          </div>
          <div class="col-span-2 flex h-9 items-center justify-end gap-2 sm:col-span-1">
            <Switch :checked="row.enabled" size="sm" @update:checked="(v: boolean) => row.enabled = v" />
            <AppTooltip :content="t('budget.removeRule')">
              <Button type="button" variant="ghost" size="icon-sm" class="text-muted-foreground hover:text-destructive" @click="rows.splice(i, 1)">
                <Trash2 />
              </Button>
            </AppTooltip>
          </div>
        </div>
        <Button type="button" variant="outline" size="sm" @click="addRow">
          <Plus />
          {{ t('budget.addRule') }}
        </Button>
      </div>
      <p class="text-[11px] leading-relaxed text-muted-foreground">{{ t('budget.note') }}</p>
    </div>

    <template #footer>
      <Button variant="secondary" @click="isOpen = false">{{ t('common.cancel') }}</Button>
      <Button :disabled="saving" @click="save">
        <Loader2 v-if="saving" class="animate-spin" />
        {{ t('common.save') }}
      </Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Loader2, Plus, Trash2 } from '@lucide/vue'
import type { BudgetRule, BudgetSettings } from '@/types'
import { budgetService } from '@/services/budgetService'
import { BUDGET_PROVIDERS, budgetProviderName } from '@/lib/budget'
import { useConfigStore } from '@/stores/configStore'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

const props = defineProps<{
  modelValue: boolean
  settings: BudgetSettings
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const { t } = useI18n()
const toast = useToast()
const configStore = useConfigStore()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
})

interface Row {
  key: number
  id: string
  scope: string
  provider: string
  env_name: string
  period: string
  limit: string | number
  enabled: boolean
}

let seq = 0
const rows = ref<Row[]>([])
const warnPercent = ref<string | number>(80)
const saving = ref(false)

const envOptions = computed(() => configStore.environments.map(env => ({
  value: `${env.provider || 'claude'}::${env.name}`,
  label: `${budgetProviderName(env.provider || 'claude')} / ${env.name}`,
})))

function envValue(row: Row) {
  return row.provider && row.env_name ? `${row.provider}::${row.env_name}` : ''
}

function setEnv(row: Row, value: unknown) {
  const [provider, ...rest] = String(value || '').split('::')
  row.provider = provider
  row.env_name = rest.join('::')
}

function toRow(rule: BudgetRule): Row {
  return {
    key: ++seq,
    id: rule.id,
    scope: rule.scope,
    provider: rule.provider || '',
    env_name: rule.env_name || '',
    period: rule.period,
    limit: rule.limit,
    enabled: rule.enabled,
  }
}

function addRow() {
  rows.value.push({ key: ++seq, id: '', scope: 'all', provider: '', env_name: '', period: 'month', limit: '', enabled: true })
}

watch(isOpen, (open) => {
  if (!open) return
  rows.value = props.settings.rules.map(toRow)
  warnPercent.value = props.settings.warn_percent || 80
})

async function save() {
  saving.value = true
  try {
    await budgetService.save({
      warn_percent: Number(warnPercent.value) || 80,
      rules: rows.value.map(row => ({
        id: row.id,
        scope: row.scope,
        provider: row.scope === 'all' ? '' : row.provider,
        env_name: row.scope === 'env' ? row.env_name : '',
        period: row.period,
        limit: Number(row.limit),
        enabled: row.enabled,
      })),
    })
    toast.success(t('budget.saved'))
    isOpen.value = false
    emit('saved')
  } catch (e) {
    toast.error(t('budget.saveFailed', { error: e instanceof Error ? e.message : String(e) }))
  } finally {
    saving.value = false
  }
}
</script>
