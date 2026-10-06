<script setup lang="ts">
import { Button } from '@/components/ui/button'
import WorkbenchNumberInput from './WorkbenchNumberInput.vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ref, reactive } from 'vue'
import { useWorkbench, workbench } from '@/composables/useWorkbench'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'
import type { WorkbenchConfig, CostSettings, CostOverview } from '@/types/workbench'
const { tx, busy, error, run } = useWorkbench(),
  config = useConfigStore(),
  toast = useToast()
const costs = reactive<CostSettings>({
  daily_budget: 0,
  monthly_budget: 0,
  multipliers: {},
  balance_sources: [],
})
const overview = ref<CostOverview | null>(null)
const ledgerMonths = ref<string[]>([])
const ledgerMonth = ref('')
const exporting = ref(false)
async function refresh() {
  overview.value = await workbench('GetCostOverview')
  if (overview.value?.daily_exceeded || overview.value?.monthly_exceeded)
    toast.info(tx('用量已达到设置的预算，请检查费用。', 'Usage has reached your budget. Review costs.'))
}
async function save() {
  await run(
    async () => {
      await workbench('SaveCostSettings', { ...costs })
      await refresh()
    },
    tx('预算与倍率已保存', 'Budgets and multipliers saved'),
  )
}
async function exportLedger() {
  if (exporting.value || !ledgerMonth.value) return
  exporting.value = true
  try {
    const path = await workbench<string>('ExportGatewayLedger', ledgerMonth.value)
    if (path) toast.success(tx(`已导出到 ${path}`, `Exported to ${path}`))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    exporting.value = false
  }
}
void run(async () => {
  await config.loadConfig()
  const c = await workbench<WorkbenchConfig>('GetWorkbench')
  Object.assign(costs, c.costs)
  await refresh()
  ledgerMonths.value = (await workbench<string[]>('ListGatewayLedgerMonths')) || []
  ledgerMonth.value = ledgerMonths.value[0] || ''
})
</script>
<template>
  <p v-if="error" role="alert" class="wb-error">{{ error }}</p>
  <section class="wb-card">
    <div class="wb-row justify-between">
      <h2>{{ tx('网关费用', 'Gateway costs') }}</h2>
      <Button variant="outline" type="button" :disabled="busy" @click="run(refresh)">
        {{ tx('刷新', 'Refresh') }}
      </Button>
    </div>
    <p class="wb-hint">
      {{
        tx(
          '依据上游实际返回的用量与模型档案中的美元价格估算，仅统计本网关收到有效用量的请求，未上报用量的请求不计入。CLI 日志统计请查看「统计」页，两者不相加。',
          'Estimates USD costs from upstream-reported usage and model profile prices for gateway requests reporting usage only. Requests without usage are excluded. CLI log statistics are separate on the Statistics page.',
        )
      }}
    </p>
    <div class="wb-grid" v-if="overview">
      <div>
        <p class="text-xs text-muted-foreground">{{ tx('今日 / USD', 'Today / USD') }}</p>
        <p class="text-3xl font-semibold mt-2">{{ overview.today.toFixed(4) }}</p>
      </div>
      <div>
        <p class="text-xs text-muted-foreground">{{ tx('本月 / USD', 'This month / USD') }}</p>
        <p class="text-3xl font-semibold mt-2">{{ overview.month.toFixed(4) }}</p>
      </div>
      <div>
        <p class="text-xs text-muted-foreground">
          {{ tx('本月请求 / 未定价', 'Monthly requests / unpriced') }}
        </p>
        <p class="text-3xl font-semibold mt-2">{{ overview.requests }} / {{ overview.unpriced }}</p>
      </div>
    </div>
    <p v-if="overview?.unpriced" class="wb-hint mt-4">
      {{
        tx(
          '部分模型缺少唯一匹配的定价档案，未计入费用。请在模型页保存对应环境和模型的价格。',
          'Some models have no unique matching pricing profile and are excluded from costs. Add prices in Models.',
        )
      }}
      <template v-if="overview.list_estimate > 0">
        {{
          tx(
            `按官方公开价目参考估算约 $${overview.list_estimate.toFixed(4)}（中转站实际价格可能不同，不计入预算）。`,
            `At official list prices they come to about $${overview.list_estimate.toFixed(4)} (relays may charge differently; not counted toward budgets).`,
          )
        }}
      </template>
    </p>
    <p v-if="overview?.daily_exceeded || overview?.monthly_exceeded" role="alert" class="wb-error mt-4">
      {{
        tx(
          '已达到预算提醒阈值。提醒不会阻止请求。',
          'Budget threshold reached. Alerts do not block requests.',
        )
      }}
    </p>
  </section>
  <section class="wb-card">
    <h2>{{ tx('预算与供应商倍率', 'Budgets & provider multipliers') }}</h2>
    <form @submit.prevent="save">
      <div class="wb-grid">
        <label
          >{{ tx('每日预算 / USD，0 关闭', 'Daily budget / USD, 0 disables')
          }}
          <WorkbenchNumberInput :disabled="busy" v-model="costs.daily_budget" :min="0" :step="0.01" /></label
        ><label
          >{{ tx('每月预算 / USD，0 关闭', 'Monthly budget / USD, 0 disables')
          }}
          <WorkbenchNumberInput :disabled="busy" v-model="costs.monthly_budget" :min="0" :step="0.01" /></label>
      </div>
      <div class="wb-grid mt-4">
        <label
          v-for="e in config.environments.filter((e) => !e.official_login)"
          :key="`${e.provider}/${e.name}`"
          >{{ e.provider }} · {{ e.name
          }}
          <WorkbenchNumberInput
            :disabled="busy"
            :min="0"
            :max="1000"
            :step="0.01"
            :model-value="costs.multipliers[`${e.provider}/${e.name}`] ?? 1"
            @update:model-value="costs.multipliers[`${e.provider}/${e.name}`] = $event"
          /></label>
      </div>
      <Button variant="default" type="submit" class="mt-4" :disabled="busy">
        {{ tx('保存预算与倍率', 'Save budgets & multipliers') }}
      </Button>
    </form>
  </section>
  <section class="wb-card">
    <h2>{{ tx('导出请求账本', 'Export request ledger') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '把网关每条有用量的请求导出为 CSV（含 Token、首 Token 延迟、估算费用、调用方网关密钥），可用 Excel 打开对账。保留当月与前两个月。',
          'Exports every gateway request with usage as CSV (tokens, first-token latency, estimated cost, caller gateway key) for reconciling in Excel. The current and previous two months are kept.',
        )
      }}
    </p>
    <div class="wb-row !mb-0">
      <Select v-model="ledgerMonth" :disabled="exporting">
        <SelectTrigger class="h-9 w-40 min-w-0" :aria-label="tx('月份', 'Month')">
          <SelectValue />
        </SelectTrigger>
        <SelectContent position="popper" align="start">
          <SelectItem v-for="m in ledgerMonths" :key="m" :value="m">{{ m }}</SelectItem>
        </SelectContent>
      </Select>
      <Button variant="outline" type="button" :disabled="exporting || !ledgerMonth" @click="exportLedger">
        {{ exporting ? tx('导出中…', 'Exporting…') : tx('导出 CSV', 'Export CSV') }}
      </Button>
    </div>
  </section>
  <p class="wb-hint">
    {{
      tx(
        '供应商余额与订阅额度已移到「额度与余额」页签，支持更多供应商、中转站与低余额提醒。',
        'Provider balances and subscription allowances now live in the “Allowances & balances” tab, with more vendors, relays and low-balance alerts.',
      )
    }}
  </p>
</template>
