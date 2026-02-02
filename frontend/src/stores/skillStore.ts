import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Skill } from '@/types'
import { skillService } from '@/services/skillService'

export const useSkillStore = defineStore('skills', () => {
  const skills = ref<Skill[]>([])
  const isLoading = ref(false)

  const skillCount = computed(() => skills.value.length)

  const claudeCount = computed(() => skills.value.filter(s => s.enable_platform?.includes('claude-code')).length)
  const codexCount = computed(() => skills.value.filter(s => s.enable_platform?.includes('codex')).length)
  const geminiCount = computed(() => skills.value.filter(s => s.enable_platform?.includes('gemini')).length)

  async function loadSkills() {
    isLoading.value = true
    try {
      skills.value = await skillService.listSkills()
    } finally {
      isLoading.value = false
    }
  }

  async function saveSkill(skill: Skill) {
    await skillService.saveSkill(skill)
    await loadSkills()
  }

  async function deleteSkill(name: string) {
    await skillService.deleteSkill(name)
    await loadSkills()
  }

  return {
    skills,
    isLoading,
    skillCount,
    claudeCount,
    codexCount,
    geminiCount,
    loadSkills,
    saveSkill,
    deleteSkill
  }
})

