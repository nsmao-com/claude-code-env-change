<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="visible" class="fixed inset-0 z-[9999] flex items-center justify-center p-4">
        <!-- Overlay -->
        <div class="absolute inset-0 bg-black/50 backdrop-blur-sm" @click="close"></div>

        <!-- Modal -->
        <div class="relative w-full max-w-4xl max-h-[90vh] bg-card rounded-2xl shadow-2xl border border-border flex flex-col overflow-hidden">
          <!-- Header -->
          <div class="flex items-center justify-between px-6 py-4 border-b border-border flex-none">
            <div>
              <h2 class="text-lg font-bold text-foreground uppercase tracking-tight">提示词规则</h2>
              <p class="text-xs text-muted-foreground mt-0.5">编辑 Claude/Codex/Gemini 的自定义提示词</p>
            </div>
            <button
              class="w-8 h-8 rounded-full hover:bg-muted flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors"
              @click="close"
            >
              <i class="fas fa-times"></i>
            </button>
          </div>

          <!-- Tab Bar -->
          <div class="px-6 py-3 border-b border-border flex-none">
            <div class="relative p-1 bg-secondary/50 border border-border/50 rounded-full inline-flex backdrop-blur-sm">
              <!-- Glider -->
              <div
                class="absolute top-1 bottom-1 bg-white rounded-full transition-all duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] shadow-[0_2px_8px_rgba(0,0,0,0.08)] border border-black/5 dark:border-white/10"
                :style="gliderStyle"
              ></div>

              <button
                v-for="tab in tabs"
                :key="tab.value"
                ref="tabRefs"
                :class="['relative z-10 px-5 py-1.5 rounded-full text-xs font-bold uppercase tracking-wide transition-colors duration-200', { 'text-foreground': activeTab === tab.value, 'text-muted-foreground hover:text-foreground/80': activeTab !== tab.value }]"
                @click="switchTab(tab.value)"
              >
                {{ tab.label }}
              </button>
            </div>
          </div>

          <!-- Content -->
          <div class="flex-1 overflow-hidden flex flex-col min-h-0">
            <div v-if="isLoading" class="flex-1 flex items-center justify-center">
              <i class="fas fa-circle-notch fa-spin text-2xl text-muted-foreground"></i>
            </div>

            <template v-else>
              <!-- File Info -->
              <div class="px-6 py-3 flex items-center justify-between border-b border-border/50 flex-none">
                <div class="flex items-center gap-3">
                  <span class="text-xs text-muted-foreground font-mono truncate max-w-md" :title="currentFile?.path">
                    {{ currentFile?.path || '-' }}
                  </span>
                  <span v-if="currentFile?.exists" class="text-[10px] px-2 py-0.5 rounded-full bg-green-500/10 text-green-600 font-bold uppercase">
                    已存在
                  </span>
                  <span v-else class="text-[10px] px-2 py-0.5 rounded-full bg-orange-500/10 text-orange-600 font-bold uppercase">
                    未创建
                  </span>
                </div>
                <div class="flex items-center gap-2">
                  <button
                    v-if="currentFile?.exists"
                    class="text-xs px-3 py-1.5 rounded-lg border border-destructive/50 text-destructive hover:bg-destructive hover:text-destructive-foreground transition-colors font-medium"
                    @click="deleteFile"
                  >
                    <i class="fas fa-trash mr-1.5"></i>删除
                  </button>
                </div>
              </div>

              <!-- Editor -->
              <div class="flex-1 p-4 overflow-hidden">
                <textarea
                  v-model="content"
                  class="w-full h-full resize-y min-h-[200px] bg-background border border-border rounded-xl p-4 text-sm font-mono text-foreground placeholder:text-muted-foreground/50 focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary/50 transition-all"
                  :placeholder="getPlaceholder()"
                  spellcheck="false"
                ></textarea>
              </div>
            </template>
          </div>

          <!-- Footer -->
          <div class="px-6 py-4 border-t border-border flex items-center justify-between flex-none">
            <p class="text-xs text-muted-foreground">
              <i class="fas fa-info-circle mr-1.5"></i>
              修改后需要重启 CLI 工具生效
            </p>
            <div class="flex items-center gap-3">
              <button
                class="btn btn-secondary h-9 px-5 text-xs font-bold"
                @click="close"
              >
                取消
              </button>
              <button
                class="btn btn-primary h-9 px-5 text-xs font-bold"
                :disabled="isSaving"
                @click="save"
              >
                <i v-if="isSaving" class="fas fa-circle-notch fa-spin mr-2"></i>
                <i v-else class="fas fa-save mr-2"></i>
                保存
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { GetPromptFiles, SavePromptFile, DeletePromptFile } from '../../../wailsjs/go/main/App'

