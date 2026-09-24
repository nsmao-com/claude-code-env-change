<template>
  <AppModal v-model="isOpen" :title="t('mcp.importModal.title')" size="lg">
    <div class="space-y-4">
      <p class="text-sm text-muted-foreground">
        {{ t('mcp.importModal.desc') }}
      </p>

      <FileDropZone
        compact
        :title="t('mcp.importModal.dropTitle')"
        :hint="t('mcp.importModal.dropHint')"
        @file="onDropFile"
        @clear="jsonInput = ''"
        @error="onDropError"
      />

      <div class="rounded-xl bg-muted/40 p-3 font-mono text-xs text-muted-foreground">
        <p>{{ t('mcp.importModal.fmtMcpServers') }}</p>
        <p>{{ t('mcp.importModal.fmtMap') }}</p>
        <p>{{ t('mcp.importModal.fmtSingle') }}</p>
      </div>

      <div class="grid gap-1.5">
        <Label>{{ t('mcp.importModal.platforms') }}</Label>
        <ToggleGroup
          type="multiple"
          :model-value="selectedPlatformKeys"
          variant="outline"
          :spacing="2"
          class="flex w-full flex-wrap"
          @update:model-value="onPlatforms"
        >
          <ToggleGroupItem value="claude" class="flex-1">
            <BrandIcon provider="claude" />
            Claude
            <Check v-if="selectedPlatforms.claude" />
          </ToggleGroupItem>
          <ToggleGroupItem value="claude-desktop" class="flex-1">
            <BrandIcon provider="claude_desktop" />
            Claude Desktop
            <Check v-if="selectedPlatforms.claudeDesktop" />
          </ToggleGroupItem>
          <ToggleGroupItem value="codex" class="flex-1">
            <BrandIcon provider="codex" />
            Codex
            <Check v-if="selectedPlatforms.codex" />
          </ToggleGroupItem>
          <ToggleGroupItem value="antigravity" class="flex-1">
            <BrandIcon provider="antigravity" />
            Antigravity
            <Check v-if="selectedPlatforms.antigravity" />
          </ToggleGroupItem>
          <ToggleGroupItem value="opencode" class="flex-1">
            <BrandIcon provider="opencode" />
            OpenCode
            <Check v-if="selectedPlatforms.opencode" />
          </ToggleGroupItem>
          <ToggleGroupItem value="grok" class="flex-1">
            <BrandIcon provider="grok" />
            Grok
            <Check v-if="selectedPlatforms.grok" />
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      <div class="grid gap-1.5">
        <Label>{{ t('mcp.importModal.content') }}</Label>
        <CodeEditor
          v-model="jsonInput"
          language="json"
          class="min-h-64"
          max-height="22rem"
          placeholder='{"mcpServers": {"filesystem": {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem"]}}}'
        />
      </div>
    </div>

    <template #footer>
      <Button type="button" variant="outline" @click="isOpen = false">
        {{ t('common.cancel') }}
      </Button>
      <Button
        type="button"
        :disabled="isImporting || !hasSelectedPlatform"
        @click="handleImport"
      >
        <Loader2 v-if="isImporting" class="animate-spin" />
        {{ t('mcp.import') }}
      </Button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { ref, computed, watch } from 'vue'
import { Check, Loader2 } from '@lucide/vue'
import { useMcpStore } from '@/stores/mcpStore'
import { useConfigStore } from '@/stores/configStore'
import BrandIcon from '@/components/common/BrandIcon.vue'
import { useToast } from '@/composables/useToast'
import AppModal from '@/components/common/AppModal.vue'
import FileDropZone from '@/components/common/FileDropZone.vue'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import CodeEditor from '@/components/common/CodeEditor.vue'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

const { t } = useI18n()

interface Props {
  modelValue: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  imported: []
}>()

const mcpStore = useMcpStore()
const configStore = useConfigStore()
const toast = useToast()

function platformsFromFilter() {
  const tool = configStore.currentFilter
  return {
    claude: tool === 'all' || tool === 'claude',
    claudeDesktop: tool === 'claude_desktop',
    codex: tool === 'codex',
    antigravity: tool === 'antigravity',
    opencode: tool === 'opencode',
    grok: tool === 'grok',
  }
}

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const jsonInput = ref('')
const isImporting = ref(false)
const selectedPlatforms = ref(platformsFromFilter())

const hasSelectedPlatform = computed(() => {
  return selectedPlatforms.value.claude || selectedPlatforms.value.claudeDesktop || selectedPlatforms.value.codex || selectedPlatforms.value.antigravity || selectedPlatforms.value.opencode || selectedPlatforms.value.grok
})

const selectedPlatformKeys = computed(() => {
  const keys: string[] = []
  if (selectedPlatforms.value.claude) keys.push('claude')
  if (selectedPlatforms.value.claudeDesktop) keys.push('claude-desktop')
  if (selectedPlatforms.value.codex) keys.push('codex')
  if (selectedPlatforms.value.antigravity) keys.push('antigravity')
  if (selectedPlatforms.value.opencode) keys.push('opencode')
  if (selectedPlatforms.value.grok) keys.push('grok')
  return keys
})

function onDropFile(file: { name: string, text: string }) {
  jsonInput.value = file.text.replace(/^\uFEFF/, '')
}

function onDropError(message: string) {
  toast.error(message)
}

function onPlatforms(value: unknown) {
  const keys = Array.isArray(value) ? value : []
  selectedPlatforms.value = {
    claude: keys.includes('claude'),
    claudeDesktop: keys.includes('claude-desktop'),
    codex: keys.includes('codex'),
    antigravity: keys.includes('antigravity'),
    opencode: keys.includes('opencode'),
    grok: keys.includes('grok'),
  }
}

watch(isOpen, (open) => {
  if (open) {
    selectedPlatforms.value = platformsFromFilter()
    return
  }
  jsonInput.value = ''
  selectedPlatforms.value = platformsFromFilter()
})

async function handleImport() {
  if (!jsonInput.value.trim()) {
    toast.error(t('mcp.importModal.jsonRequired'))
    return
  }

  if (!hasSelectedPlatform.value) {
    toast.error(t('mcp.importModal.platformRequired'))
    return
  }

  isImporting.value = true
  try {
    const servers = await mcpStore.importFromJSON(jsonInput.value)
    if (!servers || servers.length === 0) {
      toast.error(t('mcp.importModal.noServers'))
      return
    }

    const platforms: string[] = []
    if (selectedPlatforms.value.claude) platforms.push('claude-code')
    if (selectedPlatforms.value.claudeDesktop) platforms.push('claude-desktop')
    if (selectedPlatforms.value.codex) platforms.push('codex')
    if (selectedPlatforms.value.antigravity) platforms.push('antigravity')
    if (selectedPlatforms.value.opencode) platforms.push('opencode')
    if (selectedPlatforms.value.grok) platforms.push('grok')

    servers.forEach(server => {
      server.enable_platform = platforms
    })

    await mcpStore.addServers(servers)
    toast.success(t('mcp.importModal.success', { count: servers.length }))
    isOpen.value = false
    emit('imported')
  } catch (e: any) {
    toast.error(t('mcp.importFailed', { error: e?.message || String(e) }))
  } finally {
    isImporting.value = false
  }
}
</script>
