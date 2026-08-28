<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-sm font-medium">{{ t('settings.cliTitle') }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('settings.cliHint') }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <Button variant="outline" size="sm" :disabled="diagnosing" @click="diagnose">
          <Loader2 v-if="diagnosing" class="animate-spin" />
          <Stethoscope v-else />
          {{ t('settings.cliDiagnose') }}
        </Button>
        <Button variant="outline" size="sm" :disabled="loading || upgradingAll" @click="refresh">
          <RefreshCw :class="loading && 'animate-spin'" />
          {{ t('settings.cliRefresh') }}
        </Button>
        <Button size="sm" :disabled="loading || upgradingAll || upgradableCount === 0" @click="upgradeAll">
          <Loader2 v-if="upgradingAll" class="animate-spin" />
          <ArrowUpCircle v-else />
          {{ t('settings.cliUpgradeAll') }}
          <span v-if="upgradableCount"> ({{ upgradableCount }})</span>
        </Button>
      </div>
    </div>

    <p v-if="loadError" class="rounded-xl bg-destructive/10 px-3 py-2 text-xs text-destructive">{{ loadError }}</p>

    <div class="grid grid-cols-1 gap-3 lg:grid-cols-2" :class="loading && 'opacity-70'">
      <Card v-for="tool in tools" :key="tool.id">
        <CardHeader>
          <div class="flex min-w-0 items-start justify-between gap-3">
            <div class="flex min-w-0 items-center gap-2">
              <BrandIcon :provider="tool.id" class="size-5" />
              <div class="min-w-0">
                <CardTitle class="text-sm">{{ tool.name }}</CardTitle>
                <div class="mt-1 flex flex-wrap items-center gap-1.5">
                  <Badge variant="outline">{{ tool.platform }}</Badge>
                  <Badge v-if="tool.install_method && tool.install_method !== 'unknown'" variant="secondary">
                    {{ tool.install_method }}
                  </Badge>
                </div>
              </div>
            </div>
            <Badge v-if="tool.upgradable" class="shrink-0">{{ t('settings.cliUpgradable') }}</Badge>
            <Badge v-else-if="!tool.installed" variant="outline">{{ t('settings.cliMissing') }}</Badge>
            <Badge v-else-if="!tool.runnable" variant="destructive">{{ t('settings.cliBroken') }}</Badge>
            <Badge v-else variant="secondary">{{ t('settings.cliOk') }}</Badge>
          </div>
        </CardHeader>
        <CardContent class="space-y-3">
          <div class="grid grid-cols-2 gap-2 text-xs">
            <div>
              <p class="text-muted-foreground">{{ t('settings.cliCurrent') }}</p>
              <p class="mt-0.5 font-mono">{{ tool.current_version || '-' }}</p>
            </div>
            <div class="text-right">
              <p class="text-muted-foreground">{{ t('settings.cliLatest') }}</p>
              <p class="mt-0.5 font-mono">{{ tool.latest_version || '-' }}</p>
            </div>
          </div>
          <AppTooltip v-if="tool.install_path" :content="tool.install_path" wrap>
            <p class="truncate font-mono text-[11px] text-muted-foreground">{{ tool.install_path }}</p>
          </AppTooltip>
          <p v-if="tool.config_dir" class="truncate font-mono text-[11px] text-muted-foreground">
            {{ tool.config_exists ? t('settings.dirsExists') : t('settings.dirsMissing') }} · {{ tool.config_dir }}
          </p>
          <p v-if="tool.error" class="line-clamp-2 text-xs text-destructive">{{ tool.error }}</p>
          <p v-if="tool.extra_paths?.length > 1" class="text-xs text-amber-600 dark:text-amber-400">
            {{ t('settings.cliConflictCount', { count: tool.extra_paths.length }) }}
          </p>
          <p v-if="progressText[tool.id]" class="text-xs text-muted-foreground">
            {{ progressText[tool.id] }}
          </p>
          <div
            v-if="lastResult[tool.id]"
            class="rounded-lg px-2.5 py-2 text-xs"
            :class="lastResult[tool.id].ok ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400' : 'bg-destructive/10 text-destructive'"
          >
            <p>{{ lastResult[tool.id].message }}</p>
            <p v-if="lastResult[tool.id].log" class="mt-1 max-h-24 overflow-auto whitespace-pre-wrap break-all font-mono text-[11px] opacity-80">
              {{ lastResult[tool.id].log }}
            </p>
          </div>
          <div class="flex justify-end">
            <Button
              size="sm"
              :disabled="busyId === tool.id || upgradingAll || (!tool.installed && !tool.npm_package)"
              @click="upgradeOne(tool)"
            >
              <Loader2 v-if="busyId === tool.id" class="animate-spin" />
              {{ tool.installed ? t('settings.cliUpgrade') : t('settings.cliInstall') }}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>

    <AppModal v-model="showDiagnose" :title="t('settings.cliDiagnoseTitle')" size="lg">
      <p class="mb-4 text-sm text-muted-foreground">{{ t('settings.cliDiagnoseHint') }}</p>
      <div class="space-y-3">
        <div
          v-for="tool in tools"
          :key="tool.id"
          class="rounded-xl bg-muted/40 px-3 py-3 ring-1 ring-black/[0.04] dark:ring-white/10"
        >
          <div class="flex items-center justify-between gap-2">
            <div class="flex min-w-0 items-center gap-2">
              <BrandIcon :provider="tool.id" class="size-4" />
              <span class="text-sm font-medium">{{ tool.name }}</span>
            </div>
            <Badge v-if="diagnosePaths(tool).length > 1">{{ t('settings.cliConflictCount', { count: diagnosePaths(tool).length }) }}</Badge>
            <Badge v-else-if="tool.installed" variant="secondary">{{ t('settings.cliOk') }}</Badge>
            <Badge v-else variant="outline">{{ t('settings.cliMissing') }}</Badge>
          </div>
          <div v-if="diagnosePaths(tool).length" class="mt-2 space-y-1">
            <p
              v-for="path in diagnosePaths(tool)"
              :key="path"
              class="break-all font-mono text-[11px] text-muted-foreground"
            >
              {{ path }}
            </p>
          </div>
          <p v-else class="mt-2 text-xs text-muted-foreground">{{ t('settings.cliNone') }}</p>
        </div>
      </div>
      <template #footer>
        <Button type="button" variant="outline" @click="showDiagnose = false">{{ t('common.cancel') }}</Button>
      </template>
    </AppModal>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ArrowUpCircle, Loader2, RefreshCw, Stethoscope } from '@lucide/vue'
