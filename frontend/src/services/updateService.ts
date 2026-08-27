import type { UpdateInfo, UpdateProgress } from '@/types'
import {
  CheckForUpdate,
  DownloadAndApplyUpdate,
  GetAppVersion,
  OpenReleasePage,
} from '../../wailsjs/go/main/App'
import { BrowserOpenURL, EventsOn } from '../../wailsjs/runtime/runtime'

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
  openReleasePage(): Promise<void> {
    return OpenReleasePage()
  },
  openUrl(url: string) {
    BrowserOpenURL(url)
  },
  onProgress(handler: (progress: UpdateProgress) => void): () => void {
    return EventsOn('update:progress', (...args: any[]) => {
      const progress = asProgress(args.length === 1 ? args[0] : args)
      if (progress) handler(progress)
    })
  },
}
