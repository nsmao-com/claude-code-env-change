<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑轮换组' : '新建轮换组'" size="xl" :close-on-overlay="false">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium mb-1.5">组名称</label>
          <input v-model="form.name" class="input" placeholder="例如：claude-failover" />
        </div>

        <div class="flex items-end justify-between gap-4">
          <div class="flex-1">
            <label class="block text-sm font-medium mb-1.5">Provider</label>
            <div class="provider-tabs">
              <button
                v-for="p in providers"
                :key="p.value"
                type="button"
                :class="['provider-tab', { active: form.provider === p.value }]"
                @click="switchProvider(p.value)"
              >
                <i :class="[p.icon, 'mr-2']"></i>
                {{ p.label }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div>
          <label class="block text-sm font-medium mb-1.5">启用</label>
          <label class="flex items-center gap-2 px-3 py-2 rounded-lg border border-border bg-secondary/30 text-xs font-medium w-fit">
            <input v-model="form.enabled" type="checkbox" />
            已启用轮换
          </label>
        </div>
        <div>
          <label class="block text-sm font-medium mb-1.5">失败阈值</label>
          <input v-model.number="form.failure_threshold" class="input" type="number" min="1" max="20" />
          <p class="text-[11px] text-muted-foreground mt-1">连续失败达到该次数才会切换到下一个配置</p>
        </div>
        <div>
          <label class="block text-sm font-medium mb-1.5">说明</label>
          <p class="text-xs text-muted-foreground leading-relaxed">
            轮换组只在 <span class="font-mono">监控失败</span> 连续达到阈值时触发；切换后会执行一次配置应用。
          </p>
        </div>
      </div>

      <div class="border-t border-border pt-4">
        <div class="flex items-center justify-between mb-2">
          <h4 class="text-sm font-bold uppercase tracking-wide">组内配置（顺序）</h4>
          <span class="text-xs text-muted-foreground">共 {{ form.env_names.length }} 个</span>
        </div>

        <div v-if="form.env_names.length === 0" class="p-4 rounded-xl border border-dashed border-border text-xs text-muted-foreground">
          还没有添加配置。点击下方可用配置来加入轮换组。
        </div>

        <div v-else class="space-y-2">
          <div
            v-for="(name, idx) in form.env_names"
            :key="name"
            class="flex items-center justify-between gap-3 p-3 rounded-xl border border-border bg-card/60"
          >
            <div class="min-w-0">
              <div class="font-mono text-sm font-bold truncate">{{ idx + 1 }}. {{ name }}</div>
              <div class="text-[11px] text-muted-foreground truncate">
                {{ envDesc(name) }}
              </div>
            </div>

            <div class="flex items-center gap-1 flex-none">
              <button
                type="button"
                class="w-8 h-8 rounded border border-transparent hover:border-border flex items-center justify-center text-muted-foreground hover:text-foreground transition-all"
                title="上移"
                :disabled="idx === 0"
                @click="moveUp(idx)"
              >
                <i class="fas fa-arrow-up text-xs"></i>
              </button>
              <button
                type="button"
                class="w-8 h-8 rounded border border-transparent hover:border-border flex items-center justify-center text-muted-foreground hover:text-foreground transition-all"
                title="下移"
                :disabled="idx === form.env_names.length - 1"
                @click="moveDown(idx)"
              >
                <i class="fas fa-arrow-down text-xs"></i>
              </button>
              <button
                type="button"
                class="w-8 h-8 rounded border border-transparent hover:border-destructive hover:bg-destructive hover:text-destructive-foreground flex items-center justify-center text-muted-foreground transition-all"
                title="移除"
                @click="removeAt(idx)"
              >
                <i class="fas fa-times text-xs"></i>
              </button>
            </div>
          </div>
        </div>

        <div class="mt-4">
          <div class="flex items-center justify-between mb-2">
            <h4 class="text-sm font-bold uppercase tracking-wide">可用配置</h4>
            <span class="text-xs text-muted-foreground">{{ availableEnvs.length }} 个</span>
          </div>

          <div class="flex flex-wrap gap-2">
            <button
              v-for="env in availableEnvs"
              :key="env.name"
              type="button"
              class="px-3 py-2 rounded-lg border border-border bg-secondary/30 text-xs font-medium hover:bg-secondary/60 transition-colors"
              @click="addEnv(env.name)"
            >
              <span class="font-mono">{{ env.name }}</span>
              <span v-if="env.description" class="ml-2 text-muted-foreground">{{ env.description }}</span>
            </button>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex items-center justify-between">
        <p class="text-xs text-muted-foreground">
          <i class="fas fa-info-circle mr-1.5"></i>
          轮换依据：监控结果（HTTP 可达性）
        </p>
        <div class="flex items-center gap-3">
          <button class="btn btn-secondary h-9 px-5" @click="isOpen = false">取消</button>
          <button class="btn btn-primary h-9 px-5" :disabled="isSaving" @click="handleSubmit">
            <i :class="['fas mr-2', isSaving ? 'fa-circle-notch fa-spin' : 'fa-save']"></i>
            {{ isSaving ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { EnvConfig, RotationGroup, Provider } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import { useUptimeStore } from '@/stores/uptimeStore'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'

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
  { value: 'claude' as Provider, label: 'Claude', icon: 'fas fa-brain' },
  { value: 'codex' as Provider, label: 'Codex', icon: 'fas fa-code' },
  { value: 'gemini' as Provider, label: 'Gemini', icon: 'fas fa-gem' }
]

function defaultForm(): RotationGroup {
  return {
    name: '',
    provider: 'claude',
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
  if (!open) form.value = defaultForm()
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

<style scoped>
.provider-tabs {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 6px;
}

.provider-tab {
  @apply h-9 px-3 rounded-lg border border-border bg-secondary/30 text-xs font-bold uppercase tracking-wide transition-colors;
}

.provider-tab.active {
  @apply bg-foreground text-background border-foreground;
}
</style>

