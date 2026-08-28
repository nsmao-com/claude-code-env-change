<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" :close-on-overlay="false">
    <template #header>
      <div class="flex items-center gap-3">
        <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">路由</h1>
        <Badge
          :class="isRunning
            ? 'border-transparent bg-green-500/10 text-green-600 uppercase'
            : 'border-transparent bg-red-500/10 text-red-600 uppercase'"
        >
          {{ isRunning ? `运行中 :${port}` : '已停止' }}
        </Badge>
      </div>
      <p class="mt-2 text-sm text-muted-foreground">这里只开关本机网关。上游协议在「配置」里选；打开对应模型商的开关后，才会把 CLI 指到本机这个端口。</p>
    </template>

    <Card class="mb-4">
      <CardContent>
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div class="flex flex-wrap items-center gap-4">
            <div class="flex items-center gap-2">
              <Label class="text-xs font-bold uppercase tracking-wide text-muted-foreground">端口</Label>
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
              <Label class="cursor-pointer text-xs font-bold uppercase tracking-wide text-muted-foreground">随应用启动</Label>
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
              停止网关
            </Button>
            <Button v-else size="sm" :disabled="routerStore.isToggling" @click="startGateway">
              <Loader2 v-if="routerStore.isToggling" class="animate-spin" />
              <Play v-else />
              启动网关
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card class="mb-4">
      <CardContent>
        <div class="mb-3">
          <Label class="text-xs font-bold uppercase tracking-wide text-muted-foreground">应用路由</Label>
          <p class="mt-1 text-xs leading-relaxed text-muted-foreground">
            每个模型商单独开关，共用上面的端口。开启后把该 CLI 指到本机网关；关闭则写回配置里的原地址。要转换协议，请先在配置里把上游格式改成非原生。
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
            <Switch
              size="sm"
              :checked="routerStore.isAppRoutingOn(item.id)"
              :disabled="routerStore.togglingApp === item.id"
              @update:checked="(value: boolean) => onAppRouting(item.id, value)"
            />
          </div>
        </div>
      </CardContent>
    </Card>

    <div class="mt-4">
      <div class="mb-2 flex items-center justify-between gap-2">
        <span class="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">最近请求</span>
        <div class="flex items-center gap-1.5">
          <Button variant="ghost" size="sm" @click="showLogsModal = true">查看全部</Button>
          <Button variant="ghost" size="icon-sm" @click="routerStore.refreshStatus()">
            <RefreshCw />
          </Button>
        </div>
      </div>
      <p v-if="recentLogs.length === 0" class="text-xs text-muted-foreground">暂无请求。网关运行后会在此显示最近 10 条。</p>
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
              <TableCell class="w-16 text-right text-muted-foreground">{{ log.duration_ms }}ms</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </div>

    <RouterLogsModal v-model="showLogsModal" />
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { Loader2, Play, RefreshCw, Square } from '@lucide/vue'
import type { Provider } from '@/types'
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
  { id: 'claude', label: 'Claude' },
  { id: 'codex', label: 'Codex' },
  { id: 'gemini', label: 'Gemini' },
  { id: 'opencode', label: 'OpenCode' },
  { id: 'grok', label: 'Grok' },
]

function onPortUpdate(value: string | number) {
  portInput.value = Number(value)
}

function shortTime(value: string): string {
  if (!value) return ''
  const parts = value.trim().split(' ')
  return parts.length > 1 ? parts[parts.length - 1] : value
}

function appRoutingHint(id: Provider) {
  if (!routerStore.isAppRoutingOn(id)) return '关闭时 CLI 直连配置里的地址'
  return `已转到 127.0.0.1:${port.value}/${id}`
}

async function saveGatewaySettings(kind: 'port' | 'autostart' = 'port') {
  const p = Number(portInput.value)
  if (!p || p < 1 || p > 65535) {
    toast.error('端口必须在 1-65535 之间')
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
      toast.success(autoStartInput.value ? '已开启随应用启动' : '已关闭随应用启动')
    } else {
      toast.success('网关设置已保存' + (isRunning.value ? '，已重启生效' : ''))
    }
  } catch (e: any) {
    toast.error('保存失败: ' + (e?.message || String(e)))
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
    toast.success(enabled ? `已开启 ${label} 路由` : `已关闭 ${label} 路由`)
  } catch (e: any) {
    toast.error(e?.message || String(e))
  }
}

async function startGateway() {
  try {
    await routerStore.start()
    toast.success(`网关已启动，监听 127.0.0.1:${port.value}`)
  } catch (e: any) {
    toast.error('启动失败: ' + (e?.message || String(e)))
  }
}

async function stopGateway() {
  try {
    await routerStore.stop()
    toast.success('网关已停止')
  } catch (e: any) {
    toast.error('停止失败: ' + (e?.message || String(e)))
  }
}
</script>
