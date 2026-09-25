<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" width="form" :close-on-overlay="false">
    <template #header>
      <div class="flex items-center gap-3">
        <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">{{ t('nav.router') }}</h1>
        <Badge
          :class="isRunning
            ? 'border-transparent bg-green-500/10 text-green-600 uppercase'
            : 'border-transparent bg-red-500/10 text-red-600 uppercase'"
        >
          {{ isRunning ? t('router.runningOn', { port }) : t('router.stopped') }}
        </Badge>
      </div>
      <p class="mt-2 text-sm text-muted-foreground">{{ t('router.panelHint') }}</p>
    </template>

    <Card class="mb-4">
      <CardContent>
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div class="flex flex-wrap items-center gap-4">
            <div class="flex items-center gap-2">
              <Label class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{{ t('router.port') }}</Label>
              <Input
                :model-value="portInput"
                type="number"
                min="1"
                max="65535"
                class="w-24 font-mono text-xs"
                @update:model-value="onPortUpdate"
                @change="() => saveGatewaySettings()"
              />
            </div>
            <div class="flex items-center gap-2">
              <Switch :checked="autoStartInput" @update:checked="onAutoStartChange" />
              <Label class="cursor-pointer text-xs font-bold uppercase tracking-wide text-muted-foreground">{{ t('router.autoStart') }}</Label>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <Button
              v-if="isRunning"
              variant="destructive"
              size="sm"
              :disabled="routerStore.isToggling"
              @click="stopGateway"
            >
              <Loader2 v-if="routerStore.isToggling" class="animate-spin" />
              <Square v-else />
              {{ t('router.stop') }}
            </Button>
            <Button v-else size="sm" :disabled="routerStore.isToggling" @click="startGateway">
              <Loader2 v-if="routerStore.isToggling" class="animate-spin" />
              <Play v-else />
              {{ t('router.start') }}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card class="mb-4">
      <CardContent>
        <div class="mb-3">
          <Label class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{{ t('router.appRouting') }}</Label>
          <p class="mt-1 text-xs leading-relaxed text-muted-foreground">
            {{ t('router.appRoutingDesc') }}
          </p>
        </div>
        <div class="divide-y">
          <div v-for="item in appProviders" :key="item.id" class="flex items-center justify-between gap-3 py-2.5 first:pt-0 last:pb-0">
            <div class="flex min-w-0 items-center gap-2">
              <BrandIcon :provider="item.id" class="size-3.5 shrink-0" />
              <div class="min-w-0">
                <p class="text-sm font-medium">{{ item.label }}</p>
                <p class="text-[11px] text-muted-foreground">{{ appRoutingHint(item.id) }}</p>
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <AppTooltip :content="fallbackTooltip(item.id)" wrap>
                <Button
                  variant="ghost"
                  size="sm"
                  class="h-7 px-2 text-xs"
                  :disabled="!routeFor(item.id)"
                  @click="openFallbacks(item.id)"
                >
                  {{ t('router.fallbacks') }}{{ fallbackCount(item.id) ? ` · ${fallbackCount(item.id)}` : '' }}
                </Button>
              </AppTooltip>
              <Switch
                size="sm"
                :checked="routerStore.isAppRoutingOn(item.id)"
                :disabled="routerStore.togglingApp === item.id"
                @update:checked="(value: boolean) => onAppRouting(item.id, value)"
              />
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <div class="mt-4">
      <div class="mb-2 flex items-center justify-between gap-2">
        <span class="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">{{ t('router.recent') }}</span>
        <div class="flex items-center gap-1.5">
          <Button variant="ghost" size="sm" @click="showLogsModal = true">{{ t('router.viewAll') }}</Button>
          <Button variant="ghost" size="icon-sm" @click="routerStore.refreshStatus()">
            <RefreshCw />
          </Button>
        </div>
      </div>
      <p v-if="recentLogs.length === 0" class="text-xs text-muted-foreground">{{ t('router.noRequests') }}</p>
      <div v-else class="overflow-hidden rounded-lg border">
        <Table class="font-mono text-[11px]">
          <TableBody>
            <TableRow v-for="(log, i) in recentLogs" :key="i">
              <TableCell class="w-20 text-muted-foreground">{{ shortTime(log.time) }}</TableCell>
              <TableCell class="font-bold">{{ log.route }}</TableCell>
              <TableCell class="max-w-[180px] truncate text-muted-foreground">
                <AppTooltip :content="log.path" wrap :disabled="!log.path">
                  <span class="block truncate">{{ log.path }}</span>
                </AppTooltip>
              </TableCell>
              <TableCell class="max-w-[140px] truncate text-muted-foreground">
                <AppTooltip :content="log.model" wrap :disabled="!log.model">
                  <span class="block truncate">{{ log.model }}</span>
                </AppTooltip>
              </TableCell>
              <TableCell
                class="w-14"
                :class="log.status_code >= 400 ? 'font-bold text-red-500' : 'text-green-600'"
              >
                {{ log.status_code }}
              </TableCell>
              <TableCell class="w-24 text-right text-muted-foreground"><div>{{ log.duration_ms }}ms</div><div v-if="log.first_token_ms" class="text-[10px]">TTFT {{ log.first_token_ms }}ms</div><div v-if="log.usage_reported" class="text-[10px]">{{ log.input_tokens || 0 }} ↑ {{ log.output_tokens || 0 }} ↓</div></TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </div>

    <RouterLogsModal v-model="showLogsModal" />
    <RouteEditModal v-model="showRouteEditor" :edit-route="editingRoute" @saved="routerStore.loadConfig()" />
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { ref, computed, watch, onUnmounted } from 'vue'
import { Loader2, Play, RefreshCw, Square } from '@lucide/vue'
import type { APIRoute, Provider } from '@/types'
import { useRouterStore } from '@/stores/routerStore'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'
import RouterLogsModal from './RouterLogsModal.vue'
import RouteEditModal from './RouteEditModal.vue'

