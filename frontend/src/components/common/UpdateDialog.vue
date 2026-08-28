<template>
  <Dialog :open="open" @update:open="onOpen">
    <DialogContent class="sm:max-w-lg" :show-close-button="!applying">
      <DialogHeader>
        <DialogTitle>软件更新</DialogTitle>
        <DialogDescription>
          从 GitHub Releases 检查版本，可直接在软件内下载安装。
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4">
        <div v-if="status === 'checking' || status === 'idle'" class="space-y-3">
          <Skeleton class="h-16 w-full rounded-xl" />
          <Skeleton class="h-24 w-full rounded-xl" />
        </div>

        <div v-else-if="status === 'error'" class="rounded-xl border border-destructive/20 bg-destructive/5 p-4">
          <div class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="mt-0.5 size-4 shrink-0" />
            <span>{{ error || '检查更新失败' }}</span>
          </div>
        </div>

        <template v-else>
          <div class="grid grid-cols-2 gap-3">
            <Card size="sm">
              <CardHeader class="px-4 py-3">
                <CardDescription>当前版本</CardDescription>
                <CardTitle class="font-mono text-lg">v{{ info?.current_version || '—' }}</CardTitle>
              </CardHeader>
            </Card>
            <Card size="sm">
              <CardHeader class="px-4 py-3">
                <CardDescription>GitHub 最新</CardDescription>
                <CardTitle class="font-mono text-lg">{{ latestLabel }}</CardTitle>
              </CardHeader>
            </Card>
          </div>

          <div class="flex items-center gap-2">
            <Badge :variant="info?.available ? 'default' : 'secondary'">
              {{ info?.available ? '有新版本' : '已是最新' }}
            </Badge>
            <span v-if="info?.published_at" class="text-xs text-muted-foreground">{{ publishedLabel }}</span>
          </div>

          <p v-if="info?.message" class="text-sm text-muted-foreground">{{ info.message }}</p>

          <div v-if="info?.release_notes" class="space-y-1.5">
            <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">更新说明</p>
            <ScrollArea class="h-32 rounded-xl border bg-muted/40 p-3">
              <pre class="whitespace-pre-wrap font-sans text-xs leading-relaxed text-foreground/90">{{ info.release_notes }}</pre>
            </ScrollArea>
          </div>

          <div v-if="info?.asset_name" class="text-xs text-muted-foreground">
            {{ info.asset_name }}
            <span v-if="info.asset_size"> · {{ formatBytes(info.asset_size) }}</span>
          </div>

          <div v-if="applying" class="space-y-2">
            <div class="flex items-center justify-between gap-2 text-xs text-muted-foreground">
              <span>{{ progressMessage }}</span>
              <span class="font-mono">{{ Math.round(progress) }}%</span>
            </div>
            <Progress :model-value="progress" />
          </div>
        </template>
      </div>

      <DialogFooter>
        <Button variant="secondary" :disabled="applying" @click="open = false">稍后</Button>
        <Button variant="outline" :disabled="applying || status === 'checking'" @click="openRelease">
          <ExternalLink />
          打开发布页
        </Button>
        <Button
          v-if="info?.available && info.can_apply"
          :disabled="applying || status !== 'ready'"
          @click="apply"
        >
          <Loader2 v-if="applying" class="animate-spin" />
          <Download v-else />
          {{ applying ? '正在更新' : '立即更新' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { CircleAlert, Download, ExternalLink, Loader2 } from '@lucide/vue'
import type { UpdateInfo } from '@/types'
import { updateService } from '@/services/updateService'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'

const open = defineModel<boolean>({ default: false })
const emit = defineEmits<{
  available: []
}>()

const status = ref<'idle' | 'checking' | 'ready' | 'error'>('idle')
const info = ref<UpdateInfo | null>(null)
const error = ref('')
const applying = ref(false)
const progress = ref(0)
const progressMessage = ref('准备下载…')
let lastCheckAt = 0
let offProgress: (() => void) | null = null
let silentTimer = 0

const latestLabel = computed(() => {
  const v = info.value?.latest_version
  if (!v) return '—'
  return v.startsWith('v') ? v : `v${v}`
})

const publishedLabel = computed(() => {
  const raw = info.value?.published_at
  if (!raw) return ''
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  return d.toLocaleString()
})

function formatBytes(n: number) {
  if (!n) return ''
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

function onOpen(next: boolean) {
  if (!next && applying.value) return
  open.value = next
}

async function check() {
  status.value = 'checking'
  error.value = ''
  try {
    const data = await updateService.check()
    info.value = data
    status.value = 'ready'
    lastCheckAt = Date.now()
    if (data.available) emit('available')
  } catch (e: any) {
    status.value = 'error'
    error.value = e?.message || String(e) || '检查更新失败'
  }
}

async function apply() {
  if (!info.value?.can_apply || applying.value) return
  applying.value = true
  progress.value = 0
  progressMessage.value = '开始下载…'
  try {
    await updateService.apply()
    progressMessage.value = '即将重启…'
  } catch (e: any) {
    applying.value = false
    status.value = 'error'
    error.value = e?.message || String(e) || '更新失败'
  }
}

function openRelease() {
  const url = info.value?.release_url
  if (url) {
    updateService.openUrl(url)
    return
  }
  updateService.openReleasePage()
}

watch(open, (v) => {
  if (!v || applying.value) return
  if (info.value && Date.now() - lastCheckAt < 2500) return
  void check()
})

onMounted(() => {
  offProgress = updateService.onProgress((p) => {
    progress.value = p.percent || 0
    if (p.message) progressMessage.value = p.message
    if (p.phase === 'error') {
      applying.value = false
      status.value = 'error'
      error.value = p.message || '更新失败'
    }
  })

  silentTimer = window.setTimeout(async () => {
    try {
      const data = await updateService.check()
      info.value = data
      status.value = 'ready'
      lastCheckAt = Date.now()
      if (data.available) {
        emit('available')
        open.value = true
      }
    } catch {
      /* 启动时静默失败 */
    }
  }, 2800)
})

onUnmounted(() => {
  window.clearTimeout(silentTimer)
  offProgress?.()
})
</script>
