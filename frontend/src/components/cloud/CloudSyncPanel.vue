<template>
  <AppModal v-model="isOpen" size="lg" :plain="embedded" width="form" :close-on-overlay="false">
    <template #header>
      <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">{{ t('nav.cloud') }}</h1>
      <p class="mt-2 text-sm text-muted-foreground">{{ t('cloud.panelHint') }}</p>
    </template>

    <Card class="mb-4">
      <CardHeader>
        <CardTitle>{{ t('cloud.status') }}</CardTitle>
        <CardDescription>{{ t('cloud.statusDesc') }}</CardDescription>
        <CardAction>
          <div class="flex items-center gap-2">
            <span class="text-xs text-muted-foreground">{{ form.enabled ? t('cloud.enabled') : t('cloud.disabled') }}</span>
            <Switch :checked="form.enabled" :disabled="busy" @update:checked="onFlag('enabled', $event)" />
          </div>
        </CardAction>
      </CardHeader>
      <CardContent>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div class="rounded-lg bg-muted/60 px-3 py-2.5">
            <p class="text-[11px] text-muted-foreground">{{ t('cloud.lastPush') }}</p>
            <p class="mt-0.5 text-sm font-medium">{{ status?.last_push_at ? formatTime(status.last_push_at) : t('cloud.neverPushed') }}</p>
          </div>
          <div class="rounded-lg bg-muted/60 px-3 py-2.5">
            <p class="text-[11px] text-muted-foreground">{{ t('cloud.lastPull') }}</p>
            <p class="mt-0.5 text-sm font-medium">{{ status?.last_pull_at ? formatTime(status.last_pull_at) : t('cloud.neverPulled') }}</p>
          </div>
        </div>
        <p v-if="status?.last_error" class="mt-3 text-[11px] text-destructive">{{ status.last_error }}</p>
      </CardContent>
    </Card>

    <Card v-if="status?.configured" class="mb-4">
      <CardHeader>
        <CardTitle>{{ t('cloud.history') }}</CardTitle>
        <CardDescription>{{ t('cloud.historyDesc', { count: HISTORY_KEEP }) }}</CardDescription>
        <CardAction>
          <Button variant="ghost" size="icon-sm" :disabled="versionsLoading" @click="loadVersions">
            <RefreshCw :class="['size-3.5', versionsLoading && 'animate-spin']" />
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        <p v-if="versionsError" class="text-[11px] text-destructive">{{ versionsError }}</p>
        <p v-else-if="versions.length === 0" class="text-xs text-muted-foreground">
          {{ versionsLoading ? t('cloud.loadingHistory') : t('cloud.noHistory') }}
        </p>
        <div v-else class="max-h-64 divide-y overflow-y-auto">
          <div v-for="(v, i) in versions" :key="v.key" class="flex items-center justify-between gap-3 py-2 first:pt-0 last:pb-0">
            <div class="min-w-0">
              <p class="flex items-center gap-2 text-sm font-medium tabular-nums">
                {{ formatTime(v.exported_at) }}
                <Badge v-if="i === 0" variant="secondary">{{ t('cloud.latest') }}</Badge>
                <Badge v-if="v.device && v.device === form.device_id" variant="outline">{{ t('cloud.thisDevice') }}</Badge>
              </p>
              <p class="truncate text-[11px] text-muted-foreground">
                {{ t('cloud.versionMeta', { host: v.hostname || t('cloud.unknownHost'), files: v.files, size: formatSize(v.size) }) }}
              </p>
            </div>
            <Button variant="outline" size="sm" class="shrink-0" :disabled="busy" @click="restoreVersion(v)">
              <Loader2 v-if="restoringKey === v.key" class="animate-spin" />
              <History v-else />
              {{ t('cloud.restore') }}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <div class="space-y-5">
      <div class="grid grid-cols-2 gap-3">
        <div class="grid gap-1.5">
          <Label>{{ t('cloud.provider') }}</Label>
          <Select :model-value="form.provider" @update:model-value="onProviderSelect">
            <SelectTrigger class="w-full">
              <SelectValue :placeholder="t('cloud.providerPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="aliyun">{{ t('cloud.aliyun') }}</SelectItem>
              <SelectItem value="s3">AWS S3</SelectItem>
              <SelectItem value="tencent">{{ t('cloud.tencent') }}</SelectItem>
              <SelectItem value="r2">Cloudflare R2</SelectItem>
              <SelectItem value="minio">MinIO</SelectItem>
              <SelectItem value="custom">{{ t('cloud.custom') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <AppInput v-model="form.region" label="Region" :placeholder="regionPlaceholder" />
      </div>

      <AppInput v-model="form.endpoint" label="Endpoint" :placeholder="endpointPlaceholder" :hint="t('cloud.endpointHint')" />
      <AppInput v-model="form.bucket" label="Bucket" placeholder="bucket-name" />
      <AppInput v-model="form.object_key" :label="t('cloud.objectKey')" placeholder="claude-env-switcher/backup.bin" />
      <AppInput v-model="form.access_key" label="Access Key" placeholder="AccessKeyId" />
      <AppInput v-model="form.secret_key" label="Secret Key" type="password" :placeholder="t('cloud.secretPlaceholder')" />
      <AppInput
        v-model="form.passphrase"
        :label="t('cloud.passphrase')"
        type="password"
        :hint="t('cloud.passphraseHint')"
      />

      <div class="flex items-center gap-2">
        <Switch :checked="form.path_style" :disabled="busy" @update:checked="onFlag('path_style', $event)" />
        <Label>{{ t('cloud.pathStyle') }}</Label>
      </div>
      <div class="flex items-center gap-2">
        <Switch :checked="form.auto_push" :disabled="busy" @update:checked="onFlag('auto_push', $event)" />
        <Label>{{ t('cloud.autoPush') }}</Label>
      </div>
      <div class="flex items-center gap-2">
        <Switch :checked="form.auto_pull_on_start" :disabled="busy" @update:checked="onFlag('auto_pull_on_start', $event)" />
        <Label>{{ t('cloud.autoPull') }}</Label>
      </div>

      <p class="text-[11px] leading-relaxed text-muted-foreground">
        {{ t('cloud.note') }}
      </p>
    </div>

    <template #footer>
      <div class="flex w-full items-center justify-between gap-3">
        <div class="flex gap-2">
          <Button type="button" variant="outline" size="sm" :disabled="busy" @click="testConn">
            <Loader2 v-if="testing" class="animate-spin" />
            <Unplug v-else />
            {{ t('cloud.test') }}
          </Button>
          <Button type="button" variant="outline" size="sm" :disabled="busy" @click="upload">
            <Loader2 v-if="uploading" class="animate-spin" />
            <Upload v-else />
            {{ t('cloud.upload') }}
          </Button>
          <Button type="button" variant="outline" size="sm" :disabled="busy" @click="download">
            <Loader2 v-if="downloading" class="animate-spin" />
            <Download v-else />
            {{ t('cloud.download') }}
          </Button>
        </div>
        <Button type="button" :disabled="busy" @click="save">{{ t('cloud.save') }}</Button>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { computed, reactive, ref, watch } from 'vue'
import { Download, History, Loader2, RefreshCw, Unplug, Upload } from '@lucide/vue'
import type { CloudConfig, CloudProvider, CloudSyncResult, CloudVersion } from '@/types'
import { useCloudStore } from '@/stores/cloudStore'
import { cloudService } from '@/services/cloudService'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

const { t } = useI18n()

interface Props {
  modelValue: boolean
  embedded?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  pulled: []
}>()

const cloudStore = useCloudStore()
const confirm = useConfirm()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const status = computed(() => cloudStore.status)
const form = reactive<CloudConfig>({
  enabled: false,
  provider: 'aliyun',
  endpoint: 'oss-cn-hangzhou.aliyuncs.com',
  region: 'oss-cn-hangzhou',
  bucket: '',
  object_key: 'claude-env-switcher/backup.bin',
  access_key: '',
  secret_key: '',
  path_style: false,
  passphrase: '',
  auto_push: true,
  auto_pull_on_start: false
})

const testing = ref(false)
const uploading = ref(false)
const downloading = ref(false)
const saving = ref(false)
const restoringKey = ref('')
const busy = computed(() => testing.value || uploading.value || downloading.value || saving.value || restoringKey.value !== '')

// 与后端 cloudHistoryKeep 一致
const HISTORY_KEEP = 10
const versions = ref<CloudVersion[]>([])
const versionsLoading = ref(false)
const versionsError = ref('')

async function loadVersions() {
  if (!cloudStore.status?.configured) {
    versions.value = []
    return
  }
  versionsLoading.value = true
  versionsError.value = ''
  try {
    versions.value = await cloudService.listVersions()
  } catch (e: any) {
    versionsError.value = t('cloud.historyFailed', { error: e?.message || String(e) })
  } finally {
    versionsLoading.value = false
  }
}

function formatSize(bytes: number) {
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${bytes} B`
}

function conflictDetail(result: CloudSyncResult) {
  return result.remote_at
    ? t('cloud.conflictRemote', { host: result.remote_host || t('cloud.unknownHost'), time: formatTime(result.remote_at) })
    : t('cloud.conflictLocal')
}

async function restoreVersion(version: CloudVersion) {
  const time = formatTime(version.exported_at)
  const ok = await confirm.show(t('cloud.restoreTitle'), t('cloud.restoreMsg', { time }), 'warning')
  if (!ok) return
  restoringKey.value = version.key
  try {
    const result = await cloudService.restoreVersion(version.key)
    if (result.success) {
      toast.success(t('cloud.restored', { time }))
      emit('pulled')
    } else {
      toast.error(result.message)
    }
    await cloudStore.refreshStatus()
    await loadVersions()
  } catch (e: any) {
    toast.error(t('cloud.restoreFailed', { error: e?.message || String(e) }))
  } finally {
    restoringKey.value = ''
  }
}

type FlagKey = 'enabled' | 'path_style' | 'auto_push' | 'auto_pull_on_start'

const endpointPlaceholder = computed(() => {
  switch (form.provider) {
    case 'aliyun':
      return 'oss-cn-hangzhou.aliyuncs.com'
    case 's3':
      return t('cloud.s3EndpointPlaceholder')
    case 'tencent':
      return 'cos.ap-guangzhou.myqcloud.com'
    case 'r2':
      return '<accountid>.r2.cloudflarestorage.com'
    default:
      return 's3.example.com:9000'
  }
})

const regionPlaceholder = computed(() => {
  switch (form.provider) {
    case 'aliyun':
      return 'oss-cn-hangzhou'
    case 's3':
      return 'us-east-1'
    case 'tencent':
      return 'ap-guangzhou'
    case 'r2':
      return 'auto'
    default:
      return 'us-east-1'
  }
})

watch(isOpen, async (open) => {
  if (!open) return
  await cloudStore.load()
  Object.assign(form, cloudStore.config)
  void loadVersions()
}, { immediate: true })

function onProviderSelect(value: unknown) {
  if (typeof value !== 'string') return
  form.provider = value as CloudProvider
  onProviderChange()
}

function onProviderChange() {
  const p = form.provider as CloudProvider
  if (p === 'aliyun') {
    if (!form.endpoint) form.endpoint = 'oss-cn-hangzhou.aliyuncs.com'
    form.path_style = false
  } else if (p === 's3') {
    form.path_style = false
  } else if (p === 'tencent') {
    if (!form.endpoint) form.endpoint = 'cos.ap-guangzhou.myqcloud.com'
    form.path_style = false
  } else if (p === 'r2') {
    form.region = form.region || 'auto'
    form.path_style = true
  } else {
    form.path_style = true
  }
}

function formatTime(ts: number) {
  if (!ts) return ''
  return new Date(ts).toLocaleString()
}

async function persist(successMessage: string) {
  saving.value = true
  try {
    await cloudStore.save({ ...form })
    toast.success(successMessage)
  } catch (e: any) {
    toast.error(t('cloud.saveFailed', { error: e?.message || String(e) }))
    throw e
  } finally {
    saving.value = false
  }
}

async function onFlag(key: FlagKey, value: boolean) {
  const prev = form[key]
  form[key] = value
  try {
    const name = t(`cloud.flag.${key}`)
    await persist(value ? t('cloud.flagOn', { name }) : t('cloud.flagOff', { name }))
  } catch {
    form[key] = prev
  }
}

async function save() {
  try {
    await persist(t('cloud.saved'))
    // 凭证或对象 Key 可能改了，历史列表跟着换
    void loadVersions()
  } catch {
    /* persist 已提示 */
  }
}

async function testConn() {
  testing.value = true
  try {
    await cloudStore.save({ ...form })
    const result = await cloudService.testConnection()
    if (result.success) toast.success(`${result.message} (${result.latency}ms)`)
    else toast.error(result.message)
    await cloudStore.refreshStatus()
  } catch (e: any) {
    toast.error(t('cloud.testFailed', { error: e?.message || String(e) }))
  } finally {
    testing.value = false
  }
}

async function upload() {
  uploading.value = true
  try {
    await cloudStore.save({ ...form })
    let result = await cloudService.upload()
    if (result.conflict) {
      // 云端有别的电脑的新备份：确认后才覆盖（被覆盖的那份仍在历史版本里）
      const ok = await confirm.show(t('cloud.conflictTitle'), t('cloud.conflictUpload', { detail: conflictDetail(result) }), 'warning')
      if (!ok) {
        await cloudStore.refreshStatus()
        return
      }
      result = await cloudService.forceUpload()
    }
    if (result.success) toast.success(result.message)
    else toast.error(result.message)
    await cloudStore.refreshStatus()
    await loadVersions()
  } catch (e: any) {
    toast.error(t('cloud.uploadFailed', { error: e?.message || String(e) }))
  } finally {
    uploading.value = false
  }
}

async function download() {
  const ok = await confirm.show(
    t('cloud.pullTitle'),
    t('cloud.pullMsg'),
    'warning'
  )
  if (!ok) return
  downloading.value = true
  try {
    await cloudStore.save({ ...form })
    const result = await cloudService.download()
    if (result.success) {
      toast.success(result.message)
      emit('pulled')
    } else {
      toast.error(result.message)
    }
    await cloudStore.refreshStatus()
    await loadVersions()
  } catch (e: any) {
    toast.error(t('cloud.pullFailed', { error: e?.message || String(e) }))
  } finally {
    downloading.value = false
  }
}
</script>
