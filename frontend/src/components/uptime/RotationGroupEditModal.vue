<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑轮换组' : '新建轮换组'" size="xl" :close-on-overlay="false">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <AppInput v-model="form.name" label="组名称" placeholder="例如：claude-failover" />
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
          <Label>启用</Label>
          <div class="flex items-center gap-2">
            <Switch :checked="form.enabled" @update:checked="onEnabledChange" />
            <span class="text-xs text-muted-foreground">{{ form.enabled ? '已启用轮换' : '未启用轮换' }}</span>
          </div>
        </div>
        <div class="grid gap-1.5">
          <Label>失败阈值</Label>
          <Input v-model="form.failure_threshold" type="number" min="1" max="20" />
          <p class="text-[11px] text-muted-foreground">连续失败达到该次数才会切换到下一个配置</p>
        </div>
        <p class="text-xs leading-relaxed text-muted-foreground">
          轮换组只在 <span class="font-mono">监控失败</span> 连续达到阈值时触发；切换后会执行一次配置应用。
        </p>
      </div>

      <div class="border-t pt-4">
        <div class="mb-2 flex items-center justify-between gap-2">
          <h4 class="text-sm font-medium">组内配置（顺序）</h4>
          <span class="text-xs text-muted-foreground">共 {{ form.env_names.length }} 个</span>
        </div>

        <Empty v-if="form.env_names.length === 0" class="border border-dashed py-4">
          <EmptyHeader>
            <EmptyTitle class="text-sm">还没有添加配置</EmptyTitle>
            <EmptyDescription>点击下方可用配置来加入轮换组。</EmptyDescription>
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
              <Button type="button" variant="ghost" size="icon-sm" title="上移" :disabled="idx === 0" @click="moveUp(idx)">
                <ArrowUp />
              </Button>
              <Button type="button" variant="ghost" size="icon-sm" title="下移" :disabled="idx === form.env_names.length - 1" @click="moveDown(idx)">
                <ArrowDown />
              </Button>
              <Button type="button" variant="ghost" size="icon-sm" title="移除" @click="removeAt(idx)">
                <X />
              </Button>
            </div>
          </div>
        </div>

        <div class="mt-4">
          <div class="mb-2 flex items-center justify-between gap-2">
            <h4 class="text-sm font-medium">可用配置</h4>
            <span class="text-xs text-muted-foreground">{{ availableEnvs.length }} 个</span>
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
      <p class="mr-auto text-xs text-muted-foreground">轮换依据：监控结果（HTTP 可达性）</p>
      <Button variant="secondary" @click="isOpen = false">取消</Button>
      <Button :disabled="isSaving" @click="handleSubmit">
        <Loader2 v-if="isSaving" class="animate-spin" />
        {{ isSaving ? '保存中...' : '保存' }}
      </Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
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
  { value: 'claude' as Provider, label: 'Claude' },
  { value: 'codex' as Provider, label: 'Codex' },
  { value: 'antigravity' as Provider, label: 'Antigravity' },
  { value: 'opencode' as Provider, label: 'OpenCode' },
  { value: 'grok' as Provider, label: 'Grok' },
]

function providerFromFilter(): Provider {
  const filter = configStore.currentFilter
  if (filter === 'codex' || filter === 'antigravity' || filter === 'opencode' || filter === 'grok') return filter
  return 'claude'
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
  toast.success(value ? '已启用轮换' : '已停用轮换')
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
    toast.error('请输入轮换组名称')
    return
  }
  if (!form.value.provider) {
    toast.error('请选择 Provider')
    return
  }
  if (!form.value.env_names || form.value.env_names.length === 0) {
    toast.error('请至少添加 1 个配置')
    return
  }
  if (!form.value.failure_threshold || form.value.failure_threshold < 1) {
    toast.error('失败阈值必须 >= 1')
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
    toast.success('轮换组已保存')
    isOpen.value = false
    emit('saved')
  } catch (e: any) {
    toast.error('保存失败: ' + (e?.message || String(e)))
  } finally {
    isSaving.value = false
  }
}
</script>
