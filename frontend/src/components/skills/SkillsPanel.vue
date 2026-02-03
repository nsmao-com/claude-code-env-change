<template>
  <AppModal v-model="isOpen" size="xl" :close-on-overlay="false">
    <template #header>
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center">
          <i class="fas fa-layer-group text-primary"></i>
        </div>
        <div>
          <h3 class="text-lg font-semibold">Skills 管理</h3>
          <p class="text-xs text-muted-foreground">管理 Claude/Codex/Gemini 的自定义 SKILL.md</p>
        </div>
      </div>
    </template>

    <!-- Platform Filter Tabs -->
    <div class="relative mb-4 p-1 bg-secondary/50 border border-border/50 rounded-full inline-flex backdrop-blur-sm">
      <div
        class="absolute top-1 bottom-1 bg-white rounded-full transition-all duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] shadow-[0_2px_8px_rgba(0,0,0,0.08)] border border-black/5 dark:border-white/10"
        :style="gliderStyle"
      ></div>

      <button
        v-for="tab in platformTabs"
        :key="tab.value"
        ref="tabRefs"
        :class="['relative z-10 px-4 py-1.5 rounded-full text-xs font-bold uppercase tracking-wide transition-colors duration-200', { 'text-foreground dark:text-gray-900': currentPlatform === tab.value, 'text-muted-foreground hover:text-foreground/80': currentPlatform !== tab.value }]"
        @click="setFilter(tab.value)"
      >
        {{ tab.label }}
        <span v-if="tab.count > 0" class="ml-1 text-[10px] opacity-70">({{ tab.count }})</span>
      </button>
    </div>

    <!-- Toolbar -->
    <div class="flex items-center justify-between mb-4">
      <div class="flex gap-2">
        <button class="btn btn-primary btn-sm" @click="openCreate">
          <i class="fas fa-plus mr-2"></i>
          新建
        </button>
        <button class="btn btn-outline btn-sm" @click="skillStore.loadSkills">
          <i class="fas fa-sync-alt mr-2"></i>
          刷新
        </button>
      </div>
      <span class="text-xs text-muted-foreground">
        共 {{ skillStore.skillCount }} 个
      </span>
    </div>

    <!-- Empty State -->
    <div
      v-if="filteredSkills.length === 0 && !skillStore.isLoading"
      class="flex flex-col items-center justify-center py-12 text-muted-foreground"
    >
      <i class="fas fa-layer-group text-4xl mb-4"></i>
      <p class="text-sm">{{ skillStore.skillCount === 0 ? '暂无 Skills' : '该平台暂无 Skills' }}</p>
      <p class="text-xs">点击“新建”添加一个自定义 Skill</p>
    </div>

    <!-- Loading -->
    <div v-else-if="skillStore.isLoading" class="flex items-center justify-center py-12">
      <i class="fas fa-circle-notch fa-spin text-2xl text-muted-foreground"></i>
    </div>

    <!-- Skill List -->
    <div v-else class="space-y-3 max-h-[50vh] overflow-y-auto pr-2">
      <div
        v-for="skill in filteredSkills"
        :key="skill.name"
        class="p-4 rounded-xl border border-border bg-card/60 hover:bg-card transition-colors"
      >
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h4 class="font-bold text-foreground truncate">{{ skill.name }}</h4>
              <span
                v-if="!skill.has_frontmatter || !skill.has_name || !skill.has_description"
                class="text-[10px] px-2 py-0.5 rounded-full bg-red-500/10 text-red-600 font-bold uppercase"
                title="SKILL.md frontmatter 可能不完整"
              >
                格式问题
              </span>
            </div>
            <p class="text-xs text-muted-foreground mt-1 whitespace-pre-line">
              {{ skill.description || skill.frontmatter_error || '（未提供 description）' }}
            </p>

            <div class="flex flex-wrap gap-2 mt-3">
              <span class="text-[10px] px-2 py-0.5 rounded-full border border-border text-muted-foreground">
                Claude: <span :class="skill.enabled_in_claude ? 'text-green-600' : 'text-muted-foreground'">{{ skill.enabled_in_claude ? '已安装' : '未安装' }}</span>
              </span>
              <span class="text-[10px] px-2 py-0.5 rounded-full border border-border text-muted-foreground">
                Codex: <span :class="skill.enabled_in_codex ? 'text-green-600' : 'text-muted-foreground'">{{ skill.enabled_in_codex ? '已安装' : '未安装' }}</span>
              </span>
              <span class="text-[10px] px-2 py-0.5 rounded-full border border-border text-muted-foreground">
                Gemini: <span :class="skill.enabled_in_gemini ? 'text-green-600' : 'text-muted-foreground'">{{ skill.enabled_in_gemini ? '已安装' : '未安装' }}</span>
              </span>
            </div>
          </div>

          <div class="flex items-center gap-2 flex-none">
            <button class="btn btn-outline btn-sm" @click="openEdit(skill)">
              <i class="fas fa-pen mr-2"></i>
              编辑
            </button>
            <button class="btn btn-outline btn-sm border-destructive/50 text-destructive hover:bg-destructive hover:text-destructive-foreground" @click="remove(skill)">
              <i class="fas fa-trash mr-2"></i>
              删除
            </button>
          </div>
        </div>
      </div>
    </div>

    <SkillEditModal v-model="showEditModal" :edit-skill="editingSkill" @saved="onSaved" />
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import type { Skill } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import SkillEditModal from './SkillEditModal.vue'
import { useSkillStore } from '@/stores/skillStore'
import { useConfirm } from '@/composables/useConfirm'
import { useToast } from '@/composables/useToast'

