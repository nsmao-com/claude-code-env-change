<template>
  <header
    class="flex h-16 shrink-0 items-center gap-3 px-6"
    style="--wails-draggable: drag"
  >
    <div class="flex items-center gap-2" style="--wails-draggable: nodrag">
      <AppTooltip :content="t('nav.home')">
        <button
          type="button"
          class="flex items-center gap-2"
          @click="$emit('navigate', 'home')"
        >
          <AppLogo class="size-9" />
          <span class="text-[15px] font-semibold tracking-[0.14em] text-foreground">AI ENV</span>
        </button>
      </AppTooltip>
    </div>

    <div
      class="flex items-center gap-0.5"
      style="--wails-draggable: nodrag"
      @mouseleave="hoveredTool = null"
      @focusout="onNavFocusOut"
    >
      <button
        v-for="item in navItems"
        :key="item.id"
        type="button"
        class="relative isolate flex h-9 cursor-pointer items-center rounded-full px-2.5 outline-none"
        :class="showToolPill(item.id) ? 'text-foreground' : 'text-muted-foreground'"
        :aria-label="item.label"
        :aria-current="isToolActive(item.id) ? 'page' : undefined"
        @mouseenter="hoveredTool = item.id"
        @focus="hoveredTool = item.id"
        @click="onToolClick(item.id)"
      >
        <motion.span
          v-if="showToolPill(item.id)"
          layout-id="titlebar-tool-pill"
          class="absolute inset-0 rounded-full bg-card shadow-[0_1px_2px_rgba(16,24,40,0.06)] ring-1 ring-black/[0.08] dark:ring-white/10"
          :transition="{ type: 'spring', stiffness: 520, damping: 38 }"
        />
        <span class="relative z-10 flex items-center">
          <House v-if="item.id === 'home'" class="size-4 shrink-0" />
          <BrandIcon
            v-else
            :provider="item.id"
            class="size-4 shrink-0"
            :class="iconColor(item.id)"
          />
          <span
            class="grid min-w-0 transition-[grid-template-columns] duration-200 ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:duration-0 motion-reduce:transition-none"
            :class="hoveredTool === item.id ? 'grid-cols-[1fr]' : 'grid-cols-[0fr]'"
          >
            <span class="min-w-0 overflow-hidden">
              <span
                class="block pl-1.5 text-[12px] font-medium tracking-wide whitespace-nowrap transition-opacity duration-200 motion-reduce:transition-none"
                :class="hoveredTool === item.id ? 'opacity-100' : 'opacity-0'"
              >{{ item.label }}</span>
            </span>
          </span>
        </span>
      </button>
    </div>

    <div class="flex-1" />

    <button
      type="button"
      class="hidden h-10 w-[168px] items-center gap-2 rounded-full bg-card px-4 text-[13px] text-muted-foreground shadow-[0_1px_2px_rgba(16,24,40,0.04)] transition-colors hover:bg-muted/60 md:flex"
      style="--wails-draggable: nodrag"
      @click="$emit('search')"
    >
      <Search class="size-4" />
      {{ t('titlebar.search') }}
      <span class="ml-auto rounded-md bg-muted px-1.5 py-0.5 text-[10px] font-medium">Ctrl K</span>
    </button>

    <div class="flex items-center gap-2" style="--wails-draggable: nodrag">
      <AppTooltip :content="updateAvailable ? t('titlebar.updateAvailable') : t('titlebar.checkUpdate')">
        <button
          type="button"
          class="relative flex size-10 items-center justify-center rounded-full bg-card text-foreground shadow-[0_1px_2px_rgba(16,24,40,0.04)] transition-colors hover:bg-muted"
          @click="$emit('checkUpdate')"
        >
          <Bell class="size-4.5" />
          <span v-if="updateAvailable" class="absolute top-2 right-2 size-2 rounded-full bg-brand ring-2 ring-card" />
        </button>
      </AppTooltip>

      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <button
            type="button"
            class="flex size-10 items-center justify-center rounded-full bg-card text-foreground shadow-[0_1px_2px_rgba(16,24,40,0.04)] transition-colors hover:bg-muted"
          >
            <Menu class="size-4.5" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-48">
          <DropdownMenuItem
            v-for="item in menuPages"
            :key="item.id"
            :disabled="page === item.id"
            @click="$emit('navigate', item.id)"
          >
            <component :is="item.icon" />
            {{ item.label }}
            <DropdownMenuShortcut v-if="page === item.id">{{ t('nav.current') }}</DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem @click="$emit('checkUpdate')">
            <ArrowUpCircle />
            {{ t('titlebar.checkUpdate') }}
            <DropdownMenuShortcut v-if="updateAvailable">{{ t('titlebar.updateAvailable') }}</DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('export')">
            <Download />
            {{ t('titlebar.export') }}
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('import')">
            <Upload />
            {{ t('titlebar.import') }}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem @click="$emit('clearClaude')">
            <BrandIcon provider="claude" class="size-3.5" />
            {{ t('titlebar.clearClaude') }}
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('clearCodex')">
            <BrandIcon provider="codex" class="size-3.5" />
            {{ t('titlebar.clearCodex') }}
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('clearGemini')">
            <BrandIcon provider="gemini" class="size-3.5" />
            {{ t('titlebar.clearGemini') }}
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('clearOpencode')">
            <BrandIcon provider="opencode" class="size-3.5" />
            {{ t('titlebar.clearOpencode') }}
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('clearGrok')">
            <BrandIcon provider="grok" class="size-3.5" />
            {{ t('titlebar.clearGrok') }}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" @click="$emit('clearAll')">{{ t('titlebar.clearAll') }}</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem disabled>v{{ appVersion }}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <AppTooltip :content="t('nav.settings')">
        <button
          type="button"
          class="flex size-10 items-center justify-center rounded-full bg-card text-foreground shadow-[0_1px_2px_rgba(16,24,40,0.04)] transition-colors hover:bg-muted"
          :class="page === 'settings' ? 'ring-1 ring-black/[0.08]' : ''"
          @click="$emit('navigate', 'settings')"
        >
          <Settings class="size-4.5" />
        </button>
      </AppTooltip>

      <AppTooltip :content="isDark ? t('titlebar.lightMode') : t('titlebar.darkMode')">
        <button
          type="button"
          class="flex size-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          @click="setDark(!isDark)"
        >
          <Sun v-if="isDark" class="size-4" />
          <Moon v-else class="size-4" />
        </button>
      </AppTooltip>
      <span class="mx-0.5 h-5 w-px bg-border" />
      <div class="flex items-center gap-0.5">
        <AppTooltip :content="t('titlebar.minimize')">
          <button type="button" class="flex size-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground" @click="minimizeWindow">
            <Minus class="size-4" />
          </button>
        </AppTooltip>
        <AppTooltip :content="t('titlebar.maximize')">
          <button type="button" class="flex size-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground" @click="toggleMaximize">
            <Square class="size-3" />
          </button>
        </AppTooltip>
        <AppTooltip :content="t('titlebar.close')">
          <button type="button" class="flex size-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-destructive hover:text-white" @click="closeWindow">
            <X class="size-4" />
          </button>
        </AppTooltip>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { motion } from 'motion-v'
