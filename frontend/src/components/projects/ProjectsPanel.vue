<template>
  <AppModal v-model="isOpen" size="xl" :plain="embedded" width="wide" :close-on-overlay="false">
    <template #header>
      <h1 class="text-[2.5rem] leading-none font-semibold tracking-tight">{{ t('nav.projects') }}</h1>
      <p class="mt-2 text-sm text-muted-foreground">{{ t('projects.panelHint') }}</p>
    </template>
    <template #actions>
      <Button size="sm" :disabled="adding" @click="pickAndAdd">
        <Loader2 v-if="adding" class="animate-spin" />
        <FolderPlus v-else />
        {{ t('projects.add') }}
      </Button>
    </template>

    <div class="grid min-h-0 flex-1 grid-cols-1 gap-4 lg:grid-cols-[300px_minmax(0,1fr)]">
      <!-- 项目列表 -->
      <Card class="min-h-0 gap-0 overflow-hidden py-0">
        <div class="border-b p-3">
          <div class="relative">
            <Search class="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input v-model="query" class="rounded-full bg-muted/70 pl-8" :placeholder="t('projects.search')" />
          </div>
          <form class="mt-2 flex gap-1.5" @submit.prevent="addTypedPath">
            <Input v-model="typedPath" class="h-8 font-mono text-xs" :placeholder="t('projects.pathPlaceholder')" />
            <Button type="submit" size="sm" variant="outline" :disabled="!typedPath.trim() || adding">{{ t('projects.addPath') }}</Button>
          </form>
        </div>
        <div v-if="loading" class="flex justify-center py-10">
          <Loader2 class="size-5 animate-spin text-muted-foreground" />
        </div>
        <p v-else-if="filtered.length === 0" class="px-4 py-8 text-center text-xs leading-relaxed text-muted-foreground">
          {{ projects.length === 0 ? t('projects.empty') : t('projects.noMatch') }}
        </p>
        <div v-else class="max-h-[62vh] overflow-y-auto p-1.5">
          <button
            v-for="item in filtered"
            :key="item.path"
            type="button"
            :class="[
              'flex w-full flex-col items-start gap-1 rounded-xl px-3 py-2 text-left transition-colors',
              selectedPath === item.path ? 'bg-muted' : 'hover:bg-muted/50',
            ]"
            @click="select(item.path)"
          >
            <span class="flex w-full min-w-0 items-center gap-1.5">
              <FolderGit2 class="size-3.5 shrink-0 text-muted-foreground" />
              <span class="truncate text-sm font-medium">{{ item.name }}</span>
              <Pin v-if="item.pinned" class="size-3 shrink-0 text-muted-foreground" />
            </span>
            <AppTooltip :content="item.path" wrap class="block w-full min-w-0">
              <span class="block truncate font-mono text-[10.5px] text-muted-foreground">{{ item.path }}</span>
            </AppTooltip>
            <span class="flex flex-wrap gap-1">
              <Badge v-if="item.applied_env" class="h-4 px-1.5 text-[10px]">{{ item.applied_env }}</Badge>
              <Badge v-else-if="item.env_override" variant="secondary" class="h-4 px-1.5 text-[10px]">{{ t('projects.customEnv') }}</Badge>
              <Badge v-if="item.mcp_count" variant="outline" class="h-4 px-1.5 text-[10px]">MCP {{ item.mcp_count }}</Badge>
              <Badge v-if="item.has_claude_md" variant="outline" class="h-4 px-1.5 text-[10px]">CLAUDE.md</Badge>
            </span>
          </button>
        </div>
      </Card>

      <!-- 项目详情 -->
      <Card v-if="!detail" class="min-h-[320px] items-center justify-center text-sm text-muted-foreground">
        <Loader2 v-if="detailLoading" class="size-5 animate-spin" />
        <span v-else>{{ t('projects.selectHint') }}</span>
      </Card>
      <Card v-else class="min-h-0 gap-4">
        <CardHeader>
          <div class="flex min-w-0 items-start justify-between gap-3">
            <div class="min-w-0">
              <CardTitle class="truncate text-lg">{{ detail.name }}</CardTitle>
              <CardDescription class="truncate font-mono text-xs">{{ detail.path }}</CardDescription>
            </div>
            <div class="flex shrink-0 gap-1">
              <AppTooltip :content="t('projects.openFolder')">
                <Button variant="ghost" size="icon-sm" @click="reveal(detail.path)"><FolderOpen /></Button>
              </AppTooltip>
              <AppTooltip :content="t('projects.removeFromList')">
                <Button variant="ghost" size="icon-sm" @click="removeProject"><X /></Button>
              </AppTooltip>
            </div>
          </div>
          <SegmentedPills
            class="mt-3"
            :model-value="tab"
            layout-id="project-tab"
            dense
            :items="tabs"
            @update:model-value="onTab"
          />
        </CardHeader>

        <CardContent class="space-y-4">
          <!-- 接入配置 -->
          <template v-if="tab === 'env'">
            <div class="rounded-xl bg-muted/40 px-3 py-2.5 text-sm">
              <template v-if="detail.applied_env">
                <span class="text-muted-foreground">{{ t('projects.usingEnv') }}</span>
                <span class="ml-1 font-medium">{{ detail.applied_env }}</span>
              </template>
              <span v-else-if="detail.env_override">{{ t('projects.usingCustom') }}</span>
              <span v-else class="text-muted-foreground">{{ t('projects.usingGlobal') }}</span>
            </div>
            <div class="flex flex-wrap items-end gap-2">
              <div class="grid min-w-[220px] flex-1 gap-1.5">
                <Label>{{ t('projects.pickEnv') }}</Label>
                <Select v-model="envChoice">
                  <SelectTrigger class="w-full">
                    <SelectValue :placeholder="t('projects.pickEnvPlaceholder')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="env in claudeEnvs" :key="env.name" :value="env.name">{{ env.name }}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Button :disabled="!envChoice || busy" @click="applyEnv">
                <Loader2 v-if="busy" class="animate-spin" />
                {{ t('projects.applyEnv') }}
              </Button>
              <Button v-if="detail.env_override" variant="outline" :disabled="busy" @click="clearEnv">{{ t('projects.clearEnv') }}</Button>
            </div>
            <p v-if="claudeEnvs.length === 0" class="text-xs text-muted-foreground">{{ t('projects.noClaudeEnvs') }}</p>
            <div v-if="detail.env_vars.length" class="overflow-hidden rounded-xl border">
              <div
                v-for="v in detail.env_vars"
                :key="v.key"
                class="flex items-center justify-between gap-3 border-b px-3 py-1.5 font-mono text-[11px] last:border-b-0"
              >
                <span class="shrink-0 text-muted-foreground">{{ v.key }}</span>
                <span class="truncate">{{ v.value }}</span>
              </div>
            </div>
            <p class="text-[11px] leading-relaxed text-muted-foreground">
              {{ t('projects.envNote') }}
              <button type="button" class="font-mono underline-offset-2 hover:underline" @click="reveal(detail.settings_path)">.claude/settings.local.json</button>
            </p>
          </template>

          <!-- MCP -->
          <template v-else-if="tab === 'mcp'">
            <div class="flex flex-wrap items-end gap-2">
              <div class="grid min-w-[220px] flex-1 gap-1.5">
                <Label>{{ t('projects.addFromLibrary') }}</Label>
                <Select v-model="mcpChoice">
                  <SelectTrigger class="w-full">
                    <SelectValue :placeholder="t('projects.pickMcpPlaceholder')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="server in addableMcp" :key="server.name" :value="server.name">{{ server.name }}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Button :disabled="!mcpChoice || busy" @click="addMcp">
                <Plus />
                {{ t('projects.addMcp') }}
              </Button>
            </div>
            <p v-if="detail.mcp_servers.length === 0" class="py-4 text-center text-xs text-muted-foreground">{{ t('projects.noMcp') }}</p>
            <div v-else class="space-y-2">
              <div v-for="server in detail.mcp_servers" :key="server.name" class="flex items-center gap-3 rounded-xl border px-3 py-2">
                <Globe v-if="server.type === 'http' || server.type === 'sse'" class="size-4 shrink-0 text-primary" />
                <Terminal v-else class="size-4 shrink-0 text-primary" />
                <div class="min-w-0 flex-1">
                  <p class="flex items-center gap-1.5 text-sm font-medium">
                    {{ server.name }}
                    <Badge v-if="server.in_library" variant="outline" class="h-4 px-1.5 text-[10px]">{{ t('projects.inLibrary') }}</Badge>
                  </p>
                  <p class="truncate font-mono text-[11px] text-muted-foreground">{{ server.url || [server.command, ...(server.args || [])].join(' ') }}</p>
                </div>
                <AppTooltip :content="t('projects.removeMcp')">
                  <Button variant="ghost" size="icon-sm" class="text-muted-foreground hover:text-destructive" :disabled="busy" @click="removeMcp(server.name)">
                    <Trash2 />
                  </Button>
                </AppTooltip>
              </div>
            </div>
            <p class="text-[11px] leading-relaxed text-muted-foreground">
              {{ t('projects.mcpNote') }}
              <button type="button" class="font-mono underline-offset-2 hover:underline" @click="reveal(detail.mcp_path)">.mcp.json</button>
            </p>
          </template>

          <!-- CLAUDE.md -->
          <template v-else>
            <Textarea
              v-model="claudeMD"
              class="min-h-72 resize-y font-mono text-xs"
              :placeholder="t('projects.claudeMdPlaceholder')"
              spellcheck="false"
            />
            <div class="flex items-center justify-between gap-3">
              <button type="button" class="truncate font-mono text-[11px] text-muted-foreground underline-offset-2 hover:underline" @click="reveal(detail.claude_md_path)">
                {{ detail.claude_md_path }}
              </button>
              <Button :disabled="busy || claudeMD === detail.claude_md" @click="saveClaudeMD">
                <Loader2 v-if="busy" class="animate-spin" />
                <Save v-else />
                {{ t('common.save') }}
              </Button>
            </div>
          </template>
        </CardContent>
      </Card>
    </div>
  </AppModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { FolderGit2, FolderOpen, FolderPlus, Globe, Loader2, Pin, Plus, Save, Search, Terminal, Trash2, X } from '@lucide/vue'
