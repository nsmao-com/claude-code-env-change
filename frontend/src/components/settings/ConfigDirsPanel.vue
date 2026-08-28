<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-sm font-medium">{{ t('settings.dirsTitle') }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('settings.dirsHint') }}</p>
      </div>
      <Button variant="outline" size="sm" :disabled="loading" @click="refresh">
        <RefreshCw :class="loading && 'animate-spin'" />
        {{ t('settings.cliRefresh') }}
      </Button>
    </div>

    <p v-if="loadError" class="rounded-xl bg-destructive/10 px-3 py-2 text-xs text-destructive">{{ loadError }}</p>

    <div class="grid grid-cols-1 gap-3 lg:grid-cols-2" :class="loading && 'opacity-70'">
      <Card v-for="item in dirs" :key="item.id">
        <CardHeader>
          <div class="flex min-w-0 items-center justify-between gap-3">
            <div class="flex min-w-0 items-center gap-2">
              <BrandIcon v-if="item.id !== 'ai-env'" :provider="item.id" class="size-5" />
              <Folder v-else class="size-5 text-muted-foreground" />
              <CardTitle class="truncate text-sm">{{ item.name }}</CardTitle>
            </div>
            <Badge :variant="item.exists ? 'secondary' : 'outline'">
              {{ item.exists ? t('settings.dirsExists') : t('settings.dirsMissing') }}
            </Badge>
          </div>
        </CardHeader>
        <CardContent class="space-y-3">
          <AppTooltip :content="item.dir" wrap>
            <p class="truncate font-mono text-[11px] text-muted-foreground">{{ item.dir }}</p>
          </AppTooltip>
          <div class="flex flex-col gap-1.5">
            <div
              v-for="file in item.files"
              :key="file.path"
              class="flex items-center justify-between gap-2 rounded-lg bg-muted/50 px-2.5 py-1.5"
            >
              <span class="truncate font-mono text-xs">{{ file.name }}</span>
              <div class="flex shrink-0 items-center gap-1">
                <Badge variant="outline">{{ file.exists ? t('settings.dirsFileOn') : t('settings.dirsFileOff') }}</Badge>
                <Button variant="ghost" size="icon-xs" :disabled="!file.exists" @click="openFile(file.path)">
                  <FileText />
                </Button>
              </div>
            </div>
          </div>
          <div class="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" @click="openDir(item.id)">
              <FolderOpen />
              {{ t('settings.dirsOpen') }}
            </Button>
            <Button variant="ghost" size="sm" @click="copyPath(item.dir)">
              <Copy />
              {{ t('settings.dirsCopy') }}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Copy, FileText, Folder, FolderOpen, RefreshCw } from '@lucide/vue'
import type { ConfigDirInfo } from '@/types'
import { CONFIG_DIR_DEFAULTS, normalizeConfigDirs } from '@/lib/cliCatalog'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { callApp } from '@/services/appBridge'
import AppTooltip from '@/components/common/AppTooltip.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const { t } = useI18n()
const toast = useToast()
const dirs = ref<ConfigDirInfo[]>(CONFIG_DIR_DEFAULTS.map(item => ({ ...item, files: item.files.map(file => ({ ...file })) })))
const loading = ref(false)
const loadError = ref('')

async function refresh() {
  loading.value = true
  loadError.value = ''
  try {
    const list = normalizeConfigDirs(await callApp<unknown>('ListConfigDirs'))
    if (list.length > 0) dirs.value = list
  } catch (e: unknown) {
    loadError.value = e instanceof Error ? e.message : String(e)
    toast.error(loadError.value)
  } finally {
    loading.value = false
  }
}

async function openDir(id: string) {
  try {
    await callApp('OpenConfigDir', id)
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : String(e))
  }
}

async function openFile(path: string) {
  try {
    await callApp('OpenConfigFile', path)
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : String(e))
  }
}

async function copyPath(path: string) {
  try {
    await navigator.clipboard.writeText(path)
    toast.success(t('settings.dirsCopied'))
  } catch {
    toast.error(path)
  }
}

onMounted(refresh)
</script>
