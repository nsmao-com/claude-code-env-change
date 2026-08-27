<template>
  <AppModal v-model="isOpen" size="lg" :plain="embedded" :close-on-overlay="false">
    <template #header>
      <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">云同步</h1>
      <p class="mt-2 text-sm text-muted-foreground">把配置自动备份到 S3 / 阿里云 OSS，换电脑后凭同一套凭证拉取</p>
    </template>

    <Card class="mb-4">
      <CardHeader>
        <div class="flex items-center justify-between">
          <div>
            <CardTitle>状态</CardTitle>
            <CardDescription>
              <template v-if="status?.last_push_at">上次上传 {{ formatTime(status.last_push_at) }}</template>
              <template v-else>尚未上传</template>
              <span class="mx-1">·</span>
              <template v-if="status?.last_pull_at">上次拉取 {{ formatTime(status.last_pull_at) }}</template>
              <template v-else>尚未拉取</template>
            </CardDescription>
            <p v-if="status?.last_error" class="mt-1 text-[11px] text-destructive">{{ status.last_error }}</p>
          </div>
          <Badge :variant="form.enabled ? 'default' : 'secondary'">
            {{ form.enabled ? '已启用' : '未启用' }}
          </Badge>
        </div>
      </CardHeader>
    </Card>

    <div class="space-y-4">
      <div class="flex items-center gap-2">
        <Switch :checked="form.enabled" @update:checked="form.enabled = $event" />
        <Label>启用云同步</Label>
      </div>

      <div class="grid grid-cols-2 gap-3">
        <div class="grid gap-1.5">
          <Label>服务商</Label>
          <Select :model-value="form.provider" @update:model-value="onProviderSelect">
            <SelectTrigger class="w-full">
              <SelectValue placeholder="选择服务商" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="aliyun">阿里云 OSS</SelectItem>
              <SelectItem value="s3">AWS S3</SelectItem>
              <SelectItem value="tencent">腾讯云 COS</SelectItem>
              <SelectItem value="r2">Cloudflare R2</SelectItem>
              <SelectItem value="minio">MinIO</SelectItem>
              <SelectItem value="custom">自定义 S3 兼容</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <AppInput v-model="form.region" label="Region" :placeholder="regionPlaceholder" />
      </div>

      <AppInput v-model="form.endpoint" label="Endpoint" :placeholder="endpointPlaceholder" hint="可不含 https://。阿里云示例：oss-cn-hangzhou.aliyuncs.com" />
      <AppInput v-model="form.bucket" label="Bucket" placeholder="bucket-name" />
      <AppInput v-model="form.object_key" label="对象 Key" placeholder="claude-env-switcher/backup.bin" />
      <AppInput v-model="form.access_key" label="Access Key" placeholder="AccessKeyId" />
      <AppInput v-model="form.secret_key" label="Secret Key" type="password" placeholder="留空则保持原值" />
      <AppInput
        v-model="form.passphrase"
        label="加密口令（推荐）"
        type="password"
        hint="填写后整包 AES-GCM 加密再上传。换电脑拉取时必须使用同一口令。上游 API Key 仍明文存在本机 router.json / mcp.json，与现有做法一致。"
      />

      <div class="flex items-center gap-2">
        <Switch :checked="form.path_style" @update:checked="form.path_style = $event" />
        <Label>Path-style 访问（MinIO / 部分私有化 S3 需要）</Label>
      </div>
      <div class="flex items-center gap-2">
        <Switch :checked="form.auto_push" @update:checked="form.auto_push = $event" />
        <Label>本地配置变更后自动上传</Label>
      </div>
      <div class="flex items-center gap-2">
        <Switch :checked="form.auto_pull_on_start" @update:checked="form.auto_pull_on_start = $event" />
        <Label>启动时自动从云端拉取（覆盖本地）</Label>
      </div>

      <p class="text-[11px] leading-relaxed text-muted-foreground">
        换电脑：先在本机填写同样的 OSS 凭证（或设置环境变量 CLAUDIA_OSS_BUCKET / ACCESS_KEY / SECRET_KEY），点「从云端拉取」。
        同步内容包括环境配置、MCP、API 路由、Skills、监控轮换。OSS 凭证本身只保存在本机 cloud.json，不会写进备份包。
      </p>
    </div>

    <template #footer>
      <div class="flex w-full items-center justify-between">
        <div class="flex">
          <Button type="button" variant="outline" size="sm" :disabled="busy" @click="testConn">
            <Loader2 v-if="testing" class="animate-spin" />
            <Unplug v-else />
            测试连接
          </Button>
          <Button type="button" variant="outline" size="sm" :disabled="busy" @click="upload">
            <Loader2 v-if="uploading" class="animate-spin" />
            <Upload v-else />
            立即上传
          </Button>
          <Button type="button" variant="outline" size="sm" :disabled="busy" @click="download">
            <Loader2 v-if="downloading" class="animate-spin" />
            <Download v-else />
            从云端拉取
          </Button>
        </div>
        <Button type="button" :disabled="busy" @click="save">保存设置</Button>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Cloud, Download, Loader2, Unplug, Upload } from '@lucide/vue'
import type { CloudConfig, CloudProvider } from '@/types'
import { useCloudStore } from '@/stores/cloudStore'
import { cloudService } from '@/services/cloudService'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

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
const busy = computed(() => testing.value || uploading.value || downloading.value)

const endpointPlaceholder = computed(() => {
  switch (form.provider) {
    case 'aliyun':
      return 'oss-cn-hangzhou.aliyuncs.com'
    case 's3':
      return '留空则使用官方 s3.{region}.amazonaws.com'
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
})

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

async function save() {
  try {
    await cloudStore.save({ ...form })
    toast.success('云同步设置已保存')
  } catch (e: any) {
    toast.error('保存失败: ' + (e?.message || String(e)))
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
    toast.error('测试失败: ' + (e?.message || String(e)))
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
    toast.error('上传失败: ' + (e?.message || String(e)))
  } finally {
    uploading.value = false
  }
}

async function download() {
  const ok = await confirm.show(
    '从云端拉取',
    '将用云端备份覆盖本机的环境 / MCP / 路由 / Skills / 监控配置。OSS 凭证不会被覆盖。确定继续？',
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
  } catch (e: any) {
    toast.error('拉取失败: ' + (e?.message || String(e)))
  } finally {
    downloading.value = false
  }
}
</script>
