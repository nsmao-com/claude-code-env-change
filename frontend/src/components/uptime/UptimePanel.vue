<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" :close-on-overlay="false">
    <template #header>
      <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">监控</h1>
      <p class="mt-2 text-sm text-muted-foreground">每隔一段时间检测可达性，并按轮换组自动切换配置</p>
    </template>

    <Card class="mb-4">
      <CardHeader>
        <div class="flex items-start justify-between gap-4">
          <div>
            <CardTitle>Uptime 监控</CardTitle>
            <CardDescription>
              监控会对各配置的 Base URL 做 HTTP 可达性检测，并保留最近 {{ uptimeStore.settings.keep_last }} 次记录。
            </CardDescription>
          </div>
          <Button variant="outline" size="sm" :disabled="uptimeStore.isRunning" @click="runNow">
            <Loader2 v-if="uptimeStore.isRunning" class="animate-spin" />
            <Zap v-else />
            立即检测
          </Button>
        </div>
      </CardHeader>
      <CardContent class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div class="grid gap-1.5">
          <Label>启用监控</Label>
          <div class="flex items-center gap-2">
            <Switch :checked="form.enabled" :disabled="isSavingSettings" @update:checked="onEnabledChange" />
            <span class="text-xs text-muted-foreground">{{ form.enabled ? '已启用' : '未启用' }}</span>
          </div>
        </div>
        <div class="grid gap-1.5">
          <Label>间隔（分钟）</Label>
          <Input v-model="form.interval_minutes" type="number" min="1" max="1440" />
          <p class="text-[11px] text-muted-foreground">默认 5 分钟</p>
        </div>
        <div class="grid gap-1.5">
          <Label>超时（秒）</Label>
          <Input v-model="form.timeout_seconds" type="number" min="1" max="60" />
          <p class="text-[11px] text-muted-foreground">建议 8-15 秒</p>
        </div>
      </CardContent>
      <CardFooter class="justify-end">
        <Button size="sm" :disabled="isSavingSettings" @click="saveSettings">
          <Loader2 v-if="isSavingSettings" class="animate-spin" />
          <Save v-else />
          保存设置
        </Button>
      </CardFooter>
    </Card>

    <div class="mb-4 flex items-center justify-between gap-4">
      <div>
        <h4 class="text-sm font-medium">轮换组</h4>
        <p class="mt-1 text-xs text-muted-foreground">
          当某组的“当前激活配置”连续失败达到阈值时，自动切换到组内下一个（优先挑选最近成功/未检测过的）。
        </p>
      </div>
      <Button size="sm" @click="openCreate">
        <Plus />
        新建轮换组
      </Button>
    </div>

    <div v-if="uptimeStore.isLoading" class="flex items-center justify-center py-12">
      <Loader2 class="size-6 animate-spin text-muted-foreground" />
    </div>

    <Empty v-else-if="uptimeStore.groups.length === 0" class="border border-dashed py-10">
      <EmptyHeader>
        <Shuffle class="size-8 text-muted-foreground" />
        <EmptyTitle>暂无轮换组</EmptyTitle>
        <EmptyDescription>点击“新建轮换组”开始配置</EmptyDescription>
      </EmptyHeader>
    </Empty>

    <ScrollArea v-else class="h-[45vh] pr-2">
      <div class="space-y-3">
        <Card v-for="group in uptimeStore.groups" :key="group.name">
          <CardHeader>
            <div class="flex min-w-0 items-start justify-between gap-4">
              <div class="min-w-0 flex-1 overflow-hidden">
                <div class="flex min-w-0 items-center gap-2">
                  <AppTooltip :content="group.name" wrap class="min-w-0 flex-1">
                    <CardTitle>{{ group.name }}</CardTitle>
                  </AppTooltip>
                  <Badge variant="outline" class="shrink-0 gap-1">
                    <BrandIcon :provider="group.provider" class="size-3" />
                    {{ providerLabel(group.provider) }}
                  </Badge>
                  <Badge :variant="group.enabled ? 'default' : 'secondary'" class="shrink-0">
                    {{ group.enabled ? 'Enabled' : 'Disabled' }}
                  </Badge>
                </div>
                <CardDescription>
                  连续失败阈值：<span class="font-mono">{{ group.failure_threshold }}</span>
                </CardDescription>
                <div class="mt-3 flex flex-wrap gap-2">
                  <AppTooltip v-for="name in group.env_names" :key="name" :content="name" wrap>
                    <Badge variant="outline" class="max-w-full shrink truncate font-mono">
                      {{ name }}
                    </Badge>
                  </AppTooltip>
                </div>
              </div>
              <div class="flex shrink-0 gap-2">
                <Button variant="outline" size="sm" @click="toggleGroup(group)">
                  <Power />
                  {{ group.enabled ? '停用' : '启用' }}
                </Button>
                <Button variant="outline" size="sm" @click="openEdit(group)">
                  <Pencil />
                  编辑
                </Button>
                <Button variant="destructive" size="sm" @click="remove(group)">
                  <Trash2 />
                  删除
                </Button>
              </div>
            </div>
          </CardHeader>
        </Card>
      </div>
    </ScrollArea>

    <RotationGroupEditModal v-model="showGroupModal" :edit-group="editingGroup" @saved="onGroupSaved" />
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Loader2, Pencil, Plus, Power, Save, Shuffle, Trash2, Zap } from '@lucide/vue'
import type { RotationGroup } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import RotationGroupEditModal from './RotationGroupEditModal.vue'
import { useUptimeStore } from '@/stores/uptimeStore'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'

