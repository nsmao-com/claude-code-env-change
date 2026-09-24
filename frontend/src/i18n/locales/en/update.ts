import type { DeepString } from '../../types'
import type { zhUpdate } from '../zh/update'

export const enUpdate: DeepString<typeof zhUpdate> = {
  title: "Software update",
  desc: "Checks GitHub Releases for new versions; you can download and install in the app.",
  checkFailed: "Update check failed",
  current: "Current version",
  latest: "Latest on GitHub",
  available: "Update available",
  upToDate: "Up to date",
  notes: "Release notes",
  later: "Later",
  openRelease: "Open release page",
  updating: "Updating",
  updateNow: "Update now",
  preparing: "Preparing download…",
  starting: "Starting download…",
  restarting: "Restarting soon…",
  failed: "Update failed",
}
