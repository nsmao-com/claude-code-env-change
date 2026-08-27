<template>
  <AppModal v-model="isOpen" size="xl" :close-on-overlay="false">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="flex size-10 items-center justify-center rounded-lg bg-primary/10">
          <List class="size-4 text-primary" />
        </div>
        <div>
          <DialogTitle class="text-lg font-semibold">请求日志</DialogTitle>
          <p class="text-xs text-muted-foreground">保留最近 1000 条，便于排查协议转换与上游错误</p>
        </div>
      </div>
    </template>

    <div class="mb-4 flex flex-wrap items-end">
      <div class="mb-2 mr-3 grid gap-1.5">
        <Label>路由</Label>
        <Select :model-value="routeFilter || '__all__'" @update:model-value="onRouteFilter">
          <SelectTrigger class="w-40 text-xs">
            <SelectValue placeholder="全部" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">全部</SelectItem>
            <SelectItem v-for="name in routeNames" :key="name" :value="name">{{ name }}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="mb-2 mr-3 grid min-w-[180px] flex-1 gap-1.5">
        <Label>关键词</Label>
        <Input
          v-model="keyword"
          class="text-xs"
          placeholder="路径 / 模型 / 错误信息"
          @keyup.enter="reload(true)"
        />
      </div>
      <div class="mb-2 mr-3 flex h-9 items-center gap-2">
        <Switch :checked="onlyErrors" size="sm" @update:checked="onOnlyErrorsChange" />
        <Label class="cursor-pointer text-xs">只看失败</Label>
      </div>
      <Button variant="outline" size="sm" class="mb-2" @click="reload(true)">
        <Loader2 v-if="loading" class="animate-spin" />
        <Search v-else />
        查询
      </Button>
    </div>

    <div class="max-h-[46vh] overflow-auto rounded-lg border">
      <Table class="font-mono text-[11px]">
        <TableHeader class="sticky top-0 bg-card">
          <TableRow>
            <TableHead>时间</TableHead>
            <TableHead>路由</TableHead>
            <TableHead>路径</TableHead>
            <TableHead>模型</TableHead>
            <TableHead>状态</TableHead>
            <TableHead class="text-right">耗时</TableHead>
            <TableHead>错误</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableEmpty v-if="!loading && items.length === 0" :colspan="7" class="text-muted-foreground">
            没有匹配的日志
          </TableEmpty>
          <TableRow v-for="(log, i) in items" :key="i + log.time + log.path">
            <TableCell class="text-muted-foreground">{{ log.time }}</TableCell>
            <TableCell class="font-bold">{{ log.route }}</TableCell>
            <TableCell class="max-w-[220px] truncate text-muted-foreground">
              <AppTooltip :content="log.path" wrap :disabled="!log.path">
                <span class="block truncate">{{ log.path }}</span>
              </AppTooltip>
            </TableCell>
            <TableCell class="max-w-[140px] truncate text-muted-foreground">
              <AppTooltip :content="log.model" wrap :disabled="!log.model">
                <span class="block truncate">{{ log.model }}</span>
              </AppTooltip>
            </TableCell>
            <TableCell :class="log.status_code >= 400 ? 'font-bold text-red-500' : 'text-green-600'">
              {{ log.status_code }}
            </TableCell>
            <TableCell class="text-right text-muted-foreground">{{ log.duration_ms }}ms</TableCell>
            <TableCell class="max-w-[240px] truncate text-red-500">
              <AppTooltip :content="log.error" wrap :disabled="!log.error">
                <span class="block truncate">{{ log.error }}</span>
              </AppTooltip>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <div class="mt-3 flex items-center justify-between text-xs text-muted-foreground">
      <span>共 {{ total }} 条 · 第 {{ page }} / {{ pageCount }} 页</span>
      <div class="flex items-center">
        <Button variant="outline" size="sm" :disabled="offset <= 0" @click="prevPage">上一页</Button>
        <Button variant="outline" size="sm" :disabled="offset + pageSize >= total" @click="nextPage">下一页</Button>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-between">
        <Button type="button" variant="destructive" @click="clearLogs">清空日志</Button>
        <Button type="button" variant="secondary" @click="isOpen = false">关闭</Button>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { List, Loader2, Search } from '@lucide/vue'
import type { RouterLogEntry } from '@/types'
import { useRouterStore } from '@/stores/routerStore'
import { routerService } from '@/services/routerService'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import { Button } from '@/components/ui/button'
import { DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableEmpty, TableHead, TableHeader, TableRow, TableCell } from '@/components/ui/table'

interface Props {
  modelValue: boolean
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

const routeFilter = ref('')
const keyword = ref('')
const onlyErrors = ref(false)
const items = ref<RouterLogEntry[]>([])
const total = ref(0)
const offset = ref(0)
const pageSize = 50
const loading = ref(false)

const routeNames = computed(() => routerStore.config.routes.map((r) => r.name))
const page = computed(() => Math.floor(offset.value / pageSize) + 1)
const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

watch(isOpen, (open) => {
  if (open) {
    offset.value = 0
    reload(true)
  }
})

function onRouteFilter(value: unknown) {
  routeFilter.value = !value || value === '__all__' ? '' : String(value)
  reload(true)
}

function onOnlyErrorsChange(checked: boolean) {
  onlyErrors.value = checked
  reload(true)
}

async function reload(resetOffset: boolean) {
  if (resetOffset) offset.value = 0
  loading.value = true
  try {
    const pageData = await routerService.getLogs({
      route: routeFilter.value,
      keyword: keyword.value,
      only_errors: onlyErrors.value,
      limit: pageSize,
      offset: offset.value
    })
    items.value = pageData.items || []
    total.value = pageData.total || 0
  } catch (e: any) {
    toast.error('加载日志失败: ' + (e?.message || String(e)))
  } finally {
    loading.value = false
  }
}

function prevPage() {
  offset.value = Math.max(0, offset.value - pageSize)
  reload(false)
}

function nextPage() {
  offset.value += pageSize
  reload(false)
}

async function clearLogs() {
  const ok = await confirm.show('清空日志', '确定清空全部请求日志吗？此操作不可撤销。', 'danger')
  if (!ok) return
  try {
    await routerService.clearLogs()
    toast.success('日志已清空')
    await reload(true)
    await routerStore.refreshStatus()
  } catch (e: any) {
    toast.error('清空失败: ' + (e?.message || String(e)))
  }
}
</script>