import type { ProjectDetail, ProjectInfo } from '@/types'
import { projectService } from '@/services/projectService'
import { callApp } from '@/services/appBridge'
import { useConfigStore } from '@/stores/configStore'
import { useMcpStore } from '@/stores/mcpStore'
import { useConfirm } from '@/composables/useConfirm'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import AppTooltip from '@/components/common/AppTooltip.vue'
import SegmentedPills from '@/components/layout/SegmentedPills.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

const props = defineProps<{
  modelValue: boolean
  embedded?: boolean
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const { t } = useI18n()
const toast = useToast()
const confirm = useConfirm()
const configStore = useConfigStore()
const mcpStore = useMcpStore()

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
})

const projects = ref<ProjectInfo[]>([])
const loading = ref(false)
const adding = ref(false)
const busy = ref(false)
const query = ref('')
const typedPath = ref('')
const selectedPath = ref('')
const detail = ref<ProjectDetail | null>(null)
const detailLoading = ref(false)
const envChoice = ref('')
const mcpChoice = ref('')
const claudeMD = ref('')

type Tab = 'env' | 'mcp' | 'claude_md'
const tab = ref<Tab>('env')
const tabs = computed(() => [
  { value: 'env', label: t('projects.tabEnv') },
  { value: 'mcp', label: 'MCP' },
  { value: 'claude_md', label: 'CLAUDE.md' },
])

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return projects.value
  return projects.value.filter(item => item.name.toLowerCase().includes(q) || item.path.toLowerCase().includes(q))
})

