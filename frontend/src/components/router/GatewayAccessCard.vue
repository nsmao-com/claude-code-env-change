<template>
  <Card class="mb-4">
    <CardContent>
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0">
          <Label class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{{ t('router.access.title') }}</Label>
          <p class="mt-1 text-xs leading-relaxed text-muted-foreground">{{ t('router.access.desc') }}</p>
        </div>
        <Switch :checked="info.lan_share" :disabled="toggling" :aria-label="t('router.access.title')" @update:checked="toggleLAN" />
      </div>

      <div v-if="info.lan_share" class="mt-4 space-y-2">
        <p v-if="!info.running" class="rounded-lg bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-400">
          {{ t('router.access.notRunning') }}
        </p>
        <p class="text-[11px] font-medium text-muted-foreground">{{ t('router.access.addresses') }}</p>
        <p v-if="!info.addresses.length" class="text-xs text-muted-foreground">{{ t('router.access.noAddress') }}</p>
        <div v-else class="flex flex-wrap gap-2">
          <button
            v-for="addr in info.addresses"
            :key="addr"
            type="button"
            class="inline-flex items-center gap-1.5 rounded-md bg-muted px-2 py-1 font-mono text-xs hover:bg-accent"
            :title="t('router.connect.copy')"
            @click="copy(`http://${addr}:${info.port}`)"
          >
            http://{{ addr }}:{{ info.port }}
            <Copy class="size-3" aria-hidden="true" />
          </button>
        </div>
        <p class="text-[11px] text-muted-foreground">{{ t('router.access.firewall') }}</p>
      </div>

      <div v-if="info.lan_share || info.keys.length" class="mt-5 border-t pt-4">
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="text-sm font-medium">{{ t('router.access.keys') }}</p>
            <p class="mt-0.5 text-[11px] text-muted-foreground">{{ t('router.access.keysDesc') }}</p>
          </div>
          <Button v-if="!adding" variant="outline" size="sm" type="button" @click="startAdd">
            <Plus />{{ t('router.access.add') }}
          </Button>
        </div>
        <form v-if="adding" class="mt-3 flex flex-wrap items-center gap-2" @submit.prevent="createKey">
          <Input
            ref="nameInput"
            v-model="newName"
            class="h-8 w-56"
            maxlength="40"
            :placeholder="t('router.access.namePlaceholder')"
            :aria-label="t('router.access.namePlaceholder')"
          />
          <Button size="sm" type="submit" :disabled="busy || !newName.trim()">{{ t('router.access.create') }}</Button>
          <Button size="sm" variant="ghost" type="button" @click="adding = false">{{ t('router.access.cancel') }}</Button>
        </form>

        <p v-if="!info.keys.length && !adding" class="mt-3 text-xs text-muted-foreground">{{ t('router.access.noKeys') }}</p>
        <ul v-else class="mt-3 divide-y rounded-lg border">
          <li v-for="k in info.keys" :key="k.id" class="px-3 py-3">
            <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
              <div class="min-w-0 flex-1">
                <p class="flex items-center gap-2 text-sm font-medium">
                  {{ k.name }}
                  <Badge v-if="!k.enabled" variant="secondary">{{ t('router.access.disabled') }}</Badge>
                  <Badge v-else-if="k.usage.exceeded" class="border-transparent bg-red-500/10 text-red-600">{{ t('router.access.exceeded') }}</Badge>
                </p>
                <p class="mt-0.5 font-mono text-[11px] text-muted-foreground">
                  {{ revealed[k.id] ? k.key : maskKey(k.key) }}
                </p>
                <p class="mt-0.5 text-[11px] text-muted-foreground">{{ usageText(k) }}</p>
              </div>
              <div class="flex items-center gap-1">
                <AppTooltip :content="revealed[k.id] ? t('router.access.hide') : t('router.access.show')">
                  <Button variant="ghost" size="icon-sm" type="button" :aria-label="revealed[k.id] ? t('router.access.hide') : t('router.access.show')" @click="revealed[k.id] = !revealed[k.id]">
                    <EyeOff v-if="revealed[k.id]" /><Eye v-else />
                  </Button>
                </AppTooltip>
                <AppTooltip :content="t('router.access.copy')">
                  <Button variant="ghost" size="icon-sm" type="button" :aria-label="t('router.access.copy')" @click="copy(k.key)">
                    <Copy />
                  </Button>
                </AppTooltip>
                <Switch size="sm" :checked="k.enabled" :disabled="busy" :aria-label="t('router.access.enabled')" @update:checked="(v: boolean) => update({ ...k, enabled: v })" />
              </div>
            </div>
            <div class="mt-2 flex flex-wrap gap-1">
              <Button variant="ghost" size="sm" class="h-7 px-2 text-xs" type="button" @click="toggleEditor(k)">
                {{ t('router.access.settings') }}
              </Button>
              <Button variant="ghost" size="sm" class="h-7 px-2 text-xs" type="button" :disabled="busy" @click="rotate(k)">
                {{ t('router.access.rotate') }}
              </Button>
              <Button variant="ghost" size="sm" class="h-7 px-2 text-xs text-destructive" type="button" :disabled="busy" @click="remove(k)">
                {{ t('router.access.remove') }}
              </Button>
            </div>
            <form v-if="editing?.id === k.id" class="mt-3 space-y-3 rounded-lg bg-muted/50 p-3" @submit.prevent="saveEditor">
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <div class="grid gap-1.5">
                  <Label class="text-xs">{{ t('router.access.period') }}</Label>
                  <Select v-model="periodValue">
                    <SelectTrigger class="h-8 w-full"><SelectValue /></SelectTrigger>
                    <SelectContent position="popper" align="start">
                      <SelectItem value="none">{{ t('router.access.periodNone') }}</SelectItem>
                      <SelectItem value="day">{{ t('router.access.periodDay') }}</SelectItem>
                      <SelectItem value="week">{{ t('router.access.periodWeek') }}</SelectItem>
                      <SelectItem value="month">{{ t('router.access.periodMonth') }}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div class="grid gap-1.5">
                  <Label class="text-xs">{{ t('router.access.limitTokens') }}</Label>
                  <WorkbenchNumberInput v-model="editing.limit_tokens" :min="0" :step="100000" :disabled="periodValue === 'none'" />
                </div>
                <div class="grid gap-1.5">
                  <Label class="text-xs">{{ t('router.access.limitCost') }}</Label>
                  <WorkbenchNumberInput v-model="editing.limit_cost" :min="0" :step="1" :disabled="periodValue === 'none'" />
                </div>
              </div>
              <p class="text-[11px] text-muted-foreground">{{ t('router.access.limitCostHint') }}</p>
              <div v-if="info.routes.length">
                <p class="text-xs font-medium">{{ t('router.access.routes') }}</p>
                <p class="text-[11px] text-muted-foreground">{{ t('router.access.routesAll') }}</p>
                <div class="mt-2 flex flex-wrap gap-3">
                  <label v-for="r in info.routes" :key="r" class="flex items-center gap-1.5 text-xs">
                    <Checkbox :model-value="(editing.routes || []).includes(r)" @update:model-value="toggleRoute(r, $event === true)" />
                    {{ r }}
                  </label>
                </div>
              </div>
              <div class="flex gap-2">
                <Button size="sm" type="submit" :disabled="busy">{{ t('router.access.save') }}</Button>
                <Button size="sm" variant="ghost" type="button" @click="editing = null">{{ t('router.access.cancel') }}</Button>
              </div>
            </form>
          </li>
        </ul>
      </div>
    </CardContent>
  </Card>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref } from 'vue'
