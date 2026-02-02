<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑 Skill' : '新建 Skill'" size="xl" :close-on-overlay="false">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium mb-1.5">技能名称</label>
          <input
            v-model="form.name"
            class="input"
            placeholder="例如：code-reviewer"
            :disabled="isEditing"
          />
          <p class="text-[11px] text-muted-foreground mt-1">
            目录名 + /skill 命令名；建议使用 <code>a-z0-9-</code>（1-64）
          </p>
        </div>

        <div>
          <label class="block text-sm font-medium mb-1.5">启用平台</label>
          <div class="flex flex-wrap gap-2">
            <label class="flex items-center gap-2 px-3 py-2 rounded-lg border border-border bg-secondary/30 text-xs font-medium">
              <input v-model="form.enable_platform" type="checkbox" value="claude-code" />
              Claude
            </label>
            <label class="flex items-center gap-2 px-3 py-2 rounded-lg border border-border bg-secondary/30 text-xs font-medium">
              <input v-model="form.enable_platform" type="checkbox" value="codex" />
              Codex
            </label>
            <label class="flex items-center gap-2 px-3 py-2 rounded-lg border border-border bg-secondary/30 text-xs font-medium">
              <input v-model="form.enable_platform" type="checkbox" value="gemini" />
              Gemini
            </label>
          </div>
        </div>
      </div>

      <div class="flex items-center justify-between">
        <label class="block text-sm font-medium">SKILL.md</label>
        <button type="button" class="btn btn-outline btn-sm" @click="insertTemplate">
          <i class="fas fa-magic mr-2"></i>
          插入模板
        </button>
      </div>

      <textarea
        v-model="form.content"
        class="input h-64 font-mono text-xs"
        placeholder="请粘贴/编辑 SKILL.md 内容（需包含 --- frontmatter ---）"
        spellcheck="false"
      ></textarea>

      <div class="text-xs text-muted-foreground">
        安装位置：
        <span class="font-mono">~/.claude/skills/&lt;name&gt;/SKILL.md</span> /
        <span class="font-mono">~/.codex/skills/&lt;name&gt;/SKILL.md</span> /
        <span class="font-mono">~/.gemini/skills/&lt;name&gt;/SKILL.md</span>
      </div>
    </form>

    <template #footer>
      <div class="flex items-center justify-between">
        <p class="text-xs text-muted-foreground">
          <i class="fas fa-info-circle mr-1.5"></i>
          保存后建议重启对应 CLI 生效
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
import type { Skill } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import { useSkillStore } from '@/stores/skillStore'
import { useToast } from '@/composables/useToast'

interface Props {
  modelValue: boolean
  editSkill?: Skill | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const toast = useToast()
const skillStore = useSkillStore()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isEditing = computed(() => !!props.editSkill)
const isSaving = ref(false)

function defaultForm() {
  return {
    name: '',
    enable_platform: ['claude-code'] as string[],
    content: ''
  }
}

const form = ref(defaultForm())

watch(() => props.editSkill, (skill) => {
  if (skill) {
    form.value = {
      name: skill.name,
      enable_platform: [...(skill.enable_platform || [])],
      content: skill.content || ''
    }
  } else {
    form.value = defaultForm()
  }
}, { immediate: true })

watch(isOpen, (open) => {
  if (!open) {
    form.value = defaultForm()
  }
})

function insertTemplate() {
  const name = (form.value.name || 'my-skill').trim() || 'my-skill'
  if (form.value.content.trim()) {
    toast.info('SKILL.md 已有内容，未覆盖')
    return
  }
  form.value.content = `---
name: ${name}
description: 这里写这个 skill 做什么、何时使用（越具体越好）
---

# ${name}

在这里写你的 Skill 指令（步骤、约束、输出格式等）。`
}

async function handleSubmit() {
  if (isSaving.value) return

  const name = form.value.name.trim()
  if (!name) {
    toast.error('请输入技能名称')
    return
  }
  if (!/^[a-z0-9][a-z0-9-]{0,63}$/.test(name)) {
    toast.error('技能名称需为 a-z0-9- 且长度 1-64')
    return
  }
  if (!form.value.enable_platform || form.value.enable_platform.length === 0) {
    toast.error('请至少选择一个平台')
    return
  }
  if (!form.value.content.trim()) {
    toast.error('SKILL.md 内容不能为空')
    return
  }

  const payload: Skill = {
    name,
    content: form.value.content,
    enable_platform: [...form.value.enable_platform],
    enabled_in_claude: false,
    enabled_in_codex: false,
    enabled_in_gemini: false,
    frontmatter_name: '',
    description: '',
    has_frontmatter: false,
    has_name: false,
    has_description: false,
    frontmatter_error: ''
  }

  isSaving.value = true
  try {
    await skillStore.saveSkill(payload)
    toast.success('Skill 已保存')
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
.btn-sm {
  @apply h-8 px-3 text-xs;
}
</style>

