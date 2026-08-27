<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑 Skill' : '新建 Skill'" size="xl" :close-on-overlay="false">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div class="grid gap-1.5">
          <Label>技能名称</Label>
          <Input
            v-model="form.name"
            placeholder="例如：code-reviewer"
            :disabled="isEditing"
          />
          <p class="text-xs text-muted-foreground">
            目录名 + /skill 命令名；建议使用 <code class="font-mono">a-z0-9-</code>（1-64）
          </p>
        </div>

        <div class="grid gap-1.5">
          <Label>启用平台</Label>
          <ToggleGroup
            type="multiple"
            variant="outline"
            size="sm"
            :model-value="form.enable_platform"
            @update:model-value="onPlatforms"
          >
            <ToggleGroupItem value="claude-code">Claude</ToggleGroupItem>
            <ToggleGroupItem value="codex">Codex</ToggleGroupItem>
            <ToggleGroupItem value="gemini">Gemini</ToggleGroupItem>
            <ToggleGroupItem value="openclaw">OpenClaw</ToggleGroupItem>
            <ToggleGroupItem value="grok">Grok</ToggleGroupItem>
          </ToggleGroup>
        </div>
      </div>

      <div class="flex items-center justify-between">
        <Label>SKILL.md</Label>
        <Button type="button" variant="outline" size="sm" @click="insertTemplate">
          <Sparkles />
          插入模板
        </Button>
      </div>

      <Textarea
        v-model="form.content"
        class="h-64 resize-y font-mono text-xs"
        placeholder="请粘贴/编辑 SKILL.md 内容（需包含 --- frontmatter ---）"
        spellcheck="false"
      />

      <div class="text-xs text-muted-foreground">
        安装位置：
        <span class="font-mono">~/.claude/skills/&lt;name&gt;/SKILL.md</span> /
        <span class="font-mono">~/.codex/skills/&lt;name&gt;/SKILL.md</span> /
        <span class="font-mono">~/.gemini/skills/&lt;name&gt;/SKILL.md</span> /
        <span class="font-mono">~/.openclaw/skills/&lt;name&gt;/SKILL.md</span> /
        <span class="font-mono">~/.grok/skills/&lt;name&gt;/SKILL.md</span>
      </div>
    </form>

    <template #footer>
      <div class="flex items-center justify-between">
        <p class="flex items-center text-xs text-muted-foreground">
          <Info class="mr-1.5 size-3.5" />
          保存后建议重启对应 CLI 生效
        </p>
        <div class="flex items-center gap-3">
          <Button variant="secondary" @click="isOpen = false">取消</Button>
          <Button :disabled="isSaving" @click="handleSubmit">
            <Loader2 v-if="isSaving" class="animate-spin" />
            <Save v-else />
            {{ isSaving ? '保存中...' : '保存' }}
          </Button>
        </div>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Info, Loader2, Save, Sparkles } from '@lucide/vue'
import type { Skill } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import { useSkillStore } from '@/stores/skillStore'
import { useToast } from '@/composables/useToast'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

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

function onPlatforms(value: unknown) {
  form.value.enable_platform = Array.isArray(value)
    ? value.filter((v): v is string => typeof v === 'string')
    : []
}

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
    enabled_in_openclaw: false,
    enabled_in_grok: false,
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
