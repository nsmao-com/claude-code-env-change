<template>
  <AppModal v-model="isOpen" size="xl" :close-on-overlay="false">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="flex size-10 items-center justify-center rounded-lg bg-primary/10">
          <List class="size-4 text-primary" />
        </div>
        <div>
          <DialogTitle class="text-lg font-semibold">{{ t('router.logs.title') }}</DialogTitle>
          <p class="text-xs text-muted-foreground">{{ t('router.logs.subtitle') }}</p>
        </div>
      </div>
    </template>

    <div class="mb-4 flex flex-wrap items-end">
      <div class="mb-2 mr-3 grid gap-1.5">
        <Label>{{ t('router.logs.route') }}</Label>
        <Select :model-value="routeFilter || '__all__'" @update:model-value="onRouteFilter">
          <SelectTrigger class="w-40 text-xs">
            <SelectValue :placeholder="t('router.logs.all')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">{{ t('router.logs.all') }}</SelectItem>
            <SelectItem v-for="name in routeNames" :key="name" :value="name">{{ name }}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="mb-2 mr-3 grid min-w-[180px] flex-1 gap-1.5">
        <Label>{{ t('router.logs.keyword') }}</Label>
        <Input
          v-model="keyword"
          class="text-xs"
          :placeholder="t('router.logs.keywordPlaceholder')"
          @keyup.enter="reload(true)"
        />
      </div>
      <div class="mb-2 mr-3 flex h-9 items-center gap-2">
        <Switch :checked="onlyErrors" size="sm" @update:checked="onOnlyErrorsChange" />
        <Label class="cursor-pointer text-xs">{{ t('router.logs.onlyErrors') }}</Label>
      </div>
      <Button variant="outline" size="sm" class="mb-2" @click="reload(true)">
        <Loader2 v-if="loading" class="animate-spin" />
        <Search v-else />
        {{ t('router.logs.query') }}
      </Button>
    </div>

    <div class="max-h-[46vh] overflow-auto rounded-lg border">
      <Table class="font-mono text-[11px]">
        <TableHeader class="sticky top-0 bg-card">
          <TableRow>
            <TableHead>{{ t('router.logs.time') }}</TableHead>
            <TableHead>{{ t('router.logs.route') }}</TableHead>
            <TableHead>{{ t('router.logs.path') }}</TableHead>
            <TableHead>{{ t('router.logs.model') }}</TableHead>
            <TableHead>{{ t('router.logs.upstream') }}</TableHead>
            <TableHead>{{ t('router.logs.status') }}</TableHead>
            <TableHead class="text-right">{{ t('router.logs.duration') }}</TableHead>
            <TableHead class="text-right">{{ t('router.logs.tokens') }}</TableHead>
            <TableHead>{{ t('router.logs.error') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableEmpty v-if="!loading && items.length === 0" :colspan="9" class="text-muted-foreground">
            {{ t('router.logs.empty') }}
          </TableEmpty>
          <TableRow v-for="(log, i) in items" :key="i + log.time + log.path">
            <TableCell class="text-muted-foreground">{{ log.time }}</TableCell>
            <TableCell class="font-bold">{{ log.route }}</TableCell>
            <TableCell class="max-w-[170px] truncate text-muted-foreground">
              <AppTooltip :content="log.path" wrap :disabled="!log.path">
                <span class="block truncate">{{ log.path }}</span>
              </AppTooltip>
            </TableCell>
            <TableCell class="max-w-[140px] truncate text-muted-foreground">
              <AppTooltip :content="log.model" wrap :disabled="!log.model">
                <span class="block truncate">{{ log.model }}</span>
              </AppTooltip>
            </TableCell>
            <TableCell class="max-w-[150px] text-muted-foreground">
              <AppTooltip
                :content="log.failover ? t('router.logs.skipped', { list: log.failover }) : (log.upstream || '')"
                wrap
                :disabled="!log.upstream && !log.failover"
              >
                <span class="flex items-center gap-1 truncate">
                  <span v-if="log.failover" class="shrink-0 rounded bg-amber-500/15 px-1 text-amber-600 dark:text-amber-400">{{ t('router.logs.switched') }}</span>
                  <span class="truncate">{{ log.upstream }}</span>
                </span>
              </AppTooltip>
            </TableCell>
            <TableCell :class="log.status_code >= 400 ? 'font-bold text-red-500' : 'text-green-600'">
              {{ log.status_code }}
            </TableCell>
            <TableCell class="text-right text-muted-foreground">{{ log.duration_ms }}ms</TableCell>
            <TableCell class="whitespace-nowrap text-right text-muted-foreground">
              <AppTooltip
                v-if="log.input_tokens || log.output_tokens"
                :content="t('router.logs.tokensTip', { input: log.input_tokens || 0, output: log.output_tokens || 0, cache: log.cache_read_tokens || 0 })"
              >
                <span>{{ formatTokens(log.input_tokens || 0) }} / {{ formatTokens(log.output_tokens || 0) }}</span>
              </AppTooltip>
            </TableCell>
            <TableCell class="max-w-[240px] truncate text-red-500">
              <AppTooltip :content="log.error" wrap :disabled="!log.error">
                <span class="block truncate">{{ log.error }}</span>
              </AppTooltip>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <div class="mt-3 flex items-center justify-between gap-3 text-xs text-muted-foreground">
      <span>{{ t('router.logs.pager', { total, page, pages: pageCount }) }}</span>
      <div class="flex items-center gap-2">
        <Button variant="outline" size="sm" :disabled="offset <= 0" @click="prevPage">{{ t('router.logs.prev') }}</Button>
        <Button variant="outline" size="sm" :disabled="offset + pageSize >= total" @click="nextPage">{{ t('router.logs.next') }}</Button>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-between gap-3">
        <Button type="button" variant="destructive" @click="clearLogs">{{ t('router.logs.clear') }}</Button>
        <Button type="button" variant="secondary" @click="isOpen = false">{{ t('router.logs.close') }}</Button>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
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

const { t } = useI18n()

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

function formatTokens(n: number) {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

function onRouteFilter(value: unknown) {
  routeFilter.value = !value || value === '__all__' ? '' : String(value)
  reload(true)
}

function onOnlyErrorsChange(checked: boolean) {
  onlyErrors.value = checked
  toast.info(checked ? t('router.logs.showErrors') : t('router.logs.showAll'))
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
    toast.error(t('router.logs.loadFailed', { error: e?.message || String(e) }))
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
  const ok = await confirm.show(t('router.logs.clearTitle'), t('router.logs.clearMsg'), 'danger')
  if (!ok) return
  try {
    await routerService.clearLogs()
    toast.success(t('router.logs.cleared'))
    await reload(true)
    await routerStore.refreshStatus()
  } catch (e: any) {
    toast.error(t('router.logs.clearFailed', { error: e?.message || String(e) }))
  }
}
</script>