// 只有直连的 Claude Code 配置能按项目应用（需要协议转换的依赖全局路由，官方登录无法按项目撤销）
const claudeEnvs = computed(() => configStore.claudeEnvs.filter(env => !env.official_login))
const addableMcp = computed(() => {
  const existing = new Set(detail.value?.mcp_servers.map(item => item.name) || [])
  return mcpStore.servers.filter(server => !existing.has(server.name))
})

function errorText(e: unknown) {
  return e instanceof Error ? e.message : String(e)
}

function onTab(value: string) {
  if (value === 'env' || value === 'mcp' || value === 'claude_md') tab.value = value
}

async function loadProjects() {
  loading.value = true
  try {
    projects.value = await projectService.list()
    if (selectedPath.value && !projects.value.some(item => item.path === selectedPath.value)) {
      selectedPath.value = ''
      detail.value = null
    }
    if (!selectedPath.value && projects.value.length) await select(projects.value[0].path)
  } catch (e) {
    toast.error(t('projects.loadFailed', { error: errorText(e) }))
  } finally {
    loading.value = false
  }
}

async function loadDetail() {
  if (!selectedPath.value) return
  detailLoading.value = true
  try {
    detail.value = await projectService.detail(selectedPath.value)
    claudeMD.value = detail.value.claude_md
    envChoice.value = detail.value.applied_env || ''
    mcpChoice.value = ''
  } catch (e) {
    detail.value = null
    toast.error(t('projects.loadFailed', { error: errorText(e) }))
  } finally {
    detailLoading.value = false
  }
}

