import type { RotationGroup, UptimeSettings, UptimeSnapshot } from '@/types'

export const uptimeService = {
  async getSnapshot(): Promise<UptimeSnapshot> {
    return window.go.main.UptimeService.GetSnapshot()
  },

  async saveSettings(settings: UptimeSettings): Promise<void> {
    return window.go.main.UptimeService.SaveSettings(settings)
  },

  async saveRotationGroup(group: RotationGroup): Promise<void> {
    return window.go.main.UptimeService.SaveRotationGroup(group)
  },

  async deleteRotationGroup(name: string): Promise<void> {
    return window.go.main.UptimeService.DeleteRotationGroup(name)
  },

  async runOnce(): Promise<UptimeSnapshot> {
    return window.go.main.UptimeService.RunOnce()
  }
}

