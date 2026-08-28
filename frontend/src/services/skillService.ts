import type { Skill, SkillMarketItem, SkillPreset } from '@/types'

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
  },

  async applyToPlatform(platform: string): Promise<number> {
    const count = await window.go.main.SkillService.ApplyToPlatform(platform)
    return typeof count === 'number' ? count : 0
  },

  async searchMarketplace(source: string, query: string): Promise<SkillMarketItem[]> {
    const list = await window.go.main.SkillService.SearchSkillMarketplace(source, query)
    return list || []
  },

  async importMarketplace(id: string): Promise<Skill> {
    return window.go.main.SkillService.ImportSkillMarketplace(id)
  }
}
