<template>
  <AppModal v-model="isOpen" :title="isEditing ? t('uptime.groupForm.titleEdit') : t('uptime.groupForm.titleNew')" size="xl" :close-on-overlay="false">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div class="grid grid-cols-1 gap-4">
        <AppInput v-model="form.name" :label="t('uptime.groupForm.name')" :placeholder="t('uptime.groupForm.namePlaceholder')" />
        <div class="grid gap-1.5">
          <Label>Provider</Label>
          <ToggleGroup
            type="single"
            variant="outline"
            class="grid w-full grid-cols-3 sm:grid-cols-5"
            :model-value="form.provider"
            @update:model-value="onProvider"
          >
            <ToggleGroupItem v-for="p in providers" :key="p.value" :value="p.value">
              <BrandIcon :provider="p.value" class="size-3.5" />
              {{ p.label }}
            </ToggleGroupItem>
          </ToggleGroup>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div class="grid gap-1.5">
          <Label>{{ t('uptime.groupForm.enable') }}</Label>
          <div class="flex items-center gap-2">
            <Switch :checked="form.enabled" @update:checked="onEnabledChange" />
            <span class="text-xs text-muted-foreground">{{ form.enabled ? t('uptime.groupForm.rotationOn') : t('uptime.groupForm.rotationOff') }}</span>
          </div>
        </div>
        <div class="grid gap-1.5">
          <Label>{{ t('uptime.groupForm.threshold') }}</Label>
          <Input v-model="form.failure_threshold" type="number" min="1" max="20" />
          <p class="text-[11px] text-muted-foreground">{{ t('uptime.groupForm.thresholdHint') }}</p>
        </div>
        <p class="text-xs leading-relaxed text-muted-foreground">
          {{ t('uptime.groupForm.noteBefore') }} <span class="font-mono">{{ t('uptime.groupForm.noteTag') }}</span> {{ t('uptime.groupForm.noteAfter') }}
        </p>
      </div>

      <div class="border-t pt-4">
        <div class="mb-2 flex items-center justify-between gap-2">
          <h4 class="text-sm font-medium">{{ t('uptime.groupForm.members') }}</h4>
          <span class="text-xs text-muted-foreground">{{ t('uptime.groupForm.count', { count: form.env_names.length }) }}</span>
        </div>

        <Empty v-if="form.env_names.length === 0" class="border border-dashed py-4">
          <EmptyHeader>
            <EmptyTitle class="text-sm">{{ t('uptime.groupForm.emptyTitle') }}</EmptyTitle>
            <EmptyDescription>{{ t('uptime.groupForm.emptyDesc') }}</EmptyDescription>
          </EmptyHeader>
        </Empty>

        <div v-else class="space-y-2">
          <div
            v-for="(name, idx) in form.env_names"
            :key="name"
            class="flex items-center justify-between gap-3 rounded-xl border p-3"
          >
            <div class="min-w-0">
              <div class="truncate font-mono text-sm font-medium">{{ idx + 1 }}. {{ name }}</div>
              <div class="truncate text-[11px] text-muted-foreground">{{ envDesc(name) }}</div>
            </div>
            <div class="flex shrink-0 gap-1">
              <Button type="button" variant="ghost" size="icon-sm" :title="t('uptime.groupForm.moveUp')" :disabled="idx === 0" @click="moveUp(idx)">
                <ArrowUp />
              </Button>
              <Button type="button" variant="ghost" size="icon-sm" :title="t('uptime.groupForm.moveDown')" :disabled="idx === form.env_names.length - 1" @click="moveDown(idx)">
                <ArrowDown />
              </Button>
              <Button type="button" variant="ghost" size="icon-sm" :title="t('uptime.groupForm.remove')" @click="removeAt(idx)">
                <X />
              </Button>
            </div>
          </div>
        </div>

        <div class="mt-4">
          <div class="mb-2 flex items-center justify-between gap-2">
            <h4 class="text-sm font-medium">{{ t('uptime.groupForm.available') }}</h4>
            <span class="text-xs text-muted-foreground">{{ t('uptime.groupForm.availableCount', { count: availableEnvs.length }) }}</span>
          </div>
          <div class="flex flex-wrap gap-2">
            <Button
              v-for="env in availableEnvs"
              :key="env.name"
              type="button"
              variant="outline"
              size="sm"
              class="h-auto py-2"
              @click="addEnv(env.name)"
            >
              <span class="font-mono">{{ env.name }}</span>
              <span v-if="env.description" class="text-muted-foreground">{{ env.description }}</span>
            </Button>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <p class="mr-auto text-xs text-muted-foreground">{{ t('uptime.groupForm.basis') }}</p>
      <Button variant="secondary" @click="isOpen = false">{{ t('common.cancel') }}</Button>
      <Button :disabled="isSaving" @click="handleSubmit">
        <Loader2 v-if="isSaving" class="animate-spin" />
        {{ isSaving ? t('uptime.groupForm.saving') : t('common.save') }}
      </Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { ref, computed, watch } from 'vue'