import { Copy, Eye, EyeOff, Plus } from '@lucide/vue'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { callService } from '@/services/appBridge'
import type { GatewayAccessInfo, GatewayKey, GatewayKeyView } from '@/types'
import AppTooltip from '@/components/common/AppTooltip.vue'
import WorkbenchNumberInput from '@/components/workbench/WorkbenchNumberInput.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

const emit = defineEmits<{ changed: [info: GatewayAccessInfo] }>()
const { t } = useI18n()
const toast = useToast()
const confirm = useConfirm()
const router = <T,>(name: string, ...args: unknown[]) => callService<T>('RouterService', name, ...args)

const info = ref<GatewayAccessInfo>({ lan_share: false, port: 8790, running: false, addresses: [], keys: [], routes: [] })
const toggling = ref(false)
const busy = ref(false)
const adding = ref(false)
const newName = ref('')
const nameInput = ref<{ $el?: HTMLElement } | null>(null)
const revealed = reactive<Record<string, boolean>>({})
type KeyForm = GatewayKey & { limit_tokens: number; limit_cost: number }
const editing = ref<KeyForm | null>(null)

const periodValue = computed({
  get: () => editing.value?.limit_period || 'none',
  set: (v: string) => {
    if (!editing.value) return
    editing.value.limit_period = v === 'none' ? '' : (v as GatewayKey['limit_period'])
    if (v === 'none') {
      editing.value.limit_tokens = 0
      editing.value.limit_cost = 0
    }
  },
})

