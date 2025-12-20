<template>
  <AppModal v-model="isOpen" :title="isEditing ? '编辑 MCP 服务器' : '添加 MCP 服务器'" size="md">
    <form @submit.prevent="handleSubmit">
      <!-- Name -->
      <div class="mb-4">
        <AppInput
          v-model="form.name"
          label="服务器名称"
          placeholder="输入服务器名称"
        />
      </div>

      <!-- Type Toggle -->
      <div class="mb-4">
        <label class="block text-sm font-medium mb-1.5">类型</label>
        <div class="flex gap-2">
          <button
            type="button"
            :class="['btn flex-1', form.type === 'stdio' ? 'btn-primary' : 'btn-outline']"
            @click="form.type = 'stdio'"
          >
            <i class="fas fa-terminal mr-2"></i>
            Stdio
          </button>
          <button
            type="button"
            :class="['btn flex-1', form.type === 'http' ? 'btn-primary' : 'btn-outline']"
            @click="form.type = 'http'"
          >
            <i class="fas fa-globe mr-2"></i>
            HTTP
          </button>
        </div>
      </div>

      <!-- Stdio Fields -->
      <div v-if="form.type === 'stdio'" class="space-y-4">
        <AppInput
          v-model="form.command"
          label="Command"
          placeholder="npx"
        />
        <div>
          <label class="block text-sm font-medium mb-1.5">Args (每行一个)</label>
          <textarea
            v-model="form.args"
            class="input h-24 font-mono text-xs"
            placeholder="-y&#10;@modelcontextprotocol/server-filesystem"
          ></textarea>
        </div>
        <div>
          <label class="block text-sm font-medium mb-1.5">环境变量 (KEY=VALUE)</label>
          <textarea
            v-model="form.env"
            class="input h-20 font-mono text-xs"
            placeholder="API_KEY=xxx&#10;DEBUG=true"
          ></textarea>
        </div>
      </div>

      <!-- HTTP Fields -->
      <div v-if="form.type === 'http'" class="space-y-4">
        <AppInput
          v-model="form.url"
          label="URL"
          placeholder="http://localhost:3000"
        />
      </div>

      <!-- Optional Fields -->
      <div class="space-y-4 mt-4 pt-4 border-t border-border">
        <AppInput
          v-model="form.website"
          label="官网 (可选)"
          placeholder="https://..."
        />
        <AppInput
          v-model="form.tips"
          label="备注 (可选)"
          placeholder="服务器说明..."
        />

        <!-- Platform Selection -->
        <div>
          <label class="block text-sm font-medium mb-2">启用平台</label>
          <div class="flex gap-2">
            <button
              type="button"
              :class="['platform-btn', { active: form.platforms.claude }]"
              @click="form.platforms.claude = !form.platforms.claude"
            >
              <i class="fas fa-robot"></i>
              <span>Claude</span>
              <i v-if="form.platforms.claude" class="fas fa-check check-icon"></i>
            </button>
            <button
              type="button"
              :class="['platform-btn', { active: form.platforms.codex }]"
              @click="form.platforms.codex = !form.platforms.codex"
            >
              <i class="fas fa-terminal"></i>
              <span>Codex</span>
              <i v-if="form.platforms.codex" class="fas fa-check check-icon"></i>
            </button>
            <button
              type="button"
              :class="['platform-btn', { active: form.platforms.gemini }]"
              @click="form.platforms.gemini = !form.platforms.gemini"
            >
              <i class="fas fa-gem"></i>
              <span>Gemini</span>
              <i v-if="form.platforms.gemini" class="fas fa-check check-icon"></i>
            </button>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="isOpen = false">
          取消
        </button>
        <button type="button" class="btn btn-primary" @click="handleSubmit">
          {{ isEditing ? '保存' : '添加' }}
        </button>
      </div>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { MCPServer } from '@/types'
import { useMcpStore } from '@/stores/mcpStore'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppInput from '@/components/common/AppInput.vue'