const { t } = useI18n()

interface Props {
  modelValue: boolean
  embedded?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const routerStore = useRouterStore()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isRunning = computed(() => routerStore.status?.running ?? false)
const port = computed(() => routerStore.status?.port ?? routerStore.config.port)

const portInput = ref(8790)
const autoStartInput = ref(true)
const showLogsModal = ref(false)
const showRouteEditor = ref(false)
const editingRoute = ref<APIRoute | null>(null)

// 应用路由按模型商 id 命名；开启路由并应用过一次配置后才会生成
function routeFor(provider: Provider): APIRoute | undefined {
  return routerStore.config.routes.find(route => route.name.toLowerCase() === provider)
}

function fallbackCount(provider: Provider): number {
  return routeFor(provider)?.fallbacks?.length || 0
}

function fallbackTooltip(provider: Provider): string {
  if (!routeFor(provider)) return t('router.fallbackNeedsRoute')
  const count = fallbackCount(provider)
  return count
    ? t('router.fallbackConfigured', { count })
    : t('router.fallbackAdd')
}

function openFallbacks(provider: Provider) {
  const route = routeFor(provider)
  if (!route) return
  editingRoute.value = route
  showRouteEditor.value = true
}

let pollTimer: number | null = null

watch(isOpen, async (open) => {
  if (open) {
    await routerStore.loadConfig()
    await routerStore.refreshStatus()
    portInput.value = routerStore.config.port || 8790
    autoStartInput.value = routerStore.config.auto_start !== false
    pollTimer = window.setInterval(() => routerStore.refreshStatus().catch(() => {}), 5000)
  } else {
    if (pollTimer) {
      window.clearInterval(pollTimer)
      pollTimer = null
    }
    showLogsModal.value = false
  }}, { immediate: true })

onUnmounted(() => {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
})

const recentLogs = computed(() => (routerStore.status?.logs ?? []).slice(-10).reverse())

const appProviders: { id: Provider; label: string }[] = [
  { id: 'claude', label: 'Claude Code' },
  { id: 'claude_desktop', label: 'Claude Desktop' },
  { id: 'codex', label: 'Codex' },
  { id: 'antigravity', label: 'Antigravity' },
  { id: 'opencode', label: 'OpenCode' },
  { id: 'grok', label: 'Grok' },
]

function onPortUpdate(value: string | number) {
  const raw = String(value).trim()
  // 清空输入的过程中保持空串，否则 Number('')===0 会立刻回写，用户被迫全选重输
  if (raw === '') {
    portInput.value = '' as unknown as number
    return
  }
  const n = Number(raw)
  if (!Number.isNaN(n)) portInput.value = n
}

function shortTime(value: string): string {
  if (!value) return ''
  const parts = value.trim().split(' ')
  return parts.length > 1 ? parts[parts.length - 1] : value
}

function appRoutingHint(id: Provider) {
  if (!routerStore.isAppRoutingOn(id)) return t('router.directHint')
  return t('router.routedTo', { port: port.value, id })
}

async function saveGatewaySettings(kind: 'port' | 'autostart' = 'port') {
  const p = Number(portInput.value)
  if (!p || p < 1 || p > 65535) {
    toast.error(t('router.portInvalid'))
    portInput.value = routerStore.config.port
    if (kind === 'autostart') throw new Error('invalid port')
    return
  }
  try {
    await routerStore.saveConfig({
      ...routerStore.config,
      port: p,
      auto_start: autoStartInput.value
    })
    try {
      await routerStore.refreshRoutedProviders()
    } catch {
      /* 没有已开启的应用路由时忽略 */
    }
    if (kind === 'autostart') {
      toast.success(autoStartInput.value ? t('router.autoStartOn') : t('router.autoStartOff'))
    } else {
      toast.success(isRunning.value ? t('router.settingsSavedRestarted') : t('router.settingsSaved'))
    }
  } catch (e: any) {
    toast.error(t('router.saveFailed', { error: e?.message || String(e) }))
    if (kind === 'autostart') throw e
  }
}

async function onAutoStartChange(checked: boolean) {
  const prev = autoStartInput.value
  autoStartInput.value = checked
  try {
    await saveGatewaySettings('autostart')
  } catch {
    autoStartInput.value = prev
  }
}

async function onAppRouting(provider: Provider, enabled: boolean) {
  try {
    await routerStore.setAppRouting(provider, enabled)
    const label = appProviders.find(item => item.id === provider)?.label || provider
    toast.success(enabled ? t('router.routingOn', { name: label }) : t('router.routingOff', { name: label }))
  } catch (e: any) {
    toast.error(e?.message || String(e))
  }
}

async function startGateway() {
  try {
    await routerStore.start()
    toast.success(t('router.started', { port: port.value }))
  } catch (e: any) {
    toast.error(t('router.startFailed', { error: e?.message || String(e) }))
  }
}

async function stopGateway() {
  try {
    await routerStore.stop()
    toast.success(t('router.stoppedToast'))
  } catch (e: any) {
    toast.error(t('router.stopFailed', { error: e?.message || String(e) }))
  }
}
</script>
