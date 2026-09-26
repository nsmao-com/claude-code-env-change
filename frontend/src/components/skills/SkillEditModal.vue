<template>
  <AppModal v-model="isOpen" :title="isEditing ? t('skills.edit.titleEdit') : t('skills.edit.titleNew')" size="xl" :close-on-overlay="false">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div class="grid gap-1.5">
          <Label>{{ t('skills.edit.name') }}</Label>
          <Input
            v-model="form.name"
            :placeholder="t('skills.edit.namePlaceholder')"
            :disabled="isEditing"
          />
          <p class="text-xs text-muted-foreground">
            {{ t('skills.edit.nameHintBefore') }} <code class="font-mono">a-z0-9-</code> {{ t('skills.edit.nameHintAfter') }}
          </p>
        </div>

        <div class="grid gap-1.5">
          <Label>{{ t('skills.edit.platforms') }}</Label>
          <ToggleGroup
            type="multiple"
            variant="outline"
            size="sm"
            :model-value="form.enable_platform"
            @update:model-value="onPlatforms"
          >
            <ToggleGroupItem value="claude-code">
              <BrandIcon provider="claude" class="size-3.5" />
              Claude
            </ToggleGroupItem>
            <ToggleGroupItem value="codex">
              <BrandIcon provider="codex" class="size-3.5" />
              Codex
            </ToggleGroupItem>
            <ToggleGroupItem value="antigravity">
              <BrandIcon provider="antigravity" class="size-3.5" />
              Antigravity
            </ToggleGroupItem>
            <ToggleGroupItem value="opencode">
              <BrandIcon provider="opencode" class="size-3.5" />
              OpenCode
            </ToggleGroupItem>
            <ToggleGroupItem value="grok">
              <BrandIcon provider="grok" class="size-3.5" />
              Grok
            </ToggleGroupItem>
          </ToggleGroup>
        </div>
      </div>

      <div class="flex items-center justify-between gap-3">
        <Label>SKILL.md</Label>
        <Button type="button" variant="outline" size="sm" @click="insertTemplate">
          <Sparkles />
          {{ t('skills.edit.insertTemplate') }}
        </Button>
      </div>

      <Textarea
        v-model="form.content"
        class="h-64 resize-y font-mono text-xs"
        :placeholder="t('skills.edit.contentPlaceholder')"
        spellcheck="false"
      />

      <div class="text-xs text-muted-foreground">
        {{ t('skills.edit.installPath') }}
        <span class="font-mono">~/.claude/skills/&lt;name&gt;/SKILL.md</span> /
        <span class="font-mono">~/.codex/skills/&lt;name&gt;/SKILL.md</span> /
        <span class="font-mono">~/.gemini/skills/&lt;name&gt;/SKILL.md</span> /
        <span class="font-mono">~/.config/opencode/skills/&lt;name&gt;/SKILL.md</span> /
        <span class="font-mono">~/.grok/skills/&lt;name&gt;/SKILL.md</span>
      </div>
    </form>

    <template #footer>
      <div class="flex items-center justify-between gap-3">
        <p class="flex items-center text-xs text-muted-foreground">
          <Info class="mr-1.5 size-3.5" />
          {{ t('skills.edit.restartHint') }}
        </p>
        <div class="flex items-center gap-3">
          <Button variant="secondary" @click="isOpen = false">{{ t('common.cancel') }}</Button>
          <Button :disabled="isSaving" @click="handleSubmit">
            <Loader2 v-if="isSaving" class="animate-spin" />
            <Save v-else />
            {{ isSaving ? t('skills.edit.saving') : t('common.save') }}
          </Button>
        </div>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { ref, computed, watch } from 'vue'
import { Info, Loader2, Save, Sparkles } from '@lucide/vue'
import type { Skill } from '@/types'
import AppModal from '@/components/common/AppModal.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { useSkillStore } from '@/stores/skillStore'
import { useConfigStore } from '@/stores/configStore'
import { useToast } from '@/composables/useToast'
import { toolToPlatform } from '@/lib/workspace'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

const { t } = useI18n()

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
const configStore = useConfigStore()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isEditing = computed(() => !!props.editSkill)
const isSaving = ref(false)

function defaultForm() {
  const tool = configStore.currentFilter
  const platform = tool === 'all' ? 'claude-code' : toolToPlatform(tool)
  return {
    name: '',
    enable_platform: platform === 'claude-desktop' ? [] as string[] : [platform],
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
  if (open) {
    if (!props.editSkill) form.value = defaultForm()
    return
  }
  form.value = defaultForm()
})

function onPlatforms(value: unknown) {
  form.value.enable_platform = Array.isArray(value)
    ? value.filter((v): v is string => typeof v === 'string')
    : []
}

function insertTemplate() {
  const name = (form.value.name || 'my-skill').trim() || 'my-skill'
  if (form.value.content.trim()) {
    toast.info(t('skills.edit.contentExists'))
    return
  }
  form.value.content = `---
name: ${name}
description: ${t('skills.edit.templateDescription')}
---

# ${name}

${t('skills.edit.templateBody')}`
}

async function handleSubmit() {
  if (isSaving.value) return

  const name = form.value.name.trim()
  if (!name) {
    toast.error(t('skills.edit.nameRequired'))
    return
  }
  if (!/^[a-z0-9][a-z0-9-]{0,63}$/.test(name)) {
    toast.error(t('skills.edit.nameInvalid'))
    return
  }
  if (!form.value.enable_platform || form.value.enable_platform.length === 0) {
    toast.error(t('skills.edit.platformRequired'))
    return
  }
  if (!form.value.content.trim()) {
    toast.error(t('skills.edit.contentRequired'))
    return
  }

  const payload: Skill = {
    files: props.editSkill?.files,
    executable: props.editSkill?.executable,
    source: props.editSkill?.source,
    name,
    content: form.value.content,
    enable_platform: [...form.value.enable_platform],
    enabled_in_claude: false,
    enabled_in_codex: false,
    enabled_in_antigravity: false,
    enabled_in_opencode: false,
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
    toast.success(t('skills.edit.saved'))
    isOpen.value = false
    emit('saved')
  } catch (e: any) {
    toast.error(t('skills.edit.saveFailed', { error: e?.message || String(e) }))
  } finally {
    isSaving.value = false
  }
}
</script>
