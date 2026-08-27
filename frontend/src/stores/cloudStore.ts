import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { CloudConfig, CloudSyncStatus } from '@/types'
import { cloudService } from '@/services/cloudService'

const emptyConfig = (): CloudConfig => ({
  enabled: false,
  provider: 'aliyun',
  endpoint: 'oss-cn-hangzhou.aliyuncs.com',
  region: 'oss-cn-hangzhou',
  bucket: '',
  object_key: 'claude-env-switcher/backup.bin',
  access_key: '',
  secret_key: '',
  path_style: false,
  passphrase: '',
  auto_push: true,
  auto_pull_on_start: false
})

export const useCloudStore = defineStore('cloud', () => {
  const config = ref<CloudConfig>(emptyConfig())
  const status = ref<CloudSyncStatus | null>(null)
  const isLoading = ref(false)

  async function load() {
    isLoading.value = true
    try {
      config.value = { ...emptyConfig(), ...(await cloudService.getConfig()) }
      status.value = await cloudService.getStatus()
    } finally {
      isLoading.value = false
    }
  }

  async function save(cfg: CloudConfig) {
    await cloudService.saveConfig(cfg)
    config.value = cfg
    status.value = await cloudService.getStatus()
  }

  async function refreshStatus() {
    status.value = await cloudService.getStatus()
  }

  return { config, status, isLoading, load, save, refreshStatus }
})
