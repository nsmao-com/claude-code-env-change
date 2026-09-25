<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
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
const overview = ref<CostOverview | null>(null),
  selected = ref(''),
  adapter = ref('openrouter'),
  balance = ref('')
const env = computed(() => config.environments.find((e) => `${e.provider}/${e.name}` === selected.value))
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
async function check() {
  if (!env.value) return
  await run(async () => {
    const source = { provider: env.value!.provider, environment: env.value!.name, adapter: adapter.value }
    const r = await workbench<{ amount: number; currency: string }>('CheckBalance', source)
    balance.value = `${r.amount.toFixed(4)} ${r.currency}`
    costs.balance_sources = [
      ...costs.balance_sources.filter(
        (b) => b.provider !== source.provider || b.environment !== source.environment,
      ),
      source,
    ]
    await workbench('SaveCostSettings', { ...costs })
  })
}
void run(async () => {
  await config.loadConfig()
  const c = await workbench<WorkbenchConfig>('GetWorkbench')
  Object.assign(costs, c.costs)
  await refresh()
})
</script>
<template>
  <p v-if="error" role="alert" class="wb-error">{{ error }}</p>
  <section class="wb-card">
    <div class="wb-row justify-between">
      <h2>{{ tx('网关费用', 'Gateway costs') }}</h2>
      <button :disabled="busy" @click="run(refresh)">{{ tx('刷新', 'Refresh') }}</button>
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
          }}<input v-model.number="costs.daily_budget" type="number" min="0" step="0.01" /></label
        ><label
          >{{ tx('每月预算 / USD，0 关闭', 'Monthly budget / USD, 0 disables')
          }}<input v-model.number="costs.monthly_budget" type="number" min="0" step="0.01"
        /></label>
      </div>
      <div class="wb-grid mt-4">
        <label
          v-for="e in config.environments.filter((e) => !e.official_login)"
          :key="`${e.provider}/${e.name}`"
          >{{ e.provider }} · {{ e.name
          }}<input
            type="number"
            min="0"
            max="1000"
            step="0.01"
            :value="costs.multipliers[`${e.provider}/${e.name}`] ?? 1"
            @input="
              costs.multipliers[`${e.provider}/${e.name}`] = Number(($event.target as HTMLInputElement).value)
            "
        /></label>
      </div>
      <button class="primary mt-4" :disabled="busy">
        {{ tx('保存预算与倍率', 'Save budgets & multipliers') }}
      </button>
    </form>
  </section>
  <section class="wb-card">
    <h2>{{ tx('查询供应商余额', 'Provider balance') }}</h2>
    <p class="wb-hint">
      {{
        tx(
          '使用已保存的凭证请求供应商余额接口，不运行自定义脚本。Base URL 应包含供应商要求的 API 前缀。',
          'Uses saved credentials to call the provider balance API. No custom scripts are executed. Include the provider’s API prefix in Base URL.',
        )
      }}
    </p>
    <div class="wb-grid">
      <label
        >{{ tx('环境', 'Environment')
        }}<select v-model="selected">
          <option value="" disabled>{{ tx('选择环境', 'Select environment') }}</option>
          <option
            v-for="e in config.environments.filter((e) => !e.official_login)"
            :key="`${e.provider}/${e.name}`"
            :value="`${e.provider}/${e.name}`"
          >
            {{ e.provider }} · {{ e.name }}
          </option>
        </select></label
      ><label
        >{{ tx('余额接口', 'Balance API')
        }}<select v-model="adapter">
          <option value="openrouter">OpenRouter</option>
          <option value="deepseek">DeepSeek</option>
        </select></label
      >
    </div>
    <div class="wb-row mt-4">
      <button :disabled="busy || !env" @click="check">{{ tx('查询余额', 'Check balance') }}</button
      ><strong>{{ balance }}</strong>
    </div>
  </section>
</template>
