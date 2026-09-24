<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" width="form" :close-on-overlay="false">
    <template #header>
      <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">{{ t('nav.uptime') }}</h1>
      <p class="mt-2 text-sm text-muted-foreground">{{ t('uptime.panelHint') }}</p>
    </template>

    <Card class="mb-4">
      <CardHeader>
        <div class="flex items-start justify-between gap-4">
          <div>
            <CardTitle>{{ t('uptime.title') }}</CardTitle>
            <CardDescription>
              {{ t(form.probe_auth ? 'uptime.descAuth' : 'uptime.descReach', { count: uptimeStore.settings.keep_last }) }}
            </CardDescription>
            <p
              v-if="uptimeStore.snapshot?.last_rotation_error"
              class="mt-2 rounded-md bg-destructive/10 px-2.5 py-1.5 text-xs text-destructive"
            >
              {{ t('uptime.rotationFailed', { error: uptimeStore.snapshot.last_rotation_error }) }}
            </p>
            <p
              v-else-if="uptimeStore.snapshot?.last_rotation"
              class="mt-2 rounded-md bg-emerald-500/10 px-2.5 py-1.5 text-xs text-emerald-600 dark:text-emerald-400"
            >
              {{ uptimeStore.snapshot.last_rotation }}
            </p>
          </div>
          <Button variant="outline" size="sm" :disabled="uptimeStore.isRunning" @click="runNow">
            <Loader2 v-if="uptimeStore.isRunning" class="animate-spin" />
            <Zap v-else />
            {{ t('uptime.runNow') }}
          </Button>
        </div>
      </CardHeader>
      <CardContent class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div class="grid gap-1.5">
          <Label>{{ t('uptime.enable') }}</Label>
          <div class="flex items-center gap-2">
            <Switch :checked="form.enabled" :disabled="isSavingSettings" @update:checked="onEnabledChange" />
            <span class="text-xs text-muted-foreground">{{ form.enabled ? t('uptime.enabled') : t('uptime.disabled') }}</span>
          </div>
        </div>
        <div class="grid gap-1.5">
          <Label>{{ t('uptime.interval') }}</Label>
          <Input v-model="form.interval_minutes" type="number" min="1" max="1440" />
          <p class="text-[11px] text-muted-foreground">{{ t('uptime.intervalHint') }}</p>
        </div>
        <div class="grid gap-1.5">
          <Label>{{ t('uptime.timeout') }}</Label>
          <Input v-model="form.timeout_seconds" type="number" min="1" max="60" />
          <p class="text-[11px] text-muted-foreground">{{ t('uptime.timeoutHint') }}</p>
        </div>
        <div class="grid gap-1.5 sm:col-span-3">
          <Label>{{ t('uptime.probeAuth') }}</Label>
          <div class="flex items-center gap-2">
            <Switch :checked="form.probe_auth" :disabled="isSavingSettings" @update:checked="onProbeAuthChange" />
            <span class="text-xs text-muted-foreground">
              {{ form.probe_auth ? t('uptime.probeAuthOn') : t('uptime.probeAuthOff') }}
            </span>
          </div>
          <p class="text-[11px] text-muted-foreground">{{ t('uptime.probeAuthHint') }}</p>
        </div>
      </CardContent>
      <CardFooter class="justify-end">
        <Button size="sm" :disabled="isSavingSettings" @click="saveSettings">
          <Loader2 v-if="isSavingSettings" class="animate-spin" />
          <Save v-else />
          {{ t('uptime.saveSettings') }}
        </Button>
      </CardFooter>
    </Card>

    <div class="mb-4 flex items-center justify-between gap-4">
      <div>
        <h4 class="text-sm font-medium">{{ t('uptime.groups') }}</h4>
        <p class="mt-1 text-xs text-muted-foreground">
          {{ t('uptime.groupsDesc') }}
        </p>
      </div>
      <Button size="sm" @click="openCreate">
        <Plus />
        {{ t('uptime.newGroup') }}
      </Button>
    </div>

    <div v-if="uptimeStore.isLoading" class="flex items-center justify-center py-12">
      <Loader2 class="size-6 animate-spin text-muted-foreground" />
    </div>

    <Empty v-else-if="uptimeStore.groups.length === 0" class="border border-dashed py-10">
      <EmptyHeader>
        <Shuffle class="size-8 text-muted-foreground" />
        <EmptyTitle>{{ t('uptime.emptyTitle') }}</EmptyTitle>
        <EmptyDescription>{{ t('uptime.emptyDesc') }}</EmptyDescription>
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
                    {{ group.enabled ? t('uptime.groupEnabled') : t('uptime.groupDisabled') }}
                  </Badge>
                </div>
                <CardDescription>
                  {{ t('uptime.threshold') }}<span class="font-mono">{{ group.failure_threshold }}</span>
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
                  {{ group.enabled ? t('uptime.deactivate') : t('uptime.activate') }}
                </Button>
                <Button variant="outline" size="sm" @click="openEdit(group)">
                  <Pencil />
                  {{ t('uptime.edit') }}
                </Button>
                <Button variant="destructive" size="sm" @click="remove(group)">
                  <Trash2 />
                  {{ t('uptime.delete') }}
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
import { useI18n } from '@/composables/useI18n'
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

