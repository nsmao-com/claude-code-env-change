import type { DeepString } from '../../types'
import type { zhApp } from '../zh/app'

export const enApp: DeepString<typeof zhApp> = {
  refreshed: "Refreshed",
  cloudPullFailed: "Cloud pull on startup failed: {error}",
  unknownError: "unknown error",
  driftTitle: "Local config differs",
  driftMsg: "The current config of {names} differs from the one active in this app. Sync now? Cancel keeps the local config.",
  listSep: ", ",
  driftSynced: "Current configs synced",
  lastUpdateFailed: "The last in-app update failed: {error}. Download the installer from GitHub to update manually.",
  officialAdded: "Added {count} official-login config(s)",
  officialAddFailed: "Failed to add official-login configs",
  configNotFound: "Config not found",
  copySuffix: " - Copy",
  readDropFailed: "Failed to read dropped file: {error}",
  clipboardEmpty: "The clipboard is empty. Copy some config JSON first.",
  clipboardFileName: "clipboard-config.json",
  readClipboardFailed: "Failed to read the clipboard: {error}",
  clearDesktopTitle: "Clear Claude Desktop config",
  clearDesktopMsg: "This removes the active Claude Desktop gateway config and keeps other entries and MCP settings. Continue?",
  clearedDesktop: "Claude Desktop config cleared",
  clearDesktopFailed: "Failed to clear Claude Desktop: {error}",
}
