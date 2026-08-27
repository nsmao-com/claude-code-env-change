<template>
  <header
    class="flex h-16 shrink-0 items-center gap-3 px-6"
    style="--wails-draggable: drag"
  >
    <div class="flex items-center gap-2" style="--wails-draggable: nodrag">
      <div class="relative flex size-9 items-center justify-center rounded-[12px] bg-brand text-[15px] font-bold text-white">
        C
        <span class="absolute top-1 left-1.5 size-1 rounded-full bg-white/80" />
        <span class="absolute right-1.5 bottom-1 size-1 rounded-full bg-white/60" />
      </div>
      <span class="text-lg font-bold tracking-tight text-foreground">claude</span>
    </div>

    <div class="flex items-center gap-2" style="--wails-draggable: nodrag">
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <button
            type="button"
            class="flex size-9 items-center justify-center rounded-full bg-brand text-[11px] font-semibold text-white ring-2 ring-background transition-transform hover:scale-105"
            title="用户菜单"
          >
            C
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" class="w-52">
          <DropdownMenuItem @click="$emit('checkUpdate')">
            <ArrowUpCircle />
            检查更新
            <DropdownMenuShortcut v-if="updateAvailable">新</DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('export')">
            <Download />
            导出配置
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('import')">
            <Upload />
            导入配置
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem @click="$emit('clearClaude')">
            <BrandIcon provider="claude" class="size-3.5" />
            清除 Claude
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('clearCodex')">
            <BrandIcon provider="codex" class="size-3.5" />
            清除 Codex
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('clearGemini')">
            <BrandIcon provider="gemini" class="size-3.5" />
            清除 Gemini
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('clearOpencode')">
            <BrandIcon provider="opencode" class="size-3.5" />
            清除 OpenCode
          </DropdownMenuItem>
          <DropdownMenuItem @click="$emit('clearGrok')">
            <BrandIcon provider="grok" class="size-3.5" />
            清除 Grok
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" @click="$emit('clearAll')">清除全部</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem disabled>v{{ appVersion }}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <button
        type="button"
        class="flex size-9 items-center justify-center rounded-full bg-card text-foreground ring-2 ring-background transition-colors hover:bg-muted"
        :title="isDark ? '切换浅色' : '切换深色'"
        @click="setDark(!isDark)"
      >
        <Sun v-if="isDark" class="size-4" />
        <Moon v-else class="size-4" />
      </button>

      <div class="flex size-9 flex-col items-center justify-center rounded-full bg-foreground text-[9px] leading-tight font-semibold text-background">
        <span>{{ weekdayShort }}</span>
        <span>{{ dayOfMonth }}</span>
      </div>
    </div>

    <div class="flex-1" />

    <button
      type="button"
      class="hidden h-10 w-[240px] items-center gap-2 rounded-full bg-card px-4 text-[13px] text-muted-foreground shadow-[0_1px_2px_rgba(16,24,40,0.04)] transition-colors hover:bg-muted/60 md:flex"
      style="--wails-draggable: nodrag"
      @click="focusSearch"
    >
      <Search class="size-4" />
      搜索配置、模型、地址…
      <span class="ml-auto rounded-md bg-muted px-1.5 py-0.5 text-[10px] font-medium">Ctrl K</span>
    </button>

    <div class="flex items-center gap-2" style="--wails-draggable: nodrag">
      <button
        type="button"
        class="relative flex size-10 items-center justify-center rounded-full bg-card text-foreground shadow-[0_1px_2px_rgba(16,24,40,0.04)] transition-colors hover:bg-muted"
        :title="updateAvailable ? '有新版本' : '检查更新'"
        @click="$emit('checkUpdate')"
      >
        <Bell class="size-4.5" />
        <span v-if="updateAvailable" class="absolute top-2 right-2 size-2 rounded-full bg-brand ring-2 ring-card" />
      </button>

      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <button
            type="button"
            class="flex size-10 items-center justify-center rounded-full bg-card text-foreground shadow-[0_1px_2px_rgba(16,24,40,0.04)] transition-colors hover:bg-muted"
            title="页面导航"
          >
            <Menu class="size-4.5" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-48">
          <DropdownMenuItem
            v-for="item in APP_PAGES"
            :key="item.id"
            :disabled="page === item.id"
            @click="$emit('navigate', item.id)"
          >
            <component :is="item.icon" />
            {{ item.label }}
            <DropdownMenuShortcut v-if="page === item.id">当前</DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <span class="mx-0.5 h-5 w-px bg-border" />
      <button type="button" class="flex size-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground" title="最小化" @click="minimizeWindow">
        <Minus class="size-4" />
      </button>
      <button type="button" class="flex size-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground" title="最大化" @click="toggleMaximize">
        <Square class="size-3" />
      </button>
      <button type="button" class="flex size-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-destructive hover:text-white" title="关闭" @click="closeWindow">
        <X class="size-4" />
      </button>
    </div>
  </header>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  ArrowUpCircle,
  Bell,
  Download,
  Menu,
  Minus,
  Moon,
  Square,
  Search,
  Sun,
  Upload,
  X,
} from '@lucide/vue'
import type { AppPage } from '@/types'
import { APP_PAGES } from '@/lib/nav'
import { useTheme } from '@/composables/useTheme'
import { updateService } from '@/services/updateService'
import BrandIcon from '@/components/common/BrandIcon.vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

withDefaults(defineProps<{
  page?: AppPage
  updateAvailable?: boolean
}>(), {
  page: 'env',
  updateAvailable: false,
})

defineEmits<{
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
}>()

const { isDark, setDark } = useTheme()
const appVersion = ref('2.1.0')
const now = new Date()
const weekdayShort = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'][now.getDay()].slice(1)
const dayOfMonth = String(now.getDate())

onMounted(async () => {
  try {
    appVersion.value = await updateService.version()
  } catch {
    /* ignore */
  }
})

function focusSearch() {
  const el = document.getElementById('config-search') as HTMLInputElement | null
  el?.focus()
  el?.select()
}

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
