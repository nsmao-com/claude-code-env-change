import type { Skill, SkillPreset } from '@/types'

export const skillService = {
  async listSkills(): Promise<Skill[]> {
    const skills = await window.go.main.SkillService.ListSkills()
    return skills || []
  },

  async saveSkill(skill: Skill): Promise<void> {
    return window.go.main.SkillService.SaveSkill(skill)
  },

  async deleteSkill(name: string): Promise<void> {
    return window.go.main.SkillService.DeleteSkill(name)
  },

  async getPresets(): Promise<SkillPreset[]> {
    const presets = await window.go.main.SkillService.GetSkillPresets()
    return presets || []
  }
}