async function load() {
  try {
    info.value = await router<GatewayAccessInfo>('GetGatewayAccess')
    emit('changed', info.value)
  } catch (e) {
    toast.error(t('router.access.opFailed', { error: e instanceof Error ? e.message : String(e) }))
  }
}
void load()
defineExpose({ load })

function errText(e: unknown) {
  return t('router.access.opFailed', { error: e instanceof Error ? e.message : String(e) })
}

async function toggleLAN(on: boolean) {
  if (toggling.value) return
  toggling.value = true
  try {
    await router('SetLANShare', on)
    toast.success(on ? t('router.access.on') : t('router.access.off'))
    await load()
  } catch (e) {
    toast.error(t('router.access.toggleFailed', { error: e instanceof Error ? e.message : String(e) }))
  } finally {
    toggling.value = false
  }
}

async function startAdd() {
  adding.value = true
  newName.value = ''
  await nextTick()
  nameInput.value?.$el?.focus?.()
}

async function createKey() {
  const name = newName.value.trim()
  if (!name || busy.value) return
  busy.value = true
  try {
    const key = await router<GatewayKey>('AddGatewayKey', name)
    adding.value = false
    revealed[key.id] = true
    toast.success(t('router.access.created', { name }))
    await load()
  } catch (e) {
    toast.error(errText(e))
  } finally {
    busy.value = false
  }
}

async function update(k: GatewayKey) {
  if (busy.value) return
  busy.value = true
  try {
    const { id, name, key, enabled, created_at, routes, limit_period, limit_tokens, limit_cost } = k
    await router('UpdateGatewayKey', { id, name, key, enabled, created_at, routes: routes || [], limit_period: limit_period || '', limit_tokens: limit_tokens || 0, limit_cost: limit_cost || 0 })
    toast.success(t('router.access.saved'))
    await load()
  } catch (e) {
    toast.error(errText(e))
  } finally {
    busy.value = false
  }
}

function toggleEditor(k: GatewayKeyView) {
  if (editing.value?.id === k.id) {
    editing.value = null
    return
  }
  const { usage: _usage, ...plain } = k
  editing.value = { ...plain, routes: [...(k.routes || [])], limit_tokens: k.limit_tokens || 0, limit_cost: k.limit_cost || 0 }
}

function toggleRoute(route: string, on: boolean) {
  if (!editing.value) return
  const set = new Set(editing.value.routes || [])
  if (on) set.add(route)
  else set.delete(route)
  editing.value.routes = [...set]
}

async function saveEditor() {
  if (!editing.value) return
  await update(editing.value)
  editing.value = null
}

async function rotate(k: GatewayKeyView) {
  if (!(await confirm.show(t('router.access.rotateTitle'), t('router.access.rotateMsg', { name: k.name }), 'warning'))) return
  busy.value = true
  try {
    await router('RotateGatewayKey', k.id)
    revealed[k.id] = true
    toast.success(t('router.access.rotated'))
    await load()
  } catch (e) {
    toast.error(errText(e))
  } finally {
    busy.value = false
  }
}

async function remove(k: GatewayKeyView) {
  if (!(await confirm.show(t('router.access.removeTitle'), t('router.access.removeMsg', { name: k.name }), 'danger'))) return
  busy.value = true
  try {
    await router('DeleteGatewayKey', k.id)
    toast.success(t('router.access.removed'))
    await load()
  } catch (e) {
    toast.error(errText(e))
  } finally {
    busy.value = false
  }
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(t('router.access.copied'))
  } catch (e) {
    toast.error(errText(e))
  }
}

function maskKey(key: string) {
  return key.length > 16 ? `${key.slice(0, 12)}••••••••${key.slice(-4)}` : '••••••••'
}

function compact(n: number) {
  if (n >= 1e9) return `${(n / 1e9).toFixed(1)}B`
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(1)}K`
  return String(n)
}

function usageText(k: GatewayKeyView) {
  const period = k.limit_period === 'day' ? t('router.access.today') : k.limit_period === 'week' ? t('router.access.thisWeek') : t('router.access.thisMonth')
  let text = t('router.access.usage', { period, tokens: compact(k.usage.tokens), requests: k.usage.requests })
  if (k.usage.cost > 0) text += t('router.access.usageCost', { cost: k.usage.cost.toFixed(4) })
  const limits: string[] = []
  if (k.limit_tokens) limits.push(`${compact(k.limit_tokens)} Token`)
  if (k.limit_cost) limits.push(`$${k.limit_cost}`)
  if (limits.length) text += ` · ${t('router.access.usageLimit', { limit: limits.join(' / ') })}`
  if (k.limit_period && k.usage.resets_at) {
    text += ` · ${t('router.access.resets', { time: new Date(k.usage.resets_at).toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) })}`
  }
  return text
}
</script>