type PlatformFilter = 'all' | 'claude-code' | 'codex' | 'gemini'

interface Props {
  modelValue: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const toast = useToast()
const confirm = useConfirm()
const skillStore = useSkillStore()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

// Platform filter
const currentPlatform = ref<PlatformFilter>('all')
const tabRefs = ref<HTMLElement[]>([])
const gliderStyle = ref({ left: '4px', width: '0px' })

const platformTabs = computed(() => [
  { label: '全部', value: 'all' as PlatformFilter, count: skillStore.skillCount },
  { label: 'Claude', value: 'claude-code' as PlatformFilter, count: skillStore.claudeCount },
  { label: 'Codex', value: 'codex' as PlatformFilter, count: skillStore.codexCount },
  { label: 'Gemini', value: 'gemini' as PlatformFilter, count: skillStore.geminiCount }
])

const filteredSkills = computed(() => {
  if (currentPlatform.value === 'all') return skillStore.skills
  return skillStore.skills.filter(s => s.enable_platform?.includes(currentPlatform.value))
})

function updateGlider() {
  nextTick(() => {
    const activeIndex = platformTabs.value.findIndex(t => t.value === currentPlatform.value)
    if (activeIndex !== -1 && tabRefs.value[activeIndex]) {
      const el = tabRefs.value[activeIndex]
      gliderStyle.value = {
        left: `${el.offsetLeft}px`,
        width: `${el.offsetWidth}px`
      }
    }
  })
}

function setFilter(platform: PlatformFilter) {
  currentPlatform.value = platform
  updateGlider()
}

watch(currentPlatform, () => updateGlider())

// Open/close
watch(isOpen, async (open) => {
  if (open) {
    await skillStore.loadSkills()
    setTimeout(updateGlider, 100)
  } else {
    currentPlatform.value = 'all'
  }
})

const showEditModal = ref(false)
const editingSkill = ref<Skill | null>(null)

function openCreate() {
  editingSkill.value = null
  showEditModal.value = true
}

function openEdit(skill: Skill) {
  editingSkill.value = skill
  showEditModal.value = true
}

async function remove(skill: Skill) {
  const ok = await confirm.show(
    '删除 Skill',
    `确定要删除 “${skill.name}” 吗？将从已安装的平台移除 SKILL.md（不会强制删除目录内的其他文件）。`,
    'danger'
  )
  if (!ok) return
  try {
    await skillStore.deleteSkill(skill.name)
    toast.success('Skill 已删除')
  } catch (e: any) {
    toast.error('删除失败: ' + (e?.message || String(e)))
  }
}

function onSaved() {
  // 保存后 store 会自动 reload
  showEditModal.value = false
}
</script>

<style scoped>
.btn-sm {
  @apply h-8 px-3 text-xs;
}
</style>