const { t } = useI18n()

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
  timeout_seconds: 8,
  probe_auth: false
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
  form.value.probe_auth = uptimeStore.settings.probe_mode === 'auth'
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
      keep_last: uptimeStore.settings.keep_last || 10,
      probe_mode: form.value.probe_auth ? 'auth' : 'reachability'
    })
    toast.success(successMessage)
  } catch (e: any) {
    toast.error(t('uptime.saveFailed', { error: e?.message || String(e) }))
    throw e
  } finally {
    isSavingSettings.value = false
  }
}

async function onEnabledChange(value: boolean) {
  const prev = form.value.enabled
  form.value.enabled = value
  try {
    await persistSettings(value ? t('uptime.monitorOn') : t('uptime.monitorOff'))
  } catch {
    form.value.enabled = prev
  }
}

async function onProbeAuthChange(value: boolean) {
  const prev = form.value.probe_auth
  form.value.probe_auth = value
  try {
    await persistSettings(value ? t('uptime.probeOnToast') : t('uptime.probeOffToast'))
  } catch {
    form.value.probe_auth = prev
  }
}

async function saveSettings() {
  try {
    await persistSettings(t('uptime.settingsSaved'))
  } catch {
    /* persistSettings 已提示 */
  }
}

async function runNow() {
  try {
    await uptimeStore.runOnce()
    toast.success(t('uptime.runDone'))
  } catch (e: any) {
    toast.error(t('uptime.runFailed', { error: e?.message || String(e) }))
  }
}

function providerLabel(p: string): string {
  const labels: Record<string, string> = { claude: 'Claude Code', claude_desktop: 'Claude Desktop', codex: 'Codex', antigravity: 'Antigravity', opencode: 'OpenCode', grok: 'Grok' }
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
    toast.success(group.enabled ? t('uptime.toggledOff') : t('uptime.toggledOn'))
  } catch (e: any) {
    toast.error(t('uptime.opFailed', { error: e?.message || String(e) }))
  }
}

async function remove(group: RotationGroup) {
  const ok = await confirm.show(
    t('uptime.deleteTitle'),
    t('uptime.deleteMsg', { name: group.name }),
    'danger'
  )
  if (!ok) return

  try {
    await uptimeStore.deleteGroup(group.name)
    toast.success(t('uptime.deleted'))
  } catch (e: any) {
    toast.error(t('uptime.deleteFailed', { error: e?.message || String(e) }))
  }
}

function onGroupSaved() {
  showGroupModal.value = false
}
</script>
