<template>
  <header
    class="grid h-16 shrink-0 grid-cols-[1fr_auto_1fr] items-center px-4"
    style="--wails-draggable: drag"
  >
    <div class="flex items-center gap-2.5 pl-1" style="--wails-draggable: nodrag">
      <div class="flex size-7 items-center justify-center rounded-lg bg-[#F5A524] text-[13px] font-bold text-white">
        C
      </div>
      <span class="text-[15px] font-medium tracking-tight">claude</span>
    </div>

    <nav class="flex max-w-full items-center overflow-x-auto" style="--wails-draggable: nodrag">
      <button
        v-for="item in APP_PAGES"
        :key="item.id"
        type="button"
        class="relative h-9 shrink-0 whitespace-nowrap px-4 text-[13px] font-medium transition-colors"
        :class="page === item.id ? 'text-background' : 'text-muted-foreground hover:text-foreground'"
        @click="$emit('navigate', item.id)"
      >
        <motion.span
          v-if="page === item.id"
          layout-id="top-nav-pill"
          class="absolute inset-0 rounded-full bg-foreground"
          :transition="{ type: 'spring', stiffness: 480, damping: 40 }"
        />
        <span class="relative z-10">{{ item.label }}</span>
      </button>
    </nav>

    <div class="flex items-center justify-end gap-0.5" style="--wails-draggable: nodrag">
      <Button variant="ghost" size="icon-sm" title="搜索配置" @click="focusSearch">
        <Search class="size-4" />
      </Button>
      <Button variant="ghost" size="icon-sm" :title="updateAvailable ? '有新版本' : '检查更新'" @click="$emit('checkUpdate')">
        <span class="relative">
          <Bell class="size-4" />
          <span v-if="updateAvailable" class="absolute -top-0.5 -right-0.5 size-1.5 rounded-full bg-destructive" />
        </span>
      </Button>
      <Button variant="ghost" size="icon-sm" :title="isDark ? '浅色模式' : '深色模式'" @click="setDark(!isDark)">
        <Sun v-if="isDark" class="size-4" />
        <Moon v-else class="size-4" />
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button variant="ghost" class="size-8 rounded-full p-0">
            <span class="flex size-7 items-center justify-center rounded-full bg-muted text-[11px] font-medium">
              {{ avatarLetter }}
            </span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-52">
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
          <DropdownMenuItem @click="$emit('clearClaude')">清除 Claude</DropdownMenuItem>
          <DropdownMenuItem @click="$emit('clearCodex')">清除 Codex</DropdownMenuItem>
          <DropdownMenuItem @click="$emit('clearGemini')">清除 Gemini</DropdownMenuItem>
          <DropdownMenuItem @click="$emit('clearOpenclaw')">清除 OpenClaw</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" @click="$emit('clearAll')">清除全部</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem disabled>v{{ appVersion }}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <span class="mx-1 h-4 w-px bg-border" />
      <Button variant="ghost" size="icon-sm" title="最小化" @click="minimizeWindow">
        <Minus class="size-3.5" />
      </Button>
      <Button variant="ghost" size="icon-sm" title="最大化" @click="toggleMaximize">
        <Square class="size-3" />
      </Button>
      <Button variant="ghost" size="icon-sm" class="hover:bg-destructive hover:text-white" title="关闭" @click="closeWindow">
        <X class="size-3.5" />
      </Button>
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
  Minus,
  Moon,
  Search,
  Square,
  Sun,
  Upload,
  X,
} from '@lucide/vue'
import type { AppPage } from '@/types'
import { APP_PAGES } from '@/lib/nav'
import { useTheme } from '@/composables/useTheme'
import { updateService } from '@/services/updateService'
import { Button } from '@/components/ui/button'
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
  clearOpenclaw: []
  clearAll: []
}>()

const { isDark, setDark } = useTheme()
const appVersion = ref('2.0.0')
const avatarLetter = computed(() => 'C')

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