import {
  ArrowUpCircle,
  Bell,
  Download,
  House,
  Menu,
  Minus,
  Moon,
  Square,
  Search,
  Settings,
  Sun,
  Upload,
  X,
} from '@lucide/vue'
import type { AppPage, Provider } from '@/types'
import { APP_PAGES } from '@/lib/nav'
import { WORKSPACE_TOOLS } from '@/lib/workspace'
import { useConfigStore } from '@/stores/configStore'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/composables/useI18n'
import { updateService } from '@/services/updateService'
import AppLogo from '@/components/common/AppLogo.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import BrandIcon from '@/components/common/BrandIcon.vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

const props = withDefaults(defineProps<{
  page?: AppPage
  updateAvailable?: boolean
}>(), {
  page: 'home',
  updateAvailable: false,
})

const emit = defineEmits<{
  navigate: [page: AppPage]
  checkUpdate: []
  export: []
  import: []
  clearClaude: []
  clearCodex: []
  clearGemini: []
  clearOpencode: []
  clearGrok: []
  clearAll: []
  search: []
}>()

const configStore = useConfigStore()
const { isDark, setDark } = useTheme()
const { t } = useI18n()
const appVersion = ref('2.2.0')

const providerIcons = WORKSPACE_TOOLS.filter(item => item.id !== 'all') as { id: Provider; label: string }[]
const navItems = computed(() => [
  { id: 'home', label: t('nav.home') },
  ...providerIcons,
])
const menuPages = computed(() => APP_PAGES.map(item => ({
  ...item,
  label: t(`nav.${item.id}`),
})))
const hoveredTool = ref<string | null>(null)

const ICON_COLORS: Record<string, string> = {
  claude: 'text-[#D97757]',
  codex: 'text-[#1A1D21] dark:text-white/80',
  gemini: 'text-[#4F6BED]',
  opencode: 'text-[#131010] dark:text-white/80',
  grok: 'text-[#6B7280]',
}

function iconColor(id: string) {
  return ICON_COLORS[id] || 'text-muted-foreground'
}

function isToolActive(id: string) {
  if (id === 'home') return props.page === 'home'
  return props.page === 'env' && configStore.currentFilter === id
}

function showToolPill(id: string) {
  if (hoveredTool.value) return hoveredTool.value === id
  return isToolActive(id)
}

function goHome() {
  configStore.setFilter('all')
  emit('navigate', 'home')
}

function onProvider(id: Provider) {
  configStore.setFilter(id)
  emit('navigate', 'env')
}

function onToolClick(id: string) {
  if (id === 'home') goHome()
  else onProvider(id as Provider)
}

function onNavFocusOut(event: FocusEvent) {
  const root = event.currentTarget as HTMLElement
  const next = event.relatedTarget as Node | null
  if (!next || !root.contains(next)) hoveredTool.value = null
}

onMounted(async () => {
  try {
    appVersion.value = await updateService.version()
  } catch {
    /* ignore */
  }
})

function closeWindow() {
  window.runtime?.Quit()
}

function minimizeWindow() {
  window.runtime?.WindowMinimise()
}

function toggleMaximize() {
  window.runtime?.WindowToggleMaximise()
}
</script>