import { ArrowDown, ArrowUp, Loader2, X } from '@lucide/vue'
import type { EnvConfig, RotationGroup, Provider } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { useUptimeStore } from '@/stores/uptimeStore'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

const { t } = useI18n()

interface Props {
  modelValue: boolean
  editGroup?: RotationGroup | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const toast = useToast()
const uptimeStore = useUptimeStore()
const configStore = useConfigStore()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isEditing = computed(() => !!props.editGroup)
const isSaving = ref(false)

const providers = [
  { value: 'claude' as Provider, label: 'Claude Code' },
  // Claude Desktop 不参与轮换：后端轮换按 provider 切换激活配置并写回 CLI，
  // desktop 没有独立的切换入口，放进列表只会得到一个点了没反应的死选项
  { value: 'codex' as Provider, label: 'Codex' },
  { value: 'antigravity' as Provider, label: 'Antigravity' },
  { value: 'opencode' as Provider, label: 'OpenCode' },
  { value: 'grok' as Provider, label: 'Grok' },
]

function providerFromFilter(): Provider {
  return providers.find(item => item.value === configStore.currentFilter)?.value || 'claude'
}

function defaultForm(): RotationGroup {
  return {
    name: '',
    provider: providerFromFilter(),
    env_names: [],
    enabled: true,
    failure_threshold: 3
  }
}

const form = ref<RotationGroup>(defaultForm())

watch(() => props.editGroup, (group) => {
  if (group) {
    form.value = {
      name: group.name,
      provider: (group.provider || 'claude') as Provider,
      env_names: [...(group.env_names || [])],
      enabled: !!group.enabled,
      failure_threshold: group.failure_threshold || 3
    }
  } else {
    form.value = defaultForm()
  }
}, { immediate: true })

watch(isOpen, (open) => {
  if (open) {
    if (!props.editGroup) form.value = defaultForm()
    return
  }
  form.value = defaultForm()
})

const providerEnvs = computed<EnvConfig[]>(() => {
  return configStore.environments.filter(e => (e.provider || 'claude') === form.value.provider)
})

const availableEnvs = computed<EnvConfig[]>(() => {
  const selected = new Set(form.value.env_names)
  return providerEnvs.value.filter(e => !selected.has(e.name))
})

function envDesc(name: string): string {
  return providerEnvs.value.find(e => e.name === name)?.description || ''
}

function onEnabledChange(value: boolean) {
  form.value.enabled = value
}

function onProvider(value: unknown) {
  if (value === 'claude' || value === 'codex' || value === 'antigravity' || value === 'opencode' || value === 'grok') {
    switchProvider(value)
  }
}

function switchProvider(p: Provider) {
  if (form.value.provider === p) return
  form.value.provider = p
  form.value.env_names = []
}

function addEnv(name: string) {
  if (form.value.env_names.includes(name)) return
  form.value.env_names.push(name)
}

function removeAt(index: number) {
  form.value.env_names.splice(index, 1)
}

function moveUp(index: number) {
  if (index <= 0) return
  const arr = form.value.env_names
  ;[arr[index - 1], arr[index]] = [arr[index], arr[index - 1]]
}

function moveDown(index: number) {
  const arr = form.value.env_names
  if (index >= arr.length - 1) return
  ;[arr[index], arr[index + 1]] = [arr[index + 1], arr[index]]
}

async function handleSubmit() {
  if (isSaving.value) return

  const name = form.value.name.trim()
  if (!name) {
    toast.error(t('uptime.groupForm.nameRequired'))
    return
  }
  if (!form.value.provider) {
    toast.error(t('uptime.groupForm.providerRequired'))
    return
  }
  if (!form.value.env_names || form.value.env_names.length === 0) {
    toast.error(t('uptime.groupForm.envRequired'))
    return
  }
  if (!form.value.failure_threshold || form.value.failure_threshold < 1) {
    toast.error(t('uptime.groupForm.thresholdInvalid'))
    return
  }

  isSaving.value = true
  try {
    await uptimeStore.saveGroup({
      name,
      provider: form.value.provider,
      env_names: [...form.value.env_names],
      enabled: !!form.value.enabled,
      failure_threshold: form.value.failure_threshold
    })
    toast.success(t('uptime.groupForm.saved'))
    isOpen.value = false
    emit('saved')
  } catch (e: any) {
    toast.error(t('uptime.saveFailed', { error: e?.message || String(e) }))
  } finally {
    isSaving.value = false
  }
}
</script>
