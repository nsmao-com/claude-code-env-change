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
  const antigravityCount = computed(() => skills.value.filter(s => s.enable_platform?.includes('antigravity')).length)
  const opencodeCount = computed(() => skills.value.filter(s => s.enable_platform?.includes('opencode')).length)
  const grokCount = computed(() => skills.value.filter(s => s.enable_platform?.includes('grok')).length)

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

  async function togglePlatform(skill: Skill, platform: string) {
    const enabled = skill.enable_platform || []
    const next = enabled.includes(platform)
      ? enabled.filter(item => item !== platform)
      : [...enabled, platform]
    await saveSkill({ ...skill, enable_platform: next })
  }

  async function applyToPlatform(platform: string): Promise<number> {
    const added = await skillService.applyToPlatform(platform)
    await loadSkills()
    return added
  }

  return {
    skills,
    isLoading,
    skillCount,
    claudeCount,
    codexCount,
    antigravityCount,
    opencodeCount,
    grokCount,
    loadSkills,
    saveSkill,
    deleteSkill,
    togglePlatform,
    applyToPlatform,
  }
})