interface PromptFile {
  provider: string
  path: string
  content: string
  exists: boolean
}

interface Props {
  visible: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  saved: []
}>()

const tabs = [
  { value: 'claude', label: 'CLAUDE' },
  { value: 'codex', label: 'CODEX' },
  { value: 'gemini', label: 'GEMINI' }
]

const activeTab = ref('claude')
const isLoading = ref(false)
const isSaving = ref(false)
const files = ref<PromptFile[]>([])
const content = ref('')

// Tab glider
const tabRefs = ref<HTMLElement[]>([])
const gliderStyle = ref({ left: '4px', width: '0px' })

const currentFile = computed(() => {
  return files.value.find(f => f.provider === activeTab.value)
})

function updateGlider() {
  nextTick(() => {
    const activeIndex = tabs.findIndex(t => t.value === activeTab.value)
    if (activeIndex !== -1 && tabRefs.value[activeIndex]) {
      const el = tabRefs.value[activeIndex]
      gliderStyle.value = {
        left: `${el.offsetLeft}px`,
        width: `${el.offsetWidth}px`
      }
    }
  })
}

function getPlaceholder(): string {
  const placeholders: Record<string, string> = {
    claude: `# CLAUDE.md 示例

## 项目规则
- 使用 TypeScript 编写代码
- 遵循 ESLint 规则
- 不要创建测试文件

## 代码风格
- 使用函数式编程风格
- 注释使用中文`,
    codex: `# AGENTS.md 示例

## Agent 指令
- 优先使用函数式编程模式
- 注释使用中文
- 代码风格遵循项目规范`,
    gemini: `# GEMINI.md 示例

## Gemini 指令
- 回复使用中文
- 代码风格遵循 Google Style Guide
- 简洁明了地回答问题`
  }
  return placeholders[activeTab.value] || ''
}

async function loadFiles() {
  isLoading.value = true
  try {
    files.value = await GetPromptFiles()
    // 设置当前 tab 的内容
    const file = files.value.find(f => f.provider === activeTab.value)
    content.value = file?.content || ''
  } catch (e) {
    console.error('Failed to load prompt files:', e)
  } finally {
    isLoading.value = false
  }
}

function switchTab(tab: string) {
  // 保存当前内容到对应文件对象
  const currentFileObj = files.value.find(f => f.provider === activeTab.value)
  if (currentFileObj) {
    currentFileObj.content = content.value
  }

  activeTab.value = tab

  // 加载新 tab 的内容
  const newFile = files.value.find(f => f.provider === tab)
  content.value = newFile?.content || ''

  updateGlider()
}

async function save() {
  isSaving.value = true
  try {
    await SavePromptFile(activeTab.value, content.value)

    // 更新本地状态
    const file = files.value.find(f => f.provider === activeTab.value)
    if (file) {
      file.content = content.value
      file.exists = true
    }

    emit('saved')
  } catch (e) {
    console.error('Failed to save prompt file:', e)
  } finally {
    isSaving.value = false
  }
}

async function deleteFile() {
  if (!confirm(`确定要删除 ${activeTab.value.toUpperCase()} 的提示词文件吗？`)) {
    return
  }

  try {
    await DeletePromptFile(activeTab.value)

    // 更新本地状态
    const file = files.value.find(f => f.provider === activeTab.value)
    if (file) {
      file.content = ''
      file.exists = false
    }
    content.value = ''

    emit('saved')
  } catch (e) {
    console.error('Failed to delete prompt file:', e)
  }
}

function close() {
  emit('close')
}

// 监听 visible 变化，打开时加载数据
watch(() => props.visible, (newVal) => {
  if (newVal) {
    loadFiles()
    setTimeout(updateGlider, 100)
  }
})
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-active .relative,
.modal-leave-active .relative {
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-from .relative,
.modal-leave-to .relative {
  transform: scale(0.95);
  opacity: 0;
}

textarea {
  scrollbar-width: thin;
  scrollbar-color: hsl(var(--muted)) transparent;
}

textarea::-webkit-scrollbar {
  width: 6px;
}

textarea::-webkit-scrollbar-track {
  background: transparent;
}

textarea::-webkit-scrollbar-thumb {
  background: hsl(var(--muted));
  border-radius: 3px;
}
</style>
