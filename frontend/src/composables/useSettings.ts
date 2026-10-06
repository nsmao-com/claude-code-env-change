import { computed, reactive, watch } from 'vue'
import { isLocale, type Locale } from '@/i18n'

export type ThemeMode = 'system' | 'light' | 'dark'
export type AccentId = 'orange' | 'blue' | 'emerald' | 'violet' | 'rose' | 'zinc'
export type TextSize = 'small' | 'default' | 'large' | 'xlarge'

// 文字大小：调整根字号，界面中以 rem 计的文字与间距等比缩放
export const TEXT_SIZES: Record<TextSize, string> = { small: '15px', default: '16px', large: '17px', xlarge: '18px' }

export interface AppSettings {
  language: Locale
  theme: ThemeMode
  accent: AccentId
  reducedMotion: boolean
  textSize: TextSize
  checkUpdateOnLaunch: boolean
  restoreLastPage: boolean
}

const STORAGE_KEY = 'ai-env-settings'
const LAST_PAGE_KEY = 'ai-env-last-page'
const LEGACY_THEME_KEY = 'theme'
const SETTINGS_VERSION_KEY = 'ai-env-settings-version'
// 2：启动统一回首页，“记住上次打开的页面”改为默认关闭
const SETTINGS_VERSION = 2

export const ACCENTS: { id: AccentId; swatch: string }[] = [
  { id: 'orange', swatch: 'oklch(0.666 0.178 45)' },
  { id: 'blue', swatch: 'oklch(0.58 0.16 250)' },
  { id: 'emerald', swatch: 'oklch(0.62 0.15 155)' },
  { id: 'violet', swatch: 'oklch(0.58 0.18 300)' },
  { id: 'rose', swatch: 'oklch(0.62 0.19 15)' },
  { id: 'zinc', swatch: 'oklch(0.45 0.02 270)' },
]

const defaults: AppSettings = {
  language: 'zh',
  theme: 'light',
  accent: 'orange',
  reducedMotion: false,
  textSize: 'default',
  checkUpdateOnLaunch: true,
  restoreLastPage: false,
}

const state = reactive<AppSettings>({ ...defaults })
let initialized = false
let media: MediaQueryList | null = null

function readStored(): Partial<AppSettings> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return JSON.parse(raw) as Partial<AppSettings>
  } catch { /* ignore */ }
  return {}
}

// 旧版本默认开启“记住上次打开的页面”，并把这个默认值写进了 localStorage，
// 只改默认值对老用户不生效。这里按版本号一次性清掉旧的默认值和残留页面记录，
// 之后用户自己在设置里打开的选择照常保留。
function migrateStored(stored: Partial<AppSettings>): boolean {
  let version = 0
  try {
    version = Number(localStorage.getItem(SETTINGS_VERSION_KEY)) || 0
  } catch { /* ignore */ }
  if (version >= SETTINGS_VERSION) return false

  delete stored.restoreLastPage
  try {
    localStorage.removeItem(LAST_PAGE_KEY)
    localStorage.setItem(SETTINGS_VERSION_KEY, String(SETTINGS_VERSION))
  } catch { /* ignore */ }
  return true
}

function persist() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...state }))
  } catch { /* ignore */ }
}

function systemPrefersDark() {
  if (typeof window === 'undefined' || !window.matchMedia) return false
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

export function resolvedDark(theme: ThemeMode) {
  if (theme === 'system') return systemPrefersDark()
  return theme === 'dark'
}

function applyDocument() {
  const dark = resolvedDark(state.theme)
  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.dataset.accent = state.accent
  document.documentElement.dataset.reduceMotion = state.reducedMotion ? 'true' : 'false'
  document.documentElement.style.fontSize = TEXT_SIZES[state.textSize] || TEXT_SIZES.default
}

function onSystemTheme() {
  if (state.theme === 'system') applyDocument()
}

export function initSettings() {
  if (initialized) return
  initialized = true

  const stored = readStored()
  const migrated = migrateStored(stored)
  if (isLocale(stored.language)) state.language = stored.language
  if (stored.theme === 'system' || stored.theme === 'light' || stored.theme === 'dark') {
    state.theme = stored.theme
  } else {
    const legacy = localStorage.getItem(LEGACY_THEME_KEY)
    if (legacy === 'dark' || legacy === 'light') state.theme = legacy
  }
  if (ACCENTS.some(item => item.id === stored.accent)) state.accent = stored.accent as AccentId
  if (typeof stored.reducedMotion === 'boolean') state.reducedMotion = stored.reducedMotion
  if (stored.textSize && stored.textSize in TEXT_SIZES) state.textSize = stored.textSize
  if (typeof stored.checkUpdateOnLaunch === 'boolean') state.checkUpdateOnLaunch = stored.checkUpdateOnLaunch
  if (typeof stored.restoreLastPage === 'boolean') state.restoreLastPage = stored.restoreLastPage

  applyDocument()
  if (migrated) persist()

  if (typeof window !== 'undefined' && window.matchMedia) {
    media = window.matchMedia('(prefers-color-scheme: dark)')
    media.addEventListener('change', onSystemTheme)
  }
}

initSettings()

watch(state, () => {
  persist()
  applyDocument()
  try {
    localStorage.setItem(LEGACY_THEME_KEY, resolvedDark(state.theme) ? 'dark' : 'light')
  } catch { /* ignore */ }
}, { deep: true })

export function useSettings() {
  const isDark = computed(() => resolvedDark(state.theme))

  function patch(partial: Partial<AppSettings>) {
    Object.assign(state, partial)
  }

  function setTheme(theme: ThemeMode) {
    state.theme = theme
  }

  function setDark(dark: boolean) {
    state.theme = dark ? 'dark' : 'light'
  }

  function toggleDark() {
    setDark(!isDark.value)
  }

  // 始终记录当前页，只在读取时判断开关；
  // 否则用户刚打开开关时没有历史记录，下次启动仍然回首页，像是开关没生效。
  function saveLastPage(page: string) {
    try {
      localStorage.setItem(LAST_PAGE_KEY, page)
    } catch { /* ignore */ }
  }

  function readLastPage() {
    if (!state.restoreLastPage) return null
    try {
      return localStorage.getItem(LAST_PAGE_KEY)
    } catch {
      return null
    }
  }

  return {
    settings: state,
    isDark,
    patch,
    setTheme,
    setDark,
    toggleDark,
    saveLastPage,
    readLastPage,
  }
}
