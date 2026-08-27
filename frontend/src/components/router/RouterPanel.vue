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
      <p class="mt-2 text-sm text-muted-foreground">本地协议转换网关：Anthropic ↔ OpenAI（含 Codex Responses API），跨工具复用 API</p>
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
                @change="saveGatewaySettings"
              />
            </div>
            <div class="flex items-center gap-2">
              <Switch :checked="autoStartInput" @update:checked="onAutoStartChange" />
              <Label class="cursor-pointer text-xs font-bold uppercase tracking-wide text-muted-foreground">随应用启动</Label>
            </div>
          </div>
          <div class="flex items-center">
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

    <div class="mb-4 flex items-center justify-between">
      <Button size="sm" @click="openAdd">
        <Plus />
        添加路由
      </Button>
      <span class="text-xs text-muted-foreground">共 {{ routerStore.config.routes.length }} 条路由</span>
    </div>

    <Empty v-if="routerStore.config.routes.length === 0" class="py-10">
      <EmptyHeader>
        <EmptyTitle>暂无路由</EmptyTitle>
        <EmptyDescription>添加路由后，即可让 Claude Code 使用 OpenAI 兼容接口、Codex 使用 Claude 接口</EmptyDescription>
      </EmptyHeader>
      <EmptyContent>
        <Button size="sm" @click="openAdd">
          <Plus />
          添加路由
        </Button>
      </EmptyContent>
    </Empty>

    <ScrollArea v-else class="h-[38vh] pr-2">
      <div class="space-y-3">
        <Card v-for="route in routerStore.config.routes" :key="route.name" size="sm">
          <CardHeader>
            <div class="flex items-start justify-between gap-4">
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <CardTitle>{{ route.name }}</CardTitle>
                  <Badge :variant="route.enabled ? 'default' : 'secondary'">
                    {{ route.enabled ? '启用' : '停用' }}
                  </Badge>
                  <Badge variant="outline" class="font-mono">
                    {{ directionLabel(route) }}
                  </Badge>
                </div>
                <p v-if="route.description" class="mt-1 text-xs text-muted-foreground">{{ route.description }}</p>

                <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-muted-foreground">
                  <AppTooltip :content="route.base_url" wrap>
                    <span class="flex max-w-[280px] items-center truncate font-mono">
                      <Cloud class="mr-1 size-3 opacity-60" />{{ route.base_url }}
                    </span>
                  </AppTooltip>
                  <span v-if="route.default_model" class="flex items-center font-mono">
                    <Cpu class="mr-1 size-3 opacity-60" />{{ route.default_model }}
                  </span>
                  <span v-if="statsOf(route.name)" class="flex items-center">
                    <Zap class="mr-1 size-3 opacity-60" />
                    {{ statsOf(route.name)!.total_requests }} 次请求
                    <template v-if="statsOf(route.name)!.failed_requests > 0">
                      / <span class="text-red-500">{{ statsOf(route.name)!.failed_requests }} 失败</span>
                    </template>
                  </span>
                  <AppTooltip
                    v-if="statsOf(route.name)?.last_request_at"
                    :content="new Date(statsOf(route.name)!.last_request_at!).toLocaleString()"
                  >
                    <span class="flex items-center">
                      <Clock class="mr-1 size-3 opacity-60" />{{ formatLastRequest(statsOf(route.name)!.last_request_at!) }}
                    </span>
                  </AppTooltip>
                </div>

                <AppTooltip
                  v-if="statsOf(route.name)?.last_error"
                  :content="statsOf(route.name)!.last_error"
                  wrap
                >
                  <div class="mt-2 flex max-w-[480px] items-center truncate text-[11px] text-red-500">
                    <TriangleAlert class="mr-1 size-3" />{{ statsOf(route.name)!.last_error }}
                  </div>
                </AppTooltip>
              </div>

              <div class="flex shrink-0 items-center">
                <Button variant="outline" size="icon-sm" title="复制接入 URL" @click="copyRouteUrl(route)">
                  <Copy />
                </Button>
                <Button
                  variant="outline"
                  size="icon-sm"
                  title="测试上游连通性"
                  :disabled="testingRoute === route.name"
                  @click="testRoute(route)"
                >
                  <Loader2 v-if="testingRoute === route.name" class="animate-spin" />
                  <Plug v-else />
                </Button>
                <Button
                  :variant="route.enabled ? 'outline' : 'default'"
                  size="icon-sm"
                  :title="route.enabled ? '停用' : '启用'"
                  @click="toggleRoute(route)"
                >
                  <Power />
                </Button>
                <Button variant="outline" size="icon-sm" @click="openEdit(route)">
                  <Pencil />
                </Button>
                <Button variant="destructive" size="icon-sm" @click="removeRoute(route)">
                  <Trash2 />
                </Button>
              </div>
            </div>
          </CardHeader>
        </Card>
      </div>
    </ScrollArea>

    <div class="mt-4">
      <div class="mb-2 flex items-center justify-between">
        <span class="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">最近请求</span>
        <div class="flex items-center">
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
              <TableCell class="max-w-[180px] truncate text-muted-foreground" :title="log.path">{{ log.path }}</TableCell>
              <TableCell class="max-w-[140px] truncate text-muted-foreground">{{ log.model }}</TableCell>
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

    <RouteEditModal v-model="showEditModal" :edit-route="editingRoute" @saved="onSaved" />
    <RouterLogsModal v-model="showLogsModal" />
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import {
  Clock,
  Cloud,
  Copy,
  Cpu,
  Loader2,
  Pencil,
  Play,
  Plug,
  Plus,
  Power,
  RefreshCw,
  Square,
  Trash2,
  TriangleAlert,
  Zap,
} from '@lucide/vue'
import type { APIRoute } from '@/types'
import { useRouterStore } from '@/stores/routerStore'
import { routerService } from '@/services/routerService'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'
import RouteEditModal from './RouteEditModal.vue'
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
const confirm = useConfirm()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isRunning = computed(() => routerStore.status?.running ?? false)
const port = computed(() => routerStore.status?.port ?? routerStore.config.port)