import type { CliToolStatus, CliUpgradeResult } from '@/types'
import { CLI_TOOL_DEFAULTS, normalizeCliTools } from '@/lib/cliCatalog'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { asRecord, callApp, onAppEvent, pickBool, pickText } from '@/services/appBridge'
import AppModal from '@/components/common/AppModal.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const { t } = useI18n()
const toast = useToast()
const tools = ref<CliToolStatus[]>(CLI_TOOL_DEFAULTS.map(item => ({ ...item })))
const loading = ref(false)
const loadError = ref('')
const busyId = ref('')
const upgradingAll = ref(false)
const diagnosing = ref(false)
const showDiagnose = ref(false)
const progressText = ref<Record<string, string>>({})
const lastResult = ref<Record<string, { ok: boolean, message: string, log: string }>>({})
let stopProgress: (() => void) | undefined

const upgradableCount = computed(() => tools.value.filter(item => item.upgradable).length)

async function refresh() {
  loading.value = true
  loadError.value = ''
  try {
    const list = normalizeCliTools(await callApp<unknown>('ListCliTools'))
    if (list.length > 0) tools.value = list
  } catch (e: unknown) {
    loadError.value = e instanceof Error ? e.message : String(e)
    toast.error(loadError.value)
  } finally {
    loading.value = false
  }
}

function diagnosePaths(tool: CliToolStatus) {
  const paths = [...(tool.extra_paths || [])]
  if (tool.install_path && !paths.includes(tool.install_path)) paths.unshift(tool.install_path)
  return paths
}

async function diagnose() {
  if (diagnosing.value) return
  diagnosing.value = true
  try {
    await refresh()
  } finally {
    diagnosing.value = false
    showDiagnose.value = true
    const conflicts = tools.value.filter(item => diagnosePaths(item).length > 1)
    if (conflicts.length === 0) toast.success(t('settings.cliNoConflict'))
    else toast.error(t('settings.cliConflictFound', { count: conflicts.length }))
  }
}

function rememberResult(id: string, ok: boolean, message: string, log = '') {
  const text = message.trim() || (ok ? t('settings.cliUpgradeOk') : t('settings.cliUpgradeFail'))
  lastResult.value = { ...lastResult.value, [id]: { ok, message: text, log: log.trim() } }
  progressText.value = { ...progressText.value, [id]: '' }
  if (ok) toast.success(text)
  else toast.error(text)
}

async function upgradeOne(tool: CliToolStatus) {
  busyId.value = tool.id
  progressText.value = { ...progressText.value, [tool.id]: t('settings.cliUpgrading', { name: tool.name }) }
  toast.info(t('settings.cliUpgrading', { name: tool.name }))
  try {
    const result = await callApp<CliUpgradeResult>('UpgradeCliTool', tool.id)
    rememberResult(
      tool.id,
      pickBool(result, 'success', 'Success'),
      pickText(result, 'message', 'Message'),
      pickText(result, 'log', 'Log'),
    )
    await refresh()
  } catch (e: unknown) {
    rememberResult(tool.id, false, e instanceof Error ? e.message : String(e))
  } finally {
    busyId.value = ''
  }
}

async function upgradeAll() {
  upgradingAll.value = true
  toast.info(t('settings.cliUpgrading', { name: t('settings.cliUpgradeAll') }))
  try {
    const results = (await callApp<unknown>('UpgradeAllCliTools')) || []
    const list = Array.isArray(results) ? results : []
    if (list.length === 0) {
      toast.success(t('settings.cliUpgradeAllOk'))
      await refresh()
      return
    }
    for (const item of list) {
      const rec = asRecord(item)
      const id = pickText(rec, 'id', 'ID')
      if (!id) continue
      rememberResult(
        id,
        pickBool(rec, 'success', 'Success'),
        pickText(rec, 'message', 'Message'),
        pickText(rec, 'log', 'Log'),
      )
    }
    const failed = list.filter(item => !pickBool(item, 'success', 'Success'))
    if (failed.length === 0) toast.success(t('settings.cliUpgradeAllOk'))
    else toast.error(failed.map(item => pickText(item, 'message', 'Message') || t('settings.cliUpgradeFail')).join('；'))
    await refresh()
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    upgradingAll.value = false
  }
}

onMounted(() => {
  void refresh()
  stopProgress = onAppEvent('cli:progress', (data) => {
    const rec = asRecord(data)
    const id = pickText(rec, 'id', 'ID')
    const message = pickText(rec, 'message', 'Message')
    if (!id || !message) return
    progressText.value = { ...progressText.value, [id]: message }
  })
})

onBeforeUnmount(() => {
  stopProgress?.()
})
</script>