interface Props {
  modelValue: boolean
  editServer?: MCPServer | null
  editIndex?: number
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const mcpStore = useMcpStore()
const toast = useToast()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isEditing = computed(() => !!props.editServer)

const defaultForm = () => ({
  name: '',
  type: 'stdio' as 'stdio' | 'http',
  command: '',
  args: '',
  env: '',
  url: '',
  website: '',
  tips: '',
  platforms: {
    claude: true,
    codex: false,
    gemini: false
  }
})

const form = ref(defaultForm())

// Watch for edit server changes
watch(() => props.editServer, (server) => {
  if (server) {
    form.value.name = server.name || ''
    form.value.type = (server.type || 'stdio') as 'stdio' | 'http'
    form.value.command = server.command || ''
    form.value.args = (server.args || []).join('\n')
    form.value.env = Object.entries(server.env || {})
      .map(([k, v]) => `${k}=${v}`)
      .join('\n')
    form.value.url = server.url || ''
    form.value.website = server.website || ''
    form.value.tips = server.tips || ''
    const platforms = server.enable_platform || []
    form.value.platforms.claude = platforms.includes('claude-code')
    form.value.platforms.codex = platforms.includes('codex')
    form.value.platforms.gemini = platforms.includes('gemini')
  } else {
    form.value = defaultForm()
  }
}, { immediate: true })

// Reset form when modal closes
watch(isOpen, (open) => {
  if (!open) {
    form.value = defaultForm()
  }
})

async function handleSubmit() {
  const name = form.value.name.trim()
  if (!name) {
    toast.error('请输入服务器名称')
    return
  }

  // Check duplicate
  const exists = mcpStore.servers.some(
    (s, i) => s.name === name && i !== props.editIndex
  )
  if (exists) {
    toast.error('服务器名称已存在')
    return
  }

  // Validate type-specific fields
  if (form.value.type === 'http' && !form.value.url.trim()) {
    toast.error('请输入 URL')
    return
  }
  if (form.value.type === 'stdio' && !form.value.command.trim()) {
    toast.error('请输入 Command')
    return
  }

  // Build enable_platform
  const enablePlatform: string[] = []
  if (form.value.platforms.claude) enablePlatform.push('claude-code')
  if (form.value.platforms.codex) enablePlatform.push('codex')
  if (form.value.platforms.gemini) enablePlatform.push('gemini')

  // Parse args
  const args = form.value.args.trim()
    ? form.value.args.split('\n').map(s => s.trim()).filter(s => s)
    : []

  // Parse env
  const env: Record<string, string> = {}
  if (form.value.env.trim()) {
    form.value.env.split('\n').forEach(line => {
      const idx = line.indexOf('=')
      if (idx > 0) {
        const key = line.substring(0, idx).trim()
        const value = line.substring(idx + 1).trim()
        if (key) env[key] = value
      }
    })
  }

  const serverData: MCPServer = {
    name,
    type: form.value.type,
    command: form.value.type === 'stdio' ? form.value.command.trim() : undefined,
    args: form.value.type === 'stdio' ? args : undefined,
    env: form.value.type === 'stdio' ? env : undefined,
    url: form.value.type === 'http' ? form.value.url.trim() : undefined,
    website: form.value.website.trim() || undefined,
    tips: form.value.tips.trim() || undefined,
    enable_platform: enablePlatform,
    enabled_in_claude: false,
    enabled_in_codex: false,
    enabled_in_gemini: false,
    missing_placeholders: []
  }

  try {
    if (isEditing.value && props.editIndex !== undefined) {
      await mcpStore.updateServer(props.editIndex, serverData)
    } else {
      await mcpStore.addServer(serverData)
    }
    toast.success('MCP 服务器已保存')
    isOpen.value = false
    emit('saved')
  } catch (e: any) {
    toast.error('保存失败: ' + e.message)
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
