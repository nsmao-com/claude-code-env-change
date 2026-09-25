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
              <SelectItem value="webdav">WebDAV</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <AppInput v-if="form.provider !== 'webdav'" v-model="form.region" label="Region" :placeholder="regionPlaceholder" />
      </div>

      <AppInput v-model="form.endpoint" label="Endpoint" :placeholder="endpointPlaceholder" :hint="t('cloud.endpointHint')" />
      <AppInput v-if="form.provider !== 'webdav'" v-model="form.bucket" label="Bucket" placeholder="bucket-name" />
      <AppInput v-model="form.object_key" :label="t('cloud.objectKey')" placeholder="claude-env-switcher/backup.bin" />
      <AppInput v-model="form.access_key" :label="form.provider === 'webdav' ? tx('用户名', 'Username') : 'Access Key'" />
      <AppInput v-model="form.secret_key" :label="form.provider === 'webdav' ? tx('密码 / 应用令牌', 'Password / app token') : 'Secret Key'" type="password" :placeholder="t('cloud.secretPlaceholder')" />
      <AppInput
        v-model="form.passphrase"
        :label="t('cloud.passphrase')"
        type="password"
        :hint="t('cloud.passphraseHint')"
      />

      <div v-if="form.provider !== 'webdav'" class="flex items-center gap-2">
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

    <div v-if="restorePreview" class="workbench mt-5"><DiffPreview :changes="restorePreview.changes" v-model="restoreFiles"/><Button class="mt-4" :disabled="busy || !restoreFiles.length" @click="confirmRestore">{{ tx('确认恢复所选文件', 'Restore selected files') }}</Button></div>
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
import { useWorkbench } from '@/composables/useWorkbench'
import { callService } from '@/services/appBridge'
import type { HistoryPreview, Result } from '@/types/workbench'
import DiffPreview from '@/components/workbench/DiffPreview.vue'
import { computed, reactive, ref, watch } from 'vue'
import { Download, Loader2, Unplug, Upload } from '@lucide/vue'
import type { CloudConfig, CloudProvider } from '@/types'
import { useCloudStore } from '@/stores/cloudStore'
import { cloudService } from '@/services/cloudService'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

const { t } = useI18n()
const { tx } = useWorkbench()
const restorePreview = ref<HistoryPreview | null>(null)
const restoreFiles = ref<string[]>([])

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
const busy = computed(() => testing.value || uploading.value || downloading.value || saving.value)

type FlagKey = 'enabled' | 'path_style' | 'auto_push' | 'auto_pull_on_start'

const endpointPlaceholder = computed(() => {
  switch (form.provider) {
    case 'webdav':
      return 'https://dav.example.com/backup'
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
  Object.assign(form, cloudStore.config)}, { immediate: true })

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
    const result = await cloudService.upload()
    if (result.success) toast.success(result.message)
    else toast.error(result.message)
    await cloudStore.refreshStatus()
  } catch (e: any) {
    toast.error(t('cloud.uploadFailed', { error: e?.message || String(e) }))
  } finally {
    uploading.value = false
  }
}

async function download() {
  downloading.value = true
  try {
    await cloudStore.save({ ...form })
    restorePreview.value = await callService<HistoryPreview>('CloudSyncService', 'PreviewCloudRestore')
    restoreFiles.value = restorePreview.value.changes.filter(c => c.changed).map(c => c.path)
  } catch (e: unknown) { toast.error(String(e)) }
  finally { downloading.value = false }
}
async function confirmRestore() {
  if (!restorePreview.value || !await confirm.show(t('cloud.pullTitle'), t('cloud.pullMsg'), 'warning')) return
  downloading.value = true
  try {
    const result = await callService<Result>('CloudSyncService', 'ConfirmCloudRestore', restorePreview.value.token, restoreFiles.value)
    if (!result.success) throw new Error(result.message)
    toast.success(result.message); restorePreview.value = null; emit('pulled')
    await cloudStore.refreshStatus()
  } catch (e: unknown) { toast.error(String(e)) }
  finally { downloading.value = false }
}
</script>
