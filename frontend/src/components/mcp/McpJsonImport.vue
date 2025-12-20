<template>
  <AppModal v-model="isOpen" title="导入 MCP 服务器" size="lg">
    <div class="space-y-4">
      <p class="text-sm text-muted-foreground">
        粘贴 MCP 配置 JSON，支持 Claude/Codex/Gemini 格式
      </p>

      <div class="text-xs text-muted-foreground bg-muted p-3 rounded-lg font-mono">
        <p>• mcpServers 对象: {"mcpServers": {...}}</p>
        <p>• 服务器列表对象: {"server1": {...}, "server2": {...}}</p>
        <p>• 单个服务器: {"command": "npx", "args": [...]}</p>
      </div>

      <!-- Platform Selection -->
      <div>
        <label class="block text-sm font-medium mb-2">导入到平台</label>
        <div class="flex gap-2">
          <button
            type="button"
            :class="['platform-btn', { active: selectedPlatforms.claude }]"
            @click="selectedPlatforms.claude = !selectedPlatforms.claude"
          >
            <i class="fas fa-robot"></i>
            <span>Claude</span>
            <i v-if="selectedPlatforms.claude" class="fas fa-check check-icon"></i>
          </button>
          <button
            type="button"
            :class="['platform-btn', { active: selectedPlatforms.codex }]"
            @click="selectedPlatforms.codex = !selectedPlatforms.codex"
          >
            <i class="fas fa-terminal"></i>
            <span>Codex</span>
            <i v-if="selectedPlatforms.codex" class="fas fa-check check-icon"></i>
          </button>
          <button
            type="button"
            :class="['platform-btn', { active: selectedPlatforms.gemini }]"
            @click="selectedPlatforms.gemini = !selectedPlatforms.gemini"
          >
            <i class="fas fa-gem"></i>
            <span>Gemini</span>
            <i v-if="selectedPlatforms.gemini" class="fas fa-check check-icon"></i>
          </button>
        </div>
      </div>

      <div>
        <label class="block text-sm font-medium mb-1.5">JSON 内容</label>
        <textarea
          v-model="jsonInput"
          class="input h-64 font-mono text-xs"
          placeholder='{"mcpServers": {"filesystem": {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem"]}}}'
        ></textarea>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="isOpen = false">
          取消
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="isImporting || !hasSelectedPlatform"
          @click="handleImport"
        >
          <i v-if="isImporting" class="fas fa-circle-notch fa-spin mr-2"></i>
          导入
        </button>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useMcpStore } from '@/stores/mcpStore'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'

interface Props {
  modelValue: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  imported: []
}>()

const mcpStore = useMcpStore()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const jsonInput = ref('')
const isImporting = ref(false)
const selectedPlatforms = ref({
  claude: true,
  codex: false,
  gemini: false
})

const hasSelectedPlatform = computed(() => {
  return selectedPlatforms.value.claude || selectedPlatforms.value.codex || selectedPlatforms.value.gemini
})

// Reset when modal closes
watch(isOpen, (open) => {
  if (!open) {
    jsonInput.value = ''
    selectedPlatforms.value = { claude: true, codex: false, gemini: false }
  }
})

async function handleImport() {
  if (!jsonInput.value.trim()) {
    toast.error('请输入 JSON 内容')
    return
  }

  if (!hasSelectedPlatform.value) {
    toast.error('请至少选择一个平台')
    return
  }

  isImporting.value = true
  try {
    const servers = await mcpStore.importFromJSON(jsonInput.value)
    if (!servers || servers.length === 0) {
      toast.error('没有找到有效的服务器配置')
      return
    }

    // Set enable_platform based on selection
    const platforms: string[] = []
    if (selectedPlatforms.value.claude) platforms.push('claude-code')
    if (selectedPlatforms.value.codex) platforms.push('codex')
    if (selectedPlatforms.value.gemini) platforms.push('gemini')

    // Update each server's enable_platform
    servers.forEach(server => {
      server.enable_platform = platforms
    })

    await mcpStore.addServers(servers)
    toast.success(`成功导入 ${servers.length} 个 MCP 服务器`)
    isOpen.value = false
    emit('imported')
  } catch (e: any) {
    toast.error('导入失败: ' + e.message)
  } finally {
    isImporting.value = false
  }
}
</script>

<style scoped>
textarea.input {
  resize: vertical;
}

.platform-btn {
  @apply flex-1 flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg border-2 border-border bg-background text-muted-foreground text-sm font-medium transition-all duration-200 cursor-pointer relative;
}

.platform-btn:hover {
  @apply border-foreground/30 text-foreground;
}

.platform-btn.active {
  @apply border-primary bg-primary/10 text-primary;
}

.platform-btn .check-icon {
  @apply absolute -top-1.5 -right-1.5 w-4 h-4 bg-primary text-primary-foreground rounded-full text-[10px] flex items-center justify-center;
}
</style>