import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Switch } from '@/components/ui/switch'

interface Props {
  modelValue: boolean
  embedded?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const uptimeStore = useUptimeStore()
const confirm = useConfirm()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isSavingSettings = ref(false)

const form = ref({
  enabled: false,
  interval_minutes: 5,
  timeout_seconds: 8
})

watch(isOpen, async (open) => {
  if (open) {
    await uptimeStore.loadSnapshot()
    hydrateForm()
  }}, { immediate: true })

function hydrateForm() {
  form.value.enabled = uptimeStore.settings.enabled
  form.value.interval_minutes = Math.max(1, Math.round((uptimeStore.settings.interval_seconds || 300) / 60))
  form.value.timeout_seconds = uptimeStore.settings.timeout_seconds || 8
}

async function persistSettings(successMessage: string) {
  if (isSavingSettings.value) return
  isSavingSettings.value = true
  try {
    const intervalSeconds = Math.max(60, Number(form.value.interval_minutes) * 60)
    const timeoutSeconds = Math.max(1, Number(form.value.timeout_seconds))
    await uptimeStore.saveSettings({
      enabled: !!form.value.enabled,
      interval_seconds: intervalSeconds,
      timeout_seconds: timeoutSeconds,
      keep_last: uptimeStore.settings.keep_last || 10
    })
    toast.success(successMessage)
  } catch (e: any) {
    toast.error('保存失败: ' + (e?.message || String(e)))
    throw e
  } finally {
    isSavingSettings.value = false
  }
}

async function onEnabledChange(value: boolean) {
  const prev = form.value.enabled
  form.value.enabled = value
  try {
    await persistSettings(value ? '已开启监控' : '已关闭监控')
  } catch {
    form.value.enabled = prev
  }
}

async function saveSettings() {
  try {
    await persistSettings('设置已保存')
  } catch {
    /* persistSettings 已提示 */
  }
}

async function runNow() {
  try {
    await uptimeStore.runOnce()
    toast.success('检测已完成')
  } catch (e: any) {
    toast.error('检测失败: ' + (e?.message || String(e)))
  }
}

function providerLabel(p: string): string {
  const labels: Record<string, string> = { claude: 'Claude', codex: 'Codex', gemini: 'Gemini', opencode: 'OpenCode', grok: 'Grok' }
  return labels[p] || p
}

const showGroupModal = ref(false)
const editingGroup = ref<RotationGroup | null>(null)

function openCreate() {
  editingGroup.value = null
  showGroupModal.value = true
}

function openEdit(group: RotationGroup) {
  editingGroup.value = group
  showGroupModal.value = true
}

async function toggleGroup(group: RotationGroup) {
  try {
    await uptimeStore.saveGroup({ ...group, enabled: !group.enabled } as RotationGroup)
    toast.success(group.enabled ? '已停用' : '已启用')
  } catch (e: any) {
    toast.error('操作失败: ' + (e?.message || String(e)))
  }
}

async function remove(group: RotationGroup) {
  const ok = await confirm.show(
    '删除轮换组',
    `确定要删除 “${group.name}” 吗？`,
    'danger'
  )
  if (!ok) return

  try {
    await uptimeStore.deleteGroup(group.name)
    toast.success('轮换组已删除')
  } catch (e: any) {
    toast.error('删除失败: ' + (e?.message || String(e)))
  }
}

function onGroupSaved() {
  showGroupModal.value = false
}
</script>