const portInput = ref(8790)
const autoStartInput = ref(true)
const showEditModal = ref(false)
const showLogsModal = ref(false)
const editingRoute = ref<APIRoute | null>(null)
const testingRoute = ref<string | null>(null)

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
    showEditModal.value = false
    showLogsModal.value = false
  }
})

onUnmounted(() => {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
})

const recentLogs = computed(() => (routerStore.status?.logs ?? []).slice(-10).reverse())

function onPortUpdate(value: string | number) {
  portInput.value = Number(value)
}

function onAutoStartChange(checked: boolean) {
  autoStartInput.value = checked
  void saveGatewaySettings()
}

function shortTime(value: string): string {
  if (!value) return ''
  const parts = value.trim().split(' ')
  return parts.length > 1 ? parts[parts.length - 1] : value
}

function statsOf(name: string) {
  return routerStore.status?.stats?.[name]
}

function formatLastRequest(ts: number): string {
  const diff = Date.now() - ts
  if (diff < 60_000) return '刚刚'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`
  return new Date(ts).toLocaleDateString()
}

function directionLabel(route: APIRoute): string {
  const source = route.source_format === 'anthropic' ? 'Anthropic' : 'OpenAI'
  const target = route.target_format === 'anthropic' ? 'Anthropic' : 'OpenAI'
  if (route.source_format === route.target_format) {
    return `${source} 直连`
  }
  return route.source_format === 'anthropic' ? `${source} → ${target}` : `${target} ← ${source}`
}

function routeUrlOf(route: APIRoute): string {
  const base = `http://127.0.0.1:${port.value}/${route.name}`
  return route.source_format === 'openai' ? `${base}/v1` : base
}

async function saveGatewaySettings() {
  const p = Number(portInput.value)
  if (!p || p < 1 || p > 65535) {
    toast.error('端口必须在 1-65535 之间')
    portInput.value = routerStore.config.port
    return
  }
  try {
    await routerStore.saveConfig({
      ...routerStore.config,
      port: p,
      auto_start: autoStartInput.value
    })
    toast.success('网关设置已保存' + (isRunning.value ? '，已重启生效' : ''))
  } catch (e: any) {
    toast.error('保存失败: ' + (e?.message || String(e)))
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

function openAdd() {
  editingRoute.value = null
  showEditModal.value = true
}

function openEdit(route: APIRoute) {
  editingRoute.value = JSON.parse(JSON.stringify(route))
  showEditModal.value = true
}

async function toggleRoute(route: APIRoute) {
  const routes = routerStore.config.routes.map((r) =>
    r.name === route.name ? { ...r, enabled: !r.enabled } : r
  )
  try {
    await routerStore.saveConfig({ ...routerStore.config, routes })
    toast.success(route.enabled ? `路由 ${route.name} 已停用` : `路由 ${route.name} 已启用`)
  } catch (e: any) {
    toast.error('操作失败: ' + (e?.message || String(e)))
  }
}

async function removeRoute(route: APIRoute) {
  const ok = await confirm.show('删除路由', `确定要删除路由 "${route.name}" 吗？`, 'danger')
  if (!ok) return
  const routes = routerStore.config.routes.filter((r) => r.name !== route.name)
  try {
    await routerStore.saveConfig({ ...routerStore.config, routes })
    toast.success('路由已删除')
  } catch (e: any) {
    toast.error('删除失败: ' + (e?.message || String(e)))
  }
}

async function testRoute(route: APIRoute) {
  testingRoute.value = route.name
  try {
    const result = await routerService.testRoute(route.name)
    if (result.success) {
      toast.success(`${route.name}: ${result.message} (${result.latency}ms)`)
    } else {
      toast.error(`${route.name}: ${result.message}`)
    }
  } catch (e: any) {
    toast.error('测试失败: ' + (e?.message || String(e)))
  } finally {
    testingRoute.value = null
  }
}

async function copyRouteUrl(route: APIRoute) {
  const url = routeUrlOf(route)
  try {
    await navigator.clipboard.writeText(url)
    toast.success(`已复制: ${url}`)
  } catch {
    toast.info(url)
  }
}

function onSaved() {
  routerStore.refreshStatus().catch(() => {})
}
</script>
