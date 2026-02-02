<template>
  <AppModal v-model="isOpen" size="xl" :close-on-overlay="false">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center">
          <i class="fas fa-heartbeat text-primary"></i>
        </div>
        <div>
          <h3 class="text-lg font-semibold">监控 & 轮换</h3>
          <p class="text-xs text-muted-foreground">每隔一段时间检测可达性，并按轮换组自动切换配置</p>
        </div>
      </div>
    </template>

    <!-- Settings -->
    <div class="p-4 rounded-xl border border-border bg-card/60 mb-4">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h4 class="text-sm font-bold uppercase tracking-wide">Uptime 监控</h4>
          <p class="text-xs text-muted-foreground mt-1">
            监控会对各配置的 Base URL 做 HTTP 可达性检测，并保留最近 {{ uptimeStore.settings.keep_last }} 次记录。
          </p>
        </div>
        <button
          class="btn btn-outline btn-sm"
          :disabled="uptimeStore.isRunning"
          @click="runNow"
        >
          <i :class="['fas mr-2', uptimeStore.isRunning ? 'fa-circle-notch fa-spin' : 'fa-bolt']"></i>
          立即检测
        </button>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mt-4">
        <div>
          <label class="block text-sm font-medium mb-1.5">启用监控</label>
          <label class="flex items-center gap-2 px-3 py-2 rounded-lg border border-border bg-secondary/30 text-xs font-medium w-fit">
            <input v-model="form.enabled" type="checkbox" />
            已启用
          </label>
        </div>

        <div>
          <label class="block text-sm font-medium mb-1.5">间隔（分钟）</label>
          <input v-model.number="form.interval_minutes" class="input" type="number" min="1" max="1440" />
          <p class="text-[11px] text-muted-foreground mt-1">默认 5 分钟</p>
        </div>

        <div>
          <label class="block text-sm font-medium mb-1.5">超时（秒）</label>
          <input v-model.number="form.timeout_seconds" class="input" type="number" min="1" max="60" />
          <p class="text-[11px] text-muted-foreground mt-1">建议 8-15 秒</p>
        </div>
      </div>

      <div class="flex items-center justify-end mt-4">
        <button class="btn btn-primary btn-sm" :disabled="isSavingSettings" @click="saveSettings">
          <i :class="['fas mr-2', isSavingSettings ? 'fa-circle-notch fa-spin' : 'fa-save']"></i>
          保存设置
        </button>
      </div>
    </div>

    <!-- Groups -->
    <div class="flex items-center justify-between mb-3">
      <div>
        <h4 class="text-sm font-bold uppercase tracking-wide">轮换组</h4>
        <p class="text-xs text-muted-foreground mt-1">
          当某组的“当前激活配置”连续失败达到阈值时，自动切换到组内下一个（优先挑选最近成功/未检测过的）。
        </p>
      </div>
      <button class="btn btn-primary btn-sm" @click="openCreate">
        <i class="fas fa-plus mr-2"></i>
        新建轮换组
      </button>
    </div>

    <div v-if="uptimeStore.isLoading" class="flex items-center justify-center py-12">
      <i class="fas fa-circle-notch fa-spin text-2xl text-muted-foreground"></i>
    </div>

    <div
      v-else-if="uptimeStore.groups.length === 0"
      class="flex flex-col items-center justify-center py-10 text-muted-foreground border border-dashed border-border rounded-xl"
    >
      <i class="fas fa-random text-3xl mb-3"></i>
      <p class="text-sm">暂无轮换组</p>
      <p class="text-xs">点击“新建轮换组”开始配置</p>
    </div>

    <div v-else class="space-y-3 max-h-[45vh] overflow-y-auto pr-2">
      <div
        v-for="group in uptimeStore.groups"
        :key="group.name"
        class="p-4 rounded-xl border border-border bg-card/60 hover:bg-card transition-colors"
      >
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h5 class="font-bold text-foreground truncate">{{ group.name }}</h5>
              <span class="text-[10px] px-2 py-0.5 rounded-full border border-border text-muted-foreground font-bold uppercase">
                {{ providerLabel(group.provider) }}
              </span>
              <span
                :class="['text-[10px] px-2 py-0.5 rounded-full font-bold uppercase', group.enabled ? 'bg-green-500/10 text-green-600' : 'bg-muted text-muted-foreground']"
              >
                {{ group.enabled ? 'Enabled' : 'Disabled' }}
              </span>
            </div>
            <p class="text-xs text-muted-foreground mt-1">
              连续失败阈值：<span class="font-mono">{{ group.failure_threshold }}</span>
            </p>
            <div class="flex flex-wrap gap-2 mt-3">
              <span
                v-for="name in group.env_names"
                :key="name"
                class="text-[10px] px-2 py-0.5 rounded-full border border-border bg-secondary/30 font-mono"
              >
                {{ name }}
              </span>
            </div>
          </div>

          <div class="flex items-center gap-2 flex-none">
            <button class="btn btn-outline btn-sm" @click="toggleGroup(group)">
              <i class="fas fa-power-off mr-2"></i>
              {{ group.enabled ? '停用' : '启用' }}
            </button>
            <button class="btn btn-outline btn-sm" @click="openEdit(group)">
              <i class="fas fa-pen mr-2"></i>
              编辑
            </button>
            <button
              class="btn btn-outline btn-sm border-destructive/50 text-destructive hover:bg-destructive hover:text-destructive-foreground"
              @click="remove(group)"
            >
              <i class="fas fa-trash mr-2"></i>
              删除
            </button>
          </div>
        </div>
      </div>
    </div>

    <RotationGroupEditModal v-model="showGroupModal" :edit-group="editingGroup" @saved="onGroupSaved" />
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { RotationGroup } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import RotationGroupEditModal from './RotationGroupEditModal.vue'
import { useUptimeStore } from '@/stores/uptimeStore'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'

interface Props {
  modelValue: boolean
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
  }
})

function hydrateForm() {
  form.value.enabled = uptimeStore.settings.enabled
  form.value.interval_minutes = Math.max(1, Math.round((uptimeStore.settings.interval_seconds || 300) / 60))
  form.value.timeout_seconds = uptimeStore.settings.timeout_seconds || 8
}

async function saveSettings() {
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
    toast.success('设置已保存')
  } catch (e: any) {
    toast.error('保存失败: ' + (e?.message || String(e)))
  } finally {
    isSavingSettings.value = false
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
  const labels: Record<string, string> = { claude: 'Claude', codex: 'Codex', gemini: 'Gemini' }
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

<style scoped>
.btn-sm {
  @apply h-8 px-3 text-xs;
}
</style>
