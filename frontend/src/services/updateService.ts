import type { UpdateInfo, UpdateProgress } from '@/types'
import {
  CheckForUpdate,
  CheckLastUpdateResult,
  DownloadAndApplyUpdate,
  GetAppVersion,
  OpenReleasePage,
} from '../../wailsjs/go/main/App'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { onAppEvent } from '@/services/appBridge'

function asProgress(data: unknown): UpdateProgress | null {
  const raw = Array.isArray(data) ? data[0] : data
  if (!raw || typeof raw !== 'object') return null
  return raw as UpdateProgress
}

export const updateService = {
  check(): Promise<UpdateInfo> {
    return CheckForUpdate() as Promise<UpdateInfo>
  },
  apply(): Promise<void> {
    return DownloadAndApplyUpdate()
  },
  version(): Promise<string> {
    return GetAppVersion()
  },
  lastUpdateError(): Promise<string> {
    return CheckLastUpdateResult()
  },
  openReleasePage(): Promise<void> {
    return OpenReleasePage()
  },
  openUrl(url: string) {
    BrowserOpenURL(url)
  },
  onProgress(handler: (progress: UpdateProgress) => void): () => void {
		return onAppEvent('update:progress', (data) => {
			const progress = asProgress(data)
      if (progress) handler(progress)
    })
  },
}