async function select(path: string) {
  selectedPath.value = path
  await loadDetail()
}

// 操作后同时刷新详情与列表里的徽标
async function refresh() {
  await loadDetail()
  const index = projects.value.findIndex(item => item.path === selectedPath.value)
  if (index >= 0 && detail.value) {
    const { mcp_servers: _m, env_vars: _e, claude_md: _c, mcp_path: _p, settings_path: _s, claude_md_path: _cp, ...info } = detail.value
    projects.value[index] = info
  }
}

async function addPath(path: string) {
  adding.value = true
  try {
    const info = await projectService.add(path)
    await loadProjects()
    await select(info.path)
    typedPath.value = ''
  } catch (e) {
    toast.error(t('projects.addFailed', { error: errorText(e) }))
  } finally {
    adding.value = false
  }
}

async function pickAndAdd() {
  try {
    const path = await projectService.pickDirectory()
    if (path) await addPath(path)
  } catch (e) {
    toast.error(t('projects.addFailed', { error: errorText(e) }))
  }
}

function addTypedPath() {
  if (typedPath.value.trim()) void addPath(typedPath.value.trim())
}

async function removeProject() {
  if (!detail.value) return
  const ok = await confirm.show(t('projects.removeTitle'), t('projects.removeMsg', { name: detail.value.name }), 'warning')
  if (!ok) return
  try {
    await projectService.remove(detail.value.path)
    selectedPath.value = ''
    detail.value = null
    await loadProjects()
  } catch (e) {
    toast.error(t('projects.opFailed', { error: errorText(e) }))
  }
}

async function run(action: () => Promise<unknown>, success?: string) {
  busy.value = true
  try {
    const result = await action()
    toast.success(typeof result === 'string' && result ? result : (success || t('projects.done')))
    await refresh()
  } catch (e) {
    toast.error(t('projects.opFailed', { error: errorText(e) }))
  } finally {
    busy.value = false
  }
}

function applyEnv() {
  if (!detail.value || !envChoice.value) return
  const path = detail.value.path
  const name = envChoice.value
  void run(() => projectService.applyEnv(path, name), t('projects.applied', { name }))
}

async function clearEnv() {
  if (!detail.value) return
  const ok = await confirm.show(t('projects.clearTitle'), t('projects.clearMsg'), 'warning')
  if (!ok) return
  const path = detail.value.path
  await run(() => projectService.clearEnv(path), t('projects.cleared'))
  envChoice.value = ''
}

function addMcp() {
  if (!detail.value || !mcpChoice.value) return
  const path = detail.value.path
  const name = mcpChoice.value
  void run(() => projectService.addMcp(path, [name]), t('projects.mcpAdded', { name }))
}

async function removeMcp(name: string) {
  if (!detail.value) return
  const ok = await confirm.show(t('projects.removeMcp'), t('projects.removeMcpMsg', { name }), 'danger')
  if (!ok) return
  const path = detail.value.path
  await run(() => projectService.removeMcp(path, name), t('projects.mcpRemoved', { name }))
}

function saveClaudeMD() {
  if (!detail.value) return
  const path = detail.value.path
  const content = claudeMD.value
  void run(() => projectService.saveClaudeMD(path, content), t('projects.claudeMdSaved'))
}

async function reveal(path: string) {
  try {
    await callApp('OpenConfigFile', path)
  } catch (e) {
    toast.error(t('projects.opFailed', { error: errorText(e) }))
  }
}

watch(isOpen, (open) => {
  if (!open) return
  void loadProjects()
  if (mcpStore.servers.length === 0) void mcpStore.loadServers()
}, { immediate: true })
</script>
