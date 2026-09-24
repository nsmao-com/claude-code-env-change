import type { CloudConfig, CloudSyncResult, CloudSyncStatus, CloudVersion } from '@/types'

export const cloudService = {
  async getConfig(): Promise<CloudConfig> {
    return window.go.main.CloudSyncService.GetCloudConfig()
  },

  async saveConfig(config: CloudConfig): Promise<void> {
    return window.go.main.CloudSyncService.SaveCloudConfig(config)
  },

  async getStatus(): Promise<CloudSyncStatus> {
    return window.go.main.CloudSyncService.GetCloudSyncStatus()
  },

  async testConnection(): Promise<CloudSyncResult> {
    return window.go.main.CloudSyncService.TestCloudConnection()
  },

  async upload(): Promise<CloudSyncResult> {
    return window.go.main.CloudSyncService.UploadToCloud()
  },

  async forceUpload(): Promise<CloudSyncResult> {
    return window.go.main.CloudSyncService.ForceUploadToCloud()
  },

  async download(): Promise<CloudSyncResult> {
    return window.go.main.CloudSyncService.DownloadFromCloud()
  },

  async listVersions(): Promise<CloudVersion[]> {
    return (await window.go.main.CloudSyncService.ListCloudVersions()) || []
  },

  async restoreVersion(key: string): Promise<CloudSyncResult> {
    return window.go.main.CloudSyncService.RestoreCloudVersion(key)
  }
}
